package core

import (
	"fmt"
	"trading_assistant/models"
	"trading_assistant/pkg/exchanges"
	"trading_assistant/pkg/redis"
	"trading_assistant/pkg/telegram"

	"github.com/sirupsen/logrus"
)

type AccountManager struct {
	running  bool
	stopChan chan bool
}

var GlobalAccountManager *AccountManager

// InitAccountManager 初始化账户管理器
func InitAccountManager() {
	GlobalAccountManager = &AccountManager{
		running:  false,
		stopChan: make(chan bool),
	}
}

// Start 开始账户监控
func (am *AccountManager) Start() {
	if am.running {
		logrus.Warn("账户监控已在运行")
		return
	}

	am.running = true
	logrus.Info("账户监控已开始...")

	// 使用统一的WebSocket管理器启动用户数据流
	if exchanges.GlobalWebSocketManager != nil {
		handlers := &exchanges.UserDataHandlers{
			OnPosition: am.handlePositionUpdate,
			OnOrder:    am.handleOrderUpdate,
			OnError:    am.handleError,
		}

		err := exchanges.GlobalWebSocketManager.StartUserData(handlers)
		if err != nil {
			logrus.Errorf("启动用户数据流失败: %v", err)
			am.running = false
			return
		}
	}

	// 初始加载当前持仓和余额
	go am.initializeData()
}

// Stop 停止账户监控
func (am *AccountManager) Stop() {
	if !am.running {
		return
	}

	am.running = false
	am.stopChan <- true
	logrus.Info("账户监控已停止")
}

// IsRunning 检查是否在运行
func (am *AccountManager) IsRunning() bool {
	return am.running
}

// initializeData 初始化数据
func (am *AccountManager) initializeData() {
	if exchanges.GlobalBinanceClient == nil {
		return
	}

	// 初始化持仓数据
	positions, err := exchanges.GlobalBinanceClient.GetPositions()
	if err != nil {
		logrus.Errorf("初始化持仓数据失败: %v", err)
	} else {
		for i := range positions {
			position := positions[i]
			err = redis.GlobalRedisClient.SetPosition(position)
			if err != nil {
				logrus.Errorf("保存初始持仓数据失败: %v", err)
			}
		}
		logrus.Infof("初始化了 %d 个持仓", len(positions))
	}

	// 发送Telegram通知
	if telegram.GlobalTelegramClient != nil {
		telegram.GlobalTelegramClient.SendMessage("positions and balances initialized")
	}
}

// handlePositionUpdate 处理持仓更新
func (am *AccountManager) handlePositionUpdate(position *models.Position) {
	logrus.Debugf("收到持仓更新: %s %s %f", position.Symbol, position.Side, position.Size)
	if redis.GlobalRedisClient != nil {
		// 保存到Redis
		err := redis.GlobalRedisClient.SetPosition(position)
		if err != nil {
			logrus.Errorf("保存持仓信息失败: %v", err)
			return
		}

		// 清空仓位缓存
		if err = redis.GlobalRedisClient.DeleteCache(redis.CacheKeyPositions); err != nil {
			logrus.Errorf("清理持仓缓存失败: %v", err)
		} else {
			logrus.Debugf("已清理持仓缓存")
		}
	}
}

// handleOrderUpdate 处理订单更新
func (am *AccountManager) handleOrderUpdate(order *models.Order) {
	logrus.Infof("订单更新: %s %s %s", order.Symbol, order.Side, order.Status)

	// 清理相关缓存，确保数据一致性
	if redis.GlobalRedisClient != nil {
		// 清理该交易对的订单缓存
		orderCacheKey := fmt.Sprintf("%s:%s", redis.CacheKeyOrders, order.Symbol)
		if err := redis.GlobalRedisClient.DeleteCache(orderCacheKey); err != nil {
			logrus.Errorf("清理订单缓存失败: %v", err)
		}

		// 清理空symbol的订单缓存
		emptySymbolCacheKey := fmt.Sprintf("%s:", redis.CacheKeyOrders)
		if err := redis.GlobalRedisClient.DeleteCache(emptySymbolCacheKey); err != nil {
			logrus.Errorf("清理全量订单缓存失败: %v", err)
		}

		// 如果订单被执行或取消，清理余额缓存
		if order.Status == "FILLED" || order.Status == "PARTIALLY_FILLED" || order.Status == "CANCELED" {
			if err := redis.GlobalRedisClient.DeleteCache(redis.CacheKeyBalances + "*"); err != nil {
				logrus.Errorf("清理余额缓存失败: %v", err)
			}
			logrus.Debugf("已清理余额缓存，订单状态: %s", order.Status)
		}

		logrus.Debugf("已清理订单相关缓存: %s", order.Symbol)
	}

	if telegram.GlobalTelegramClient != nil {
		err := telegram.GlobalTelegramClient.SendOrderNotification(order)
		if err != nil {
			logrus.Errorf("发送订单更新通知失败: %v", err)
		}
	}
}

// handleError 处理错误
func (am *AccountManager) handleError(err error) {
	logrus.Errorf("用户数据流错误: %v", err)
}

// GetAccountStatus 获取账户状态
func (am *AccountManager) GetAccountStatus() map[string]interface{} {
	status := map[string]interface{}{
		"running": am.running,
		"mode":    "websocket",
	}

	// 获取持仓信息
	if exchanges.GlobalBinanceClient != nil {
		positions, err := exchanges.GlobalBinanceClient.GetPositions()
		if err != nil {
			status["positions"] = 0
			status["error"] = err.Error()
		} else {
			status["positions"] = len(positions)

			// 计算总盈亏
			totalPnl := 0.0
			for _, pos := range positions {
				totalPnl += pos.UnrealizedPnl
			}
			status["total_pnl"] = totalPnl
		}
	}
	return status
}
