package core

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"trading_assistant/models"
	"trading_assistant/pkg/exchanges/binance"
	"trading_assistant/pkg/exchanges/types"
	"trading_assistant/pkg/redis"
	"trading_assistant/pkg/telegram"
	"trading_assistant/pkg/websocket"

	"github.com/sirupsen/logrus"
)

// AccountManager 账户管理器
type AccountManager struct {
	running       bool             // 运行状态
	stopChan      chan bool        // 停止信号通道
	binanceClient *binance.Binance // Binance客户端，用于WebSocket连接

	// 防抖动定时器
	accountDataDebounceTimer *time.Timer // 账户数据更新防抖动定时器
	debounceMutex            sync.Mutex  // 定时器锁
}

// GlobalAccountManager 全局账户管理器实例
var GlobalAccountManager *AccountManager

// InitAccountManager 初始化账户管理器
func InitAccountManager(binanceClient *binance.Binance) {
	GlobalAccountManager = &AccountManager{
		running:       false,
		stopChan:      make(chan bool),
		binanceClient: binanceClient,
		// 初始化防抖动定时器
		accountDataDebounceTimer: nil,
	}
}

// Start 启动账户管理器，开始实时监听用户数据流
func (am *AccountManager) Start() {
	if am.running {
		logrus.Warn("账户监控已在运行，跳过重复启动")
		return
	}

	am.running = true
	logrus.Info("启动账户管理器，开始实时监听...")

	// 设置用户数据流重连处理器
	if am.binanceClient != nil {
		am.binanceClient.SetUserDataReconnectHandler(am.handleUserDataReconnect)
	}

	// 初始化基础数据
	go am.initializeData()

	// 启动用户数据流WebSocket监听
	go am.startUserDataStream()
}

// Stop 停止账户监控
func (am *AccountManager) Stop() {
	if !am.running {
		return
	}

	am.running = false

	// 停止用户数据流
	if am.binanceClient != nil {
		if err := am.binanceClient.UnsubscribeFromUserData(); err != nil {
			logrus.Errorf("停止用户数据流失败: %v", err)
		} else {
			logrus.Info("用户数据流已停止")
		}
	}

	// 清理防抖动定时器
	am.cleanupDebounceTimers()

	am.stopChan <- true
	logrus.Info("账户监控已停止")
}

// IsRunning 检查是否在运行
func (am *AccountManager) IsRunning() bool {
	return am.running
}

// initializeData 初始化数据
func (am *AccountManager) initializeData() {
	if am.binanceClient == nil {
		logrus.Error("Binance客户端未初始化")
		return
	}

	logrus.Info("开始初始化账户数据...")

	// 清除旧的持仓数据
	logrus.Info("清除Redis中的旧持仓数据...")
	if err := redis.GlobalRedisClient.ClearAllPositions(); err != nil {
		logrus.Errorf("清除旧持仓数据失败: %v", err)
	}

	// 主动获取初始账户数据
	logrus.Info("正在获取初始账户数据...")
	am.refreshBalances()

	logrus.Info("账户数据初始化完成")
}

// startUserDataStream 启动用户数据流监听
func (am *AccountManager) startUserDataStream() {
	if am.binanceClient == nil {
		logrus.Error("Binance客户端未初始化，无法启动用户数据流")
		return
	}

	logrus.Info("正在建立用户数据流WebSocket连接...")

	// 订阅用户数据流，注册消息处理器
	err := am.binanceClient.SubscribeToUserData(am.handleUserDataMessage)
	if err != nil {
		logrus.Errorf("用户数据流启动失败: %v", err)

		// 发送故障通知
		if telegram.GlobalTelegramClient != nil {
			err = telegram.GlobalTelegramClient.SendMessage("连接失败")
			if err != nil {
				logrus.Errorf("发送Telegram通知失败: %v", err)
			}
		}
		return
	}

}

// handleUserDataMessage 处理用户数据流消息
func (am *AccountManager) handleUserDataMessage(metadata types.MetaData, data interface{}) error {
	logrus.Infof("AccountManager收到用户数据流消息，数据类型: %s, 交易所: %s, 时间戳: %d",
		metadata.DataType, metadata.Exchange, metadata.Timestamp)

	switch metadata.DataType {
	case "account":
		if accountUpdate, ok := data.(*types.WatchAccountUpdate); ok {
			return am.handleAccountUpdate(accountUpdate)
		} else {
			logrus.Warnf("account数据类型转换失败: %T", data)
		}
	case "order":
		if orderUpdate, ok := data.(*types.WatchOrderUpdate); ok {
			return am.handleOrderUpdate(orderUpdate)
		} else {
			logrus.Warnf("order数据类型转换失败: %T", data)
		}
	default:
		logrus.Infof("未处理的用户数据流消息类型: %s", metadata.DataType)
	}

	return nil
}

// cleanupDebounceTimers 清理防抖动定时器
func (am *AccountManager) cleanupDebounceTimers() {
	am.debounceMutex.Lock()
	defer am.debounceMutex.Unlock()

	// 停止并清理账户数据防抖动定时器
	if am.accountDataDebounceTimer != nil {
		am.accountDataDebounceTimer.Stop()
		am.accountDataDebounceTimer = nil
		logrus.Debug("账户数据防抖动定时器已清理")
	}
}

// debounceAccountDataUpdate 防抖动账户数据更新
func (am *AccountManager) debounceAccountDataUpdate() {
	am.debounceMutex.Lock()
	defer am.debounceMutex.Unlock()

	// 如果AccountManager已停止，不创建新的定时器
	if !am.running {
		logrus.Debug("AccountManager已停止，跳过账户数据防抖动")
		return
	}

	// 停止已有的账户数据定时器
	if am.accountDataDebounceTimer != nil {
		am.accountDataDebounceTimer.Stop()
		am.accountDataDebounceTimer = nil
	}

	// 创建新的统一定时器，1秒后同时执行余额和仓位刷新
	am.accountDataDebounceTimer = time.AfterFunc(1*time.Second, func() {
		// 检查AccountManager是否还在运行
		if !am.running {
			logrus.Debug("AccountManager已停止，跳过账户数据刷新")
			return
		}
		logrus.Info("防抖动延迟后执行账户数据刷新")

		// 刷新余额数据
		am.refreshBalances()
	})
}

// handleAccountUpdate 处理账户更新事件
func (am *AccountManager) handleAccountUpdate(accountUpdate *types.WatchAccountUpdate) error {
	logrus.Infof("收到账户更新事件，事件类型: %s, 余额数量: %d，持仓数量: %d, 事件时间: %d",
		accountUpdate.EventType, len(accountUpdate.Balances), len(accountUpdate.Positions), accountUpdate.EventTime)

	if len(accountUpdate.Balances) > 0 {
		logrus.Infof("检测到余额变化，启动防抖动更新，余额变化数量: %d", len(accountUpdate.Balances))
		go am.debounceAccountDataUpdate()
	}
	return nil
}

// refreshBalances 刷新余额数据并缓存到Redis
func (am *AccountManager) refreshBalances() {
	if am.binanceClient == nil {
		logrus.Error("Binance客户端未初始化，无法刷新余额")
		return
	}

	logrus.Info("开始刷新账户余额数据...")

	// 从交易所获取最新余额
	account, err := am.binanceClient.FetchBalance(context.Background(), nil)
	if err != nil {
		logrus.Errorf("获取账户余额失败: %v", err)
		return
	}

	// 获取持仓数据以计算总盈亏
	var totalPnl = 0.0
	var marginUsed = 0.0
	var positionCount = 0

	positions, err := am.binanceClient.FetchPositions(context.Background(), nil, nil)
	if err == nil {
		// 处理持仓数据
		am.processAndCachePositions(positions, &totalPnl, &marginUsed, &positionCount)
	} else {
		logrus.Warnf("获取持仓数据失败，使用缓存数据: %v", err)
		// 尝试从缓存获取持仓盈亏
		if redis.GlobalRedisClient != nil {
			if allPositions, err := redis.GlobalRedisClient.GetAllPositions(); err == nil {
				for i := range allPositions {
					pos := allPositions[i]
					if pos.Size != 0 {
						totalPnl += pos.UnrealizedPnl
						marginUsed += pos.InitialMargin
						positionCount++
					}
				}
			}
		}
	}

	// 计算余额汇总信息
	balanceSummary := am.calculateBalanceSummary(account)

	// 补充持仓相关数据
	balanceSummary["total_pnl"] = totalPnl
	balanceSummary["margin_used"] = marginUsed
	balanceSummary["margin_available"] = balanceSummary["usdt_free"].(float64)
	balanceSummary["margin_ratio"] = 0.0
	if balanceSummary["usdt_total"].(float64) > 0 {
		balanceSummary["margin_ratio"] = marginUsed / balanceSummary["usdt_total"].(float64) * 100
	}
	balanceSummary["positions"] = positionCount

	// 计算总价值
	usdtTotal := balanceSummary["usdt_total"].(float64)
	balanceSummary["total_value"] = usdtTotal
	balanceSummary["net_value"] = usdtTotal
	balanceSummary["other_assets_value"] = 0.0 // 前端计算

	// 缓存到Redis
	if redis.GlobalRedisClient != nil {
		// 缓存余额汇总
		if err := redis.GlobalRedisClient.SetBalancesRealtime(balanceSummary); err != nil {
			logrus.Errorf("缓存余额汇总失败: %v", err)
		} else {
			logrus.Info("余额汇总已更新到Redis缓存")
		}

		// 缓存详细余额信息
		cacheKey := redis.CacheKeyBalances
		if err := redis.GlobalRedisClient.SetCacheWithExpiration(cacheKey, account, 0); err != nil {
			logrus.Errorf("缓存详细余额失败: %v", err)
		}
	}

	// 通过WebSocket广播余额更新
	logrus.Infof("准备广播余额更新，净值: %.2f USDT", balanceSummary["net_value"])
	go am.broadcastBalanceUpdate(balanceSummary)

	logrus.Info("余额数据刷新完成")
}

// processAndCachePositions 处理并缓存持仓数据
func (am *AccountManager) processAndCachePositions(positions []*types.Position, totalPnl, marginUsed *float64, positionCount *int) {
	// 转成统一的model positions
	mps := make([]*models.Position, 0, len(positions))

	// 逐个处理持仓信息
	for i := range positions {
		position := positions[i]

		// 累计计算余额相关数据
		if position.Size != 0 { // 只计算有持仓的
			*totalPnl += position.UnrealizedPnl
			*marginUsed += position.InitialMargin
			*positionCount++
		}

		// 转换保证金模式格式
		marginMode := types.MarginModeCross
		if position.MarginType == types.MarginModeIsolated {
			marginMode = types.MarginModeIsolated
		}

		positionModel := &models.Position{
			Symbol:            position.Symbol,
			Side:              strings.ToUpper(position.Side),
			Size:              position.Size,
			EntryPrice:        position.EntryPrice,
			MarkPrice:         position.MarkPrice,
			UnrealizedPnl:     position.UnrealizedPnl,
			Leverage:          int(position.Leverage),
			MarginMode:        marginMode,
			IsolatedMargin:    position.IsolatedMargin,
			InitialMargin:     position.InitialMargin,
			MaintenanceMargin: position.MaintenanceMargin,
			Notional:          position.NotionalValue,
			UpdatedAt:         time.UnixMilli(position.Timestamp),
		}

		mps = append(mps, positionModel)
	}

	// 存储持仓信息到Redis
	if redis.GlobalRedisClient != nil {
		// 清理所有旧的持仓数据
		if err := redis.GlobalRedisClient.ClearAllPositions(); err != nil {
			logrus.Errorf("清理旧持仓数据失败: %v", err)
		}

		// 自动选择有仓位的币种
		am.ensurePositionCoinsSelected(positions)

		// 逐个存储持仓信息
		for i := range mps {
			modelPosition := mps[i]
			if err := redis.GlobalRedisClient.SetPosition(modelPosition); err != nil {
				logrus.Errorf("存储持仓信息失败 %s: %v", modelPosition.Symbol, err)
			}
		}

		logrus.Infof("已存储 %d 个持仓到Redis", len(mps))
	}

	// 计算总盈亏
	totalPnlForBroadcast := 0.0
	for i := range mps {
		modelPosition := mps[i]
		totalPnlForBroadcast += modelPosition.UnrealizedPnl
	}

	// 通过WebSocket广播持仓更新
	logrus.Infof("准备广播持仓更新，持仓数量: %d, 总盈亏: %.4f USDT", len(mps), totalPnlForBroadcast)
	go am.broadcastAccountUpdate(mps, totalPnlForBroadcast)

	// 广播持仓更新事件
	go am.broadcastEvent("position_update", map[string]interface{}{
		"type":    "position_update",
		"message": fmt.Sprintf("持仓已更新: %d 个持仓，总盈亏 %.4f USDT", len(mps), totalPnlForBroadcast),
		"data": map[string]interface{}{
			"position_count": len(mps),
			"total_pnl":      totalPnlForBroadcast,
			"positions":      mps,
		},
	})

	logrus.Infof("持仓数据处理完成，共 %d 个持仓", len(mps))
}

// ensurePositionCoinsSelected 确保有仓位的币种被自动选中
func (am *AccountManager) ensurePositionCoinsSelected(positions []*types.Position) {
	if len(positions) == 0 {
		return
	}

	// 统计处理的币种
	var autoSelectedCount int
	var alreadySelectedCount int

	// 遍历每个持仓
	for i := range positions {
		position := positions[i]
		if position.Size == 0 {
			continue // 跳过空仓位
		}

		symbol := position.Symbol

		// 检查是否已经被选中
		if redis.GlobalRedisClient.IsCoinSelected(symbol) {
			alreadySelectedCount++
			logrus.Debugf("币种 %s 已被选中，有仓位: %.6f", symbol, position.Size)
			continue
		}

		// 自动选择该币种
		if err := redis.GlobalRedisClient.SetCoinSelection(symbol, models.CoinSelectionActive); err != nil {
			logrus.Errorf("自动选择币种 %s 失败: %v", symbol, err)
			continue
		}

		autoSelectedCount++
		logrus.Infof("自动选择币种 %s，当前仓位: %.6f %s", symbol, position.Size, position.Side)
	}

	if autoSelectedCount > 0 || alreadySelectedCount > 0 {
		logrus.WithFields(logrus.Fields{
			"total_positions":  len(positions),
			"auto_selected":    autoSelectedCount,
			"already_selected": alreadySelectedCount,
		}).Info("仓位币种自动选择完成")
	}
}

// calculateBalanceSummary 计算余额汇总信息
func (am *AccountManager) calculateBalanceSummary(account *types.Account) map[string]interface{} {
	// 初始化基础数据结构
	var assetDetails []map[string]interface{}
	var usdtTotal, usdtFree, usdtLocked float64

	// 处理所有资产
	for asset, total := range account.Total {
		if total <= 0.000001 { // 忽略极小余额
			continue
		}

		logrus.Debugf("处理资产: %s, 余额: %f", asset, total)

		// 计算可用余额和锁定余额
		free := 0.0
		if freeAmount, exists := account.Free[asset]; exists {
			free = freeAmount
		}
		locked := total - free
		if locked < 0 {
			locked = 0
		}

		// 创建资产详情（简化字段，价格由前端处理）
		assetDetail := map[string]interface{}{
			"asset":          asset,
			"wallet_balance": total,
			"free":           free,
			"locked":         locked,
			"updated_at":     time.Now().Format("2006-01-02 15:04:05"),
		}
		assetDetails = append(assetDetails, assetDetail)

		// 单独处理USDT数据
		if asset == "USDT" {
			usdtTotal = total
			usdtFree = free
			usdtLocked = locked
		}
	}

	// 构建返回数据（简化结构，总价值计算移到前端）
	return map[string]interface{}{
		"usdt_total":    usdtTotal,
		"usdt_free":     usdtFree,
		"usdt_locked":   usdtLocked,
		"total_pnl":     0.0, // 盈亏数据来自持仓计算
		"asset_count":   len(assetDetails),
		"last_updated":  time.Now().Unix(),
		"asset_details": assetDetails,
	}
}

// handleOrderUpdate 处理订单更新事件
func (am *AccountManager) handleOrderUpdate(orderUpdate *types.WatchOrderUpdate) error {
	logrus.Infof("收到订单更新: %s %s %s 执行类型: %s",
		orderUpdate.Symbol, orderUpdate.Side, orderUpdate.OrderStatus, orderUpdate.ExecutionType)

	// 清理相关缓存，确保数据一致性
	if redis.GlobalRedisClient != nil {
		// 清理该交易对的订单缓存
		orderCacheKey := fmt.Sprintf("%s:%s", redis.CacheKeyOrders, orderUpdate.Symbol)
		if err := redis.GlobalRedisClient.DeleteCache(orderCacheKey); err != nil {
			logrus.Errorf("清理订单缓存失败: %v", err)
		}

		// 清理全量订单缓存
		emptySymbolCacheKey := fmt.Sprintf("%s:", redis.CacheKeyOrders)
		if err := redis.GlobalRedisClient.DeleteCache(emptySymbolCacheKey); err != nil {
			logrus.Errorf("清理全量订单缓存失败: %v", err)
		}

		logrus.Debugf("已清理订单相关缓存: %s", orderUpdate.Symbol)
	}

	// 广播订单更新事件
	go am.broadcastEvent("order_update", map[string]interface{}{
		"type":    "order_update",
		"message": fmt.Sprintf("订单更新: %s %s %s @ %.4f", orderUpdate.Symbol, orderUpdate.Side, orderUpdate.OrderStatus, orderUpdate.OriginalPrice),
		"data": map[string]interface{}{
			"symbol":          orderUpdate.Symbol,
			"side":            orderUpdate.Side,
			"status":          orderUpdate.OrderStatus,
			"price":           orderUpdate.OriginalPrice,
			"quantity_filled": orderUpdate.LastQuantityFilled,
			"execution_type":  orderUpdate.ExecutionType,
			"order_id":        orderUpdate.OrderID,
		},
	})

	// 发送Telegram通知
	if telegram.GlobalTelegramClient != nil {
		// 构造通知消息
		message := fmt.Sprintf("%s %s %s @ %.4f",
			orderUpdate.Symbol,
			orderUpdate.Side,
			orderUpdate.OrderStatus,
			orderUpdate.OriginalPrice)

		if orderUpdate.LastQuantityFilled > 0 {
			message += fmt.Sprintf(" | 成交 %.4f", orderUpdate.LastQuantityFilled)
		}

		if err := telegram.GlobalTelegramClient.SendMessage(message); err != nil {
			logrus.Errorf("发送订单更新通知失败: %v", err)
		}
	}

	return nil
}

// GetAccountStatus 获取账户状态
func (am *AccountManager) GetAccountStatus() map[string]interface{} {
	status := map[string]interface{}{
		"running":            am.running,
		"mode":               "websocket_realtime",
		"has_binance_client": am.binanceClient != nil,
	}

	// 如果有binance客户端，添加更多状态信息
	if am.binanceClient != nil {
		wsClient := am.binanceClient.GetWebSocketClient()
		if wsClient != nil {
			status["websocket_connected"] = true
			status["user_data_active"] = true
		} else {
			status["websocket_connected"] = false
			status["user_data_active"] = false
		}
	}

	return status
}

// GetBinanceClient 获取Binance客户端
func (am *AccountManager) GetBinanceClient() *binance.Binance {
	return am.binanceClient
}

// broadcastBalanceUpdate 广播余额更新到WebSocket客户端
func (am *AccountManager) broadcastBalanceUpdate(balanceSummary map[string]interface{}) {
	// 获取WebSocket管理器
	wsManager := websocket.GetGlobalWebSocketManager()
	if wsManager == nil {
		return
	}

	// 广播余额数据
	wsManager.BroadcastBalances(balanceSummary)
	logrus.Debugf("通过WebSocket广播余额数据更新")
}

// broadcastAccountUpdate 广播账户更新到WebSocket客户端
func (am *AccountManager) broadcastAccountUpdate(positions []*models.Position, totalPnl float64) {
	// 获取WebSocket管理器
	wsManager := websocket.GetGlobalWebSocketManager()
	if wsManager == nil {
		return
	}

	// 准备账户数据
	accountData := map[string]interface{}{
		"positions":     positions,
		"totalPnl":      totalPnl,
		"positionCount": len(positions),
		"lastUpdate":    time.Now().Unix(),
	}

	// 广播账户数据
	wsManager.BroadcastAccount(accountData)
	logrus.Debugf("通过WebSocket广播账户数据更新，持仓数量: %d", len(positions))
}

// broadcastEvent 广播事件消息到WebSocket客户端
func (am *AccountManager) broadcastEvent(eventType string, eventData map[string]interface{}) {
	// 获取WebSocket管理器
	wsManager := websocket.GetGlobalWebSocketManager()
	if wsManager == nil {
		return
	}

	// 准备事件数据
	event := map[string]interface{}{
		"event_type": eventType,
		"timestamp":  time.Now().Unix(),
		"data":       eventData,
	}

	// 广播事件
	wsManager.BroadcastEvent(event)
	logrus.Debugf("通过WebSocket广播事件: %s", eventType)
}

// handleUserDataReconnect 处理用户数据流WebSocket重连事件
func (am *AccountManager) handleUserDataReconnect(attempt int, err error) {
	if err == nil {
		// 重连成功
		message := fmt.Sprintf("用户数据流重连成功 (尝试次数: %d)", attempt)
		logrus.Infof("用户数据流WebSocket重连成功，尝试次数: %d", attempt)

		// 重连成功后重新初始化数据
		go am.initializeData()

		// 发送Telegram通知
		if telegram.GlobalTelegramClient != nil {
			if sendErr := telegram.GlobalTelegramClient.SendMessage(message); sendErr != nil {
				logrus.Errorf("发送用户数据流重连成功通知失败: %v", sendErr)
			}
		}
	} else {
		// 重连失败或正在重连
		message := fmt.Sprintf("用户数据流重连 (尝试 %d): %s", attempt, err.Error())
		logrus.Warnf("用户数据流WebSocket重连事件，尝试次数: %d, 错误: %v", attempt, err)

		// 发送Telegram通知
		if telegram.GlobalTelegramClient != nil {
			if sendErr := telegram.GlobalTelegramClient.SendMessage(message); sendErr != nil {
				logrus.Errorf("发送用户数据流重连通知失败: %v", sendErr)
			}
		}
	}
}
