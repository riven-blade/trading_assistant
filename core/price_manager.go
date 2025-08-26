package core

import (
	"context"
	"fmt"
	"sync"
	"time"
	"trading_assistant/pkg/exchanges/binance"
	"trading_assistant/pkg/exchanges/types"
	"trading_assistant/pkg/redis"

	"github.com/sirupsen/logrus"
)

// PriceManager 价格订阅管理器
type PriceManager struct {
	binanceClient    *binance.Binance
	subscriptions    map[string]*PriceSubscription // symbol -> subscription
	subscriptionsMux sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
	isRunning        bool
	runningMux       sync.RWMutex
}

// PriceSubscription 单个价格订阅状态
type PriceSubscription struct {
	Symbol    string    `json:"symbol"`
	Status    string    `json:"status"` // active, inactive, error
	StartTime time.Time `json:"start_time"`
	LastData  time.Time `json:"last_data"`
	ErrorMsg  string    `json:"error_msg,omitempty"`
}

// NewPriceManager 创建价格管理器
func NewPriceManager(binanceClient *binance.Binance) *PriceManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &PriceManager{
		binanceClient: binanceClient,
		subscriptions: make(map[string]*PriceSubscription),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start 启动价格管理器
func (pm *PriceManager) Start() error {
	pm.runningMux.Lock()
	defer pm.runningMux.Unlock()

	if pm.isRunning {
		return fmt.Errorf("价格管理器已在运行")
	}

	pm.isRunning = true
	logrus.Info("价格管理器已启动")

	// 启动选中币种的订阅
	if err := pm.subscribeSelectedCoins(); err != nil {
		logrus.Errorf("启动选中币种价格订阅失败: %v", err)
	}

	return nil
}

// Stop 停止价格管理器
func (pm *PriceManager) Stop() {
	pm.runningMux.Lock()
	defer pm.runningMux.Unlock()

	if !pm.isRunning {
		return
	}

	pm.cancel()
	pm.isRunning = false

	// 取消所有订阅
	pm.subscriptionsMux.Lock()
	for symbol := range pm.subscriptions {
		pm.unsubscribePriceUnsafe(symbol)
	}
	pm.subscriptions = make(map[string]*PriceSubscription)
	pm.subscriptionsMux.Unlock()

	logrus.Info("价格管理器已停止")
}

// IsRunning 检查是否在运行
func (pm *PriceManager) IsRunning() bool {
	pm.runningMux.RLock()
	defer pm.runningMux.RUnlock()
	return pm.isRunning
}

// SubscribePrice 订阅单个币种的markPrice
func (pm *PriceManager) SubscribePrice(symbol string) error {
	pm.subscriptionsMux.Lock()
	defer pm.subscriptionsMux.Unlock()

	// 检查是否已经订阅
	if sub, exists := pm.subscriptions[symbol]; exists && sub.Status == "active" {
		logrus.Debugf("币种 %s 价格已在订阅中，跳过", symbol)
		return nil
	}

	// 创建订阅记录
	subscription := &PriceSubscription{
		Symbol:    symbol,
		Status:    "active",
		StartTime: time.Now(),
		LastData:  time.Time{},
	}

	// 订阅markPrice
	publishFunc := pm.createPriceHandler(symbol)
	err := pm.binanceClient.SubscribeToMarkPrice(symbol, publishFunc)
	if err != nil {
		subscription.Status = "error"
		subscription.ErrorMsg = err.Error()
		pm.subscriptions[symbol] = subscription
		return fmt.Errorf("订阅 %s markPrice失败: %v", symbol, err)
	}

	pm.subscriptions[symbol] = subscription
	logrus.WithFields(logrus.Fields{
		"symbol": symbol,
	}).Info("markPrice订阅成功")

	return nil
}

// UnsubscribePrice 取消订阅单个币种的markPrice
func (pm *PriceManager) UnsubscribePrice(symbol string) error {
	pm.subscriptionsMux.Lock()
	defer pm.subscriptionsMux.Unlock()

	return pm.unsubscribePriceUnsafe(symbol)
}

// unsubscribePriceUnsafe 取消价格订阅
func (pm *PriceManager) unsubscribePriceUnsafe(symbol string) error {
	// 检查订阅是否存在
	if _, exists := pm.subscriptions[symbol]; !exists {
		return nil // 已经没有订阅了
	}

	// 调用Binance客户端取消订阅
	err := pm.binanceClient.UnsubscribeFromMarkPrice(symbol)
	if err != nil {
		logrus.Errorf("取消订阅 %s markPrice失败: %v", symbol, err)
	}

	// 移除订阅记录
	delete(pm.subscriptions, symbol)

	logrus.WithFields(logrus.Fields{
		"symbol": symbol,
	}).Info("markPrice订阅已取消")

	return err
}

// SyncSubscriptions 同步价格订阅状态
func (pm *PriceManager) SyncSubscriptions() error {
	logrus.Info("开始同步markPrice订阅状态...")

	// 获取当前选中的币种
	selectedSymbols, err := redis.GlobalRedisClient.GetSelectedCoinSymbols()
	if err != nil {
		return fmt.Errorf("获取选中币种失败: %v", err)
	}

	pm.subscriptionsMux.Lock()
	defer pm.subscriptionsMux.Unlock()

	// 转换为map便于查找
	selectedMap := make(map[string]bool)
	for i := range selectedSymbols {
		symbol := selectedSymbols[i]
		selectedMap[symbol] = true
	}

	var addedCount int
	for i := range selectedSymbols {
		symbol := selectedSymbols[i]
		if _, exists := pm.subscriptions[symbol]; !exists {
			// 创建新订阅
			if err := pm.subscribePriceWithoutLock(symbol); err != nil {
				logrus.Errorf("添加 %s markPrice订阅失败: %v", symbol, err)
			} else {
				addedCount++
			}
		}
	}

	var removedCount int
	for symbol := range pm.subscriptions {
		if !selectedMap[symbol] {
			if err := pm.unsubscribePriceUnsafe(symbol); err != nil {
				logrus.Errorf("取消 %s markPrice订阅失败: %v", symbol, err)
			} else {
				removedCount++
			}
		}
	}

	logrus.WithFields(logrus.Fields{
		"selected_count": len(selectedSymbols),
		"added_count":    addedCount,
		"removed_count":  removedCount,
		"total_subs":     len(pm.subscriptions),
	}).Info("markPrice订阅状态同步完成")

	return nil
}

// subscribePriceWithoutLock 订阅价格
func (pm *PriceManager) subscribePriceWithoutLock(symbol string) error {
	subscription := &PriceSubscription{
		Symbol:    symbol,
		Status:    "active",
		StartTime: time.Now(),
		LastData:  time.Time{},
	}

	publishFunc := pm.createPriceHandler(symbol)
	err := pm.binanceClient.SubscribeToMarkPrice(symbol, publishFunc)
	if err != nil {
		subscription.Status = "error"
		subscription.ErrorMsg = err.Error()
		pm.subscriptions[symbol] = subscription
		return err
	}

	pm.subscriptions[symbol] = subscription
	return nil
}

// subscribeSelectedCoins 订阅选中的币种
func (pm *PriceManager) subscribeSelectedCoins() error {
	selectedSymbols, err := redis.GlobalRedisClient.GetSelectedCoinSymbols()
	if err != nil {
		return fmt.Errorf("获取选中币种失败: %v", err)
	}

	if len(selectedSymbols) == 0 {
		logrus.Info("当前没有选中的币种，跳过markPrice订阅")
		return nil
	}

	var successCount, errorCount int
	for _, symbol := range selectedSymbols {
		if err := pm.SubscribePrice(symbol); err != nil {
			logrus.Errorf("订阅 %s markPrice失败: %v", symbol, err)
			errorCount++
		} else {
			successCount++
		}
	}

	logrus.WithFields(logrus.Fields{
		"total_symbols": len(selectedSymbols),
		"success_count": successCount,
		"error_count":   errorCount,
	}).Info("选中币种markPrice订阅完成")

	return nil
}

// createPriceHandler 创建价格数据处理器
func (pm *PriceManager) createPriceHandler(symbol string) func(types.MetaData, interface{}) error {
	return func(metadata types.MetaData, data interface{}) error {
		// 更新最后数据时间
		pm.updateLastDataTime(symbol)

		// 处理markPrice数据
		return pm.processPriceData(symbol, metadata, data)
	}
}

// updateLastDataTime 更新最后数据时间
func (pm *PriceManager) updateLastDataTime(symbol string) {
	pm.subscriptionsMux.Lock()
	defer pm.subscriptionsMux.Unlock()

	if sub, exists := pm.subscriptions[symbol]; exists {
		sub.LastData = time.Now()
	}
}

// processPriceData 处理markPrice数据并保存到Redis
func (pm *PriceManager) processPriceData(symbol string, metadata types.MetaData, data interface{}) error {
	// 只处理选中的币种数据
	if !redis.GlobalRedisClient.IsCoinSelected(symbol) {
		// 如果币种不再选中，应该取消订阅
		go pm.UnsubscribePrice(symbol)
		return nil
	}

	// 解析markPrice数据
	markPrice, err := pm.parseMarkPriceData(symbol, data)
	if err != nil {
		logrus.Errorf("解析 %s markPrice数据失败: %v", symbol, err)
		return err
	}

	// 检查markPrice数据有效性
	if markPrice.MarkPrice <= 0 {
		logrus.Warnf("%s markPrice数据无效，标记价格: %f，跳过保存", symbol, markPrice.MarkPrice)
		return nil
	}

	// 保存到Redis
	if err := redis.GlobalRedisClient.SetMarkPrice(markPrice); err != nil {
		logrus.Errorf("保存 %s markPrice数据失败: %v", symbol, err)
		return err
	}

	logrus.Debugf("保存 %s markPrice数据成功，标记价格: %f",
		symbol, markPrice.MarkPrice)

	return nil
}

// parseMarkPriceData 解析markPrice数据
func (pm *PriceManager) parseMarkPriceData(symbol string, data interface{}) (*types.WatchMarkPrice, error) {
	// 处理Binance WebSocket返回的WatchMarkPrice数据
	watchMarkPrice, ok := data.(*types.WatchMarkPrice)
	if !ok {
		return nil, fmt.Errorf("无效的markPrice数据格式，期望 *exchanges.WatchMarkPrice")
	}

	return watchMarkPrice, nil
}

// GetSubscriptionStatus 获取订阅状态
func (pm *PriceManager) GetSubscriptionStatus() map[string]*PriceSubscription {
	pm.subscriptionsMux.RLock()
	defer pm.subscriptionsMux.RUnlock()

	// 创建副本避免并发问题
	status := make(map[string]*PriceSubscription)
	for symbol, sub := range pm.subscriptions {
		status[symbol] = &PriceSubscription{
			Symbol:    sub.Symbol,
			Status:    sub.Status,
			StartTime: sub.StartTime,
			LastData:  sub.LastData,
			ErrorMsg:  sub.ErrorMsg,
		}
	}

	return status
}
