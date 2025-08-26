package core

import (
	"context"
	"fmt"
	"strings"
	"time"
	"trading_assistant/models"
	"trading_assistant/pkg/exchanges/binance"
	"trading_assistant/pkg/exchanges/types"
	"trading_assistant/pkg/redis"
	"trading_assistant/pkg/telegram"

	"github.com/sirupsen/logrus"
)

// AccountManager 账户管理器
type AccountManager struct {
	running       bool             // 运行状态
	stopChan      chan bool        // 停止信号通道
	binanceClient *binance.Binance // Binance客户端，用于WebSocket连接
}

// GlobalAccountManager 全局账户管理器实例
var GlobalAccountManager *AccountManager

// InitAccountManager 初始化账户管理器
func InitAccountManager(binanceClient *binance.Binance) {
	GlobalAccountManager = &AccountManager{
		running:       false,
		stopChan:      make(chan bool),
		binanceClient: binanceClient,
	}
}

// Start 启动账户管理器，开始实时监听用户数据流
func (am *AccountManager) Start() {
	if am.running {
		logrus.Warn("账户监控已在运行")
		return
	}

	am.running = true
	logrus.Info("启动账户管理器，开始实时监听...")

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

	// 主动获取初始余额数据
	logrus.Info("正在获取初始余额数据...")
	am.refreshBalances()

	// 主动获取初始持仓数据
	logrus.Info("正在获取初始持仓数据...")
	am.refreshPositions()

	logrus.Info("账户数据初始化完成")

	// 发送完成通知
	if telegram.GlobalTelegramClient != nil {
		telegram.GlobalTelegramClient.SendMessage("账户已连接")
	}
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

	logrus.Info("用户数据流监听已启动，实时监控账户变化")
}

// handleUserDataMessage 处理用户数据流消息
func (am *AccountManager) handleUserDataMessage(metadata types.MetaData, data interface{}) error {
	logrus.Infof("📨 收到用户数据流消息，类型: %s", metadata.DataType)

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

// handleAccountUpdate 处理账户更新事件
func (am *AccountManager) handleAccountUpdate(accountUpdate *types.WatchAccountUpdate) error {
	logrus.Infof("收到账户更新事件，余额数量: %d，持仓数量: %d",
		len(accountUpdate.Balances), len(accountUpdate.Positions))

	if len(accountUpdate.Balances) > 0 {
		// 重新获取和缓存余额分布
		go am.refreshBalances()
	}

	if len(accountUpdate.Positions) > 0 {
		// 重新获取和缓存持仓分布
		go am.refreshPositions()
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
	var totalPnl float64 = 0.0
	var marginUsed float64 = 0.0
	var positionCount int = 0

	positions, err := am.binanceClient.FetchPositions(context.Background(), nil, nil)
	if err == nil {
		for i := range positions {
			position := positions[i]
			if position.Size != 0 { // 只计算有持仓的
				totalPnl += position.UnrealizedPnl
				marginUsed += position.InitialMargin
				positionCount++
			}
		}
	} else {
		logrus.Warnf("获取持仓数据失败，使用缓存数据: %v", err)
		// 尝试从缓存获取持仓盈亏
		if redis.GlobalRedisClient != nil {
			if allPositions, err := redis.GlobalRedisClient.GetAllPositions(); err == nil {
				for _, pos := range allPositions {
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

	// 净值就是总价值
	balanceSummary["net_value"] = balanceSummary["total_value"].(float64)

	// 缓存到Redis
	if redis.GlobalRedisClient != nil {
		// 缓存余额汇总（实时缓存，永不过期）
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

	// 发送Telegram通知
	if telegram.GlobalTelegramClient != nil {
		message := fmt.Sprintf("余额 %.2f USDT | 可用 %.2f | 持仓 %d",
			balanceSummary["net_value"], balanceSummary["usdt_free"], positionCount)
		telegram.GlobalTelegramClient.SendMessage(message)
	}

	logrus.Info("余额数据刷新完成")
}

// refreshPositions 刷新持仓数据并缓存到Redis
func (am *AccountManager) refreshPositions() {
	if am.binanceClient == nil {
		logrus.Error("Binance客户端未初始化，无法刷新持仓")
		return
	}

	logrus.Info("开始刷新持仓数据...")

	// 从交易所获取最新持仓
	positions, err := am.binanceClient.FetchPositions(context.Background(), nil, nil)
	if err != nil {
		logrus.Errorf("获取持仓信息失败: %v", err)
		return
	}

	// 缓存到Redis
	if redis.GlobalRedisClient != nil {
		// 清理旧的持仓缓存
		if err := redis.GlobalRedisClient.DeleteCache(redis.CacheKeyPositions + "*"); err != nil {
			logrus.Errorf("清理旧持仓缓存失败: %v", err)
		}

		// 缓存新的持仓数据
		cacheKey := redis.CacheKeyPositions
		if err := redis.GlobalRedisClient.SetCacheWithExpiration(cacheKey, positions, 0); err != nil {
			logrus.Errorf("缓存持仓数据失败: %v", err)
		} else {
			logrus.Infof("已缓存 %d 个持仓到Redis", len(positions))
		}

		// 自动选择有仓位的币种
		am.ensurePositionCoinsSelected(positions)

		// 逐个存储持仓信息
		for i := range positions {
			position := positions[i]
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

			if err := redis.GlobalRedisClient.SetPosition(positionModel); err != nil {
				logrus.Errorf("存储持仓信息失败 %s: %v", position.Symbol, err)
			}
		}
	}

	// 计算总盈亏
	totalPnl := 0.0
	for i := range positions {
		position := positions[i]
		totalPnl += position.UnrealizedPnl
	}

	// 发送Telegram通知
	if telegram.GlobalTelegramClient != nil {
		message := fmt.Sprintf("持仓 %d | PNL %.4f",
			len(positions), totalPnl)
		telegram.GlobalTelegramClient.SendMessage(message)
	}

	logrus.Info("持仓数据刷新完成")
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
	summary := map[string]interface{}{
		"total_value":        0.0,
		"usdt_total":         0.0,
		"usdt_free":          0.0,
		"usdt_locked":        0.0,
		"other_assets_value": 0.0,
		"total_pnl":          0.0,
		"net_value":          0.0,
		"asset_count":        0,
		"last_updated":       time.Now().Unix(),
		"asset_details":      []map[string]interface{}{},
	}

	var assetDetails []map[string]interface{}
	otherAssetsValue := 0.0
	assetCount := 0

	// 处理所有资产
	for asset, total := range account.Total {
		if total <= 0.000001 { // 忽略极小余额
			continue
		}

		free := 0.0
		if freeAmount, exists := account.Free[asset]; exists {
			free = freeAmount
		}

		locked := total - free
		if locked < 0 {
			locked = 0
		}

		// 计算USDT价值
		usdtValue := 0.0
		if asset == "USDT" {
			usdtValue = total
		} else {
			// 尝试从Redis获取标记价格
			if redis.GlobalRedisClient != nil {
				symbol := asset + "USDT"
				if markPrice, err := redis.GlobalRedisClient.GetMarkPrice(symbol); err == nil {
					usdtValue = total * markPrice.MarkPrice
				}
			}
		}

		// 创建资产详情
		assetDetail := map[string]interface{}{
			"asset":      asset,
			"amount":     total,
			"free":       free,
			"locked":     locked,
			"value_usdt": usdtValue,
			"updated_at": time.Now().Format("2006-01-02 15:04:05"),
		}

		assetDetails = append(assetDetails, assetDetail)
		assetCount++

		if asset == "USDT" {
			summary["usdt_total"] = total
			summary["usdt_free"] = free
			summary["usdt_locked"] = locked
		} else {
			otherAssetsValue += usdtValue
		}
	}

	// 计算总价值
	totalValue := summary["usdt_total"].(float64) + otherAssetsValue

	summary["total_value"] = totalValue
	summary["other_assets_value"] = otherAssetsValue
	summary["net_value"] = totalValue
	summary["asset_count"] = assetCount
	summary["asset_details"] = assetDetails

	return summary
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
