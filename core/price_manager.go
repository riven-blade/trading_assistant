package core

import (
	"context"
	"fmt"
	"sync"
	"time"
	"trading_assistant/pkg/exchanges/binance"
	"trading_assistant/pkg/exchanges/types"
	"trading_assistant/pkg/redis"
	"trading_assistant/pkg/telegram"
	"trading_assistant/pkg/websocket"

	"github.com/sirupsen/logrus"
)

// PriceManager 全局价格流管理器
type PriceManager struct {
	binanceClient    *binance.Binance
	ctx              context.Context
	cancel           context.CancelFunc
	isRunning        bool
	isSubscribed     bool      // 是否已订阅价格流
	subscriptionTime time.Time // 订阅开始时间
	lastDataTime     time.Time // 最后数据时间
	priceCount       int64     // 接收到的价格数据计数

	// 选中币种缓存
	selectedCoins    map[string]bool // 缓存选中的币种
	selectedCoinsMux sync.RWMutex    // 保护selectedCoins
	lastCacheUpdate  time.Time       // 上次缓存更新时间
}

// NewPriceManager 创建价格流管理器
func NewPriceManager(binanceClient *binance.Binance) *PriceManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &PriceManager{
		binanceClient:   binanceClient,
		ctx:             ctx,
		cancel:          cancel,
		selectedCoins:   make(map[string]bool),
		lastCacheUpdate: time.Time{}, // 初始化为零值，强制首次更新
	}
}

// Start 启动全局价格流订阅
func (pm *PriceManager) Start() error {
	if pm.isRunning {
		return fmt.Errorf("全局价格流管理器已在运行")
	}

	pm.isRunning = true

	// 设置WebSocket重连处理器
	pm.binanceClient.SetWebSocketReconnectHandler(pm.handleReconnect)

	// 订阅全局标记价格流
	if err := pm.subscribe(); err != nil {
		pm.isRunning = false
		return fmt.Errorf("订阅全局价格流失败: %v", err)
	}

	return nil
}

// Stop 停止全局价格流订阅
func (pm *PriceManager) Stop() {
	if !pm.isRunning {
		return
	}

	logrus.Info("停止价格流管理器...")

	pm.cancel()
	pm.isRunning = false

	// 取消订阅
	if pm.isSubscribed {
		if err := pm.binanceClient.UnsubscribeFromMarkPrice(); err != nil {
			logrus.Errorf("取消价格流订阅失败: %v", err)
		} else {
			pm.isSubscribed = false
			logrus.Info("价格流订阅已取消")
		}
	}

	logrus.Info("价格流管理器已停止")
}

// IsRunning 检查管理器是否在运行
func (pm *PriceManager) IsRunning() bool {
	return pm.isRunning
}

// GetStatus 获取管理器状态信息
func (pm *PriceManager) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"running":           pm.isRunning,
		"subscribed":        pm.isSubscribed,
		"subscription_time": pm.subscriptionTime.Unix(),
		"last_data_time":    pm.lastDataTime.Unix(),
		"price_count":       pm.priceCount,
		"stream_name":       binance.StreamMarkPriceArray1s,
		"mode":              "global_stream",
	}
}

// subscribe 执行全局价格流订阅
func (pm *PriceManager) subscribe() error {
	if pm.isSubscribed {
		return nil
	}

	// 订阅标记价格流 (1秒更新)
	err := pm.binanceClient.SubscribeToMarkPrice(pm.handlePriceData)
	if err != nil {
		return err
	}

	pm.isSubscribed = true
	pm.subscriptionTime = time.Now()
	pm.priceCount = 0

	return nil
}

// handlePriceData 处理接收到的价格数据
func (pm *PriceManager) handlePriceData(metadata types.MetaData, data interface{}) error {
	// 更新统计信息
	pm.lastDataTime = time.Now()
	pm.priceCount++

	// 验证数据类型
	if metadata.DataType != "markPrice" {
		return nil
	}

	// 解析标记价格数据
	markPrice, ok := data.(*types.WatchMarkPrice)
	if !ok {
		logrus.Warnf("收到无效的markPrice数据格式: %T", data)
		return nil
	}

	// 验证价格有效性
	if markPrice.MarkPrice <= 0 || markPrice.Symbol == "" {
		logrus.Debugf("跳过无效的价格数据: symbol=%s, price=%f", markPrice.Symbol, markPrice.MarkPrice)
		return nil
	}

	// 检查币种是否被选中
	if !pm.isCoinSelected(markPrice.Symbol) {
		// 币种未被选中，跳过处理
		return nil
	}

	// 保存到Redis缓存
	if err := pm.saveToCache(markPrice); err != nil {
		logrus.Errorf("保存 %s 价格数据到缓存失败: %v", markPrice.Symbol, err)
	}

	// 广播到WebSocket客户端
	pm.broadcastUpdate(markPrice)

	// 调试日志
	if pm.priceCount%5000 == 0 {
		logrus.Debugf("已处理 %d 条价格数据，最新选中币种: %s @ %f", pm.priceCount, markPrice.Symbol, markPrice.MarkPrice)
	}

	return nil
}

// isCoinSelected 检查币种是否被选中
func (pm *PriceManager) isCoinSelected(symbol string) bool {
	// 检查是否需要更新缓存
	if time.Since(pm.lastCacheUpdate) > 30*time.Second {
		pm.updateSelectedCoinsCache()
	}

	// 读取缓存
	pm.selectedCoinsMux.RLock()
	selected := pm.selectedCoins[symbol]
	pm.selectedCoinsMux.RUnlock()

	return selected
}

// updateSelectedCoinsCache 更新选中币种缓存
func (pm *PriceManager) updateSelectedCoinsCache() {
	if redis.GlobalRedisClient == nil {
		return
	}

	// 获取选中的币种列表
	selectedSymbols, err := redis.GlobalRedisClient.GetSelectedCoinSymbols()
	if err != nil {
		logrus.Errorf("获取选中币种列表失败: %v", err)
		return
	}

	// 更新缓存
	newCache := make(map[string]bool)
	for _, symbol := range selectedSymbols {
		newCache[symbol] = true
	}

	pm.selectedCoinsMux.Lock()
	pm.selectedCoins = newCache
	pm.lastCacheUpdate = time.Now()
	pm.selectedCoinsMux.Unlock()

	logrus.Debugf("更新选中币种缓存，共 %d 个币种: %v", len(selectedSymbols), selectedSymbols)
}

// RefreshSelectedCoinsCache 手动刷新选中币种缓存（供外部调用）
func (pm *PriceManager) RefreshSelectedCoinsCache() {
	pm.updateSelectedCoinsCache()
}

// saveToCache 保存价格数据到Redis缓存
func (pm *PriceManager) saveToCache(markPrice *types.WatchMarkPrice) error {
	if redis.GlobalRedisClient == nil {
		return fmt.Errorf("Redis客户端未初始化")
	}

	return redis.GlobalRedisClient.SetMarkPrice(markPrice)
}

// broadcastUpdate 广播价格更新到WebSocket客户端
func (pm *PriceManager) broadcastUpdate(markPrice *types.WatchMarkPrice) {
	wsManager := websocket.GetGlobalWebSocketManager()
	if wsManager == nil {
		return
	}

	// 构造价格数据
	priceData := map[string]interface{}{
		markPrice.Symbol: map[string]interface{}{
			"symbol":      markPrice.Symbol,
			"markPrice":   markPrice.MarkPrice,
			"indexPrice":  markPrice.IndexPrice,
			"fundingRate": markPrice.FundingRate,
			"fundingTime": markPrice.FundingTime,
			"updateTime":  markPrice.TimeStamp,
			"serverTime":  time.Now().Unix(),
		},
	}

	// 广播价格数据
	wsManager.BroadcastPrices(priceData)
}

// handleReconnect 处理WebSocket重连事件
func (pm *PriceManager) handleReconnect(attempt int, err error) {
	if err == nil {
		// 重连成功，重新订阅
		logrus.Infof("WebSocket重连成功 (尝试 %d 次)，恢复全局价格流订阅", attempt)

		if pm.isRunning {
			// 重连后连接是新的，需要强制重新订阅
			pm.isSubscribed = false // 重置订阅状态

			if subErr := pm.subscribe(); subErr != nil {
				logrus.Errorf("重连后恢复价格流订阅失败: %v", subErr)
				pm.sendTelegramNotification(fmt.Sprintf("价格数据流重连成功但订阅失败: %s", subErr.Error()))
			} else {
				logrus.Info("重连后价格流订阅恢复成功")
				pm.sendTelegramNotification(fmt.Sprintf("价格数据流重连成功 (第%d次尝试)", attempt))

				// 重连成功后立即刷新选中币种缓存
				pm.updateSelectedCoinsCache()
			}
		}
	} else {
		// 重连失败或正在重连
		logrus.Warnf("价格数据流重连中，尝试次数: %d, 错误: %v", attempt, err)

		// 标记为未订阅
		pm.isSubscribed = false

		// 发送重连通知
		if attempt == 1 || attempt%3 == 0 { // 只在第1次和每3次失败时发送通知
			pm.sendTelegramNotification(fmt.Sprintf("价格数据流重连中 (第%d次): %s", attempt, err.Error()))
		}
	}
}

// sendTelegramNotification 发送Telegram通知
func (pm *PriceManager) sendTelegramNotification(message string) {
	if telegram.GlobalTelegramClient != nil {
		if err := telegram.GlobalTelegramClient.SendMessage(message); err != nil {
			logrus.Errorf("发送Telegram通知失败: %v", err)
		}
	}
}
