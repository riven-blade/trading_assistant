package core

import (
	"fmt"
	"time"
	"trading_assistant/models"
	"trading_assistant/pkg/exchanges/binance"
	"trading_assistant/pkg/exchanges/types"
	"trading_assistant/pkg/redis"
	"trading_assistant/pkg/telegram"
	"trading_assistant/pkg/utils"
	"trading_assistant/pkg/websocket"

	"github.com/sirupsen/logrus"
)

type PriceMonitor struct {
	running       bool
	stopChan      chan bool
	tickInterval  time.Duration
	orderExecutor *OrderExecutor
}

var GlobalPriceMonitor *PriceMonitor

// InitPriceMonitor 初始化价格监控器
func InitPriceMonitor(binanceClient *binance.Binance) {
	GlobalPriceMonitor = &PriceMonitor{
		running:       false,
		stopChan:      make(chan bool),
		tickInterval:  1 * time.Second,
		orderExecutor: NewOrderExecutor(binanceClient),
	}
}

// Start 开始价格监控
func (pm *PriceMonitor) Start() {
	if pm.running {
		logrus.Warn("price monitor is already running")
		return
	}

	pm.running = true
	logrus.Info("price monitor started")

	go pm.monitorLoop()
}

// Stop 停止价格监控
func (pm *PriceMonitor) Stop() {
	if !pm.running {
		return
	}

	pm.running = false
	pm.stopChan <- true
	logrus.Info("价格监控已停止")

	// 发送Telegram通知
	if telegram.GlobalTelegramClient != nil {
		err := telegram.GlobalTelegramClient.SendMessage("监控停止")
		if err != nil {
			logrus.Errorf("发送Telegram通知失败: %v", err)
		}
	}
}

// IsRunning 检查是否在运行
func (pm *PriceMonitor) IsRunning() bool {
	return pm.running
}

// monitorLoop 监控循环
func (pm *PriceMonitor) monitorLoop() {
	ticker := time.NewTicker(pm.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pm.stopChan:
			return
		case <-ticker.C:
			pm.checkPriceTargets()
		}
	}
}

// checkPriceTargets 检查价格目标
func (pm *PriceMonitor) checkPriceTargets() {
	// 获取所有待处理的价格预估
	estimates, err := redis.GlobalRedisClient.GetActiveEstimates()
	if err != nil {
		logrus.Errorf("获取价格预估失败: %v", err)
		return
	}

	if len(estimates) == 0 {
		return
	}

	logrus.Debugf("检查 %d 个价格预估", len(estimates))

	for i := range estimates {
		estimate := estimates[i]
		pm.checkSingleEstimate(estimate)
	}
}

// checkSingleEstimate 检查单个价格预估
func (pm *PriceMonitor) checkSingleEstimate(estimate *models.PriceEstimate) {
	// 获取标记价格
	markPriceData, err := redis.GlobalRedisClient.GetMarkPrice(estimate.Symbol)
	if err != nil {
		logrus.Debugf("未找到 %s 的标记价格数据", estimate.Symbol)
		return
	}

	if markPriceData == nil {
		logrus.Debugf("标记价格数据为空 %s", estimate.Symbol)
		return
	}

	// 使用标记价格作为当前价格
	currentPrice := markPriceData.MarkPrice
	if currentPrice <= 0 {
		logrus.Errorf("无效的标记价格 %s: %f", estimate.Symbol, currentPrice)
		return
	}

	// 根据操作类型和交易方向判断触发条件
	actionType := estimate.ActionType
	triggerType := estimate.TriggerType

	// 统一使用markPrice
	var shouldTrigger bool
	switch estimate.Side {
	case types.PositionSideLong:
		shouldTrigger = shouldTriggerLong(actionType, triggerType, currentPrice, estimate.TargetPrice)
	case types.PositionSideShort:
		shouldTrigger = shouldTriggerShort(actionType, triggerType, currentPrice, estimate.TargetPrice)
	default:
		logrus.Errorf("无效的交易方向: %s", estimate.Side)
		return
	}

	if shouldTrigger {
		logrus.Infof("价格目标触发: %s %s %s, 当前标记价格: %f, 目标价格: %f",
			estimate.Symbol, estimate.Side, actionType, currentPrice, estimate.TargetPrice)

		pm.triggerEstimate(estimate, currentPrice)
	}
}

// triggerEstimate 触发价格预估
func (pm *PriceMonitor) triggerEstimate(estimate *models.PriceEstimate, currentPrice float64) {
	// 执行自动下单
	err := pm.orderExecutor.ExecuteOrder(estimate, currentPrice)
	if err != nil {
		logrus.Errorf("双向持仓订单执行失败: %v", err)

		// 发送错误通知，包含详细的交易信息
		if telegram.GlobalTelegramClient != nil {
			// 构建详细的错误消息
			actionText := getActionText(estimate.ActionType)
			positionText := getPositionText(estimate.Side)

			errorMessage := fmt.Sprintf("双向持仓自动下单 - %s %s %s\n数量: %.6f\n目标价: %.4f\n当前价: %.6f",
				estimate.Symbol, actionText, positionText,
				estimate.Quantity, estimate.TargetPrice, currentPrice)

			telegram.GlobalTelegramClient.SendError(errorMessage, err)
		}

		// 更新预估状态为失败
		estimate.Status = models.EstimateStatusFailed
	} else {
		// 更新预估状态为已触发
		estimate.Status = models.EstimateStatusTriggered

		// 广播预估触发事件
		go pm.broadcastEstimateTriggerEvent(estimate, currentPrice)
	}

	estimate.UpdatedAt = time.Now()
	err = redis.GlobalRedisClient.SetPriceEstimate(estimate)
	if err != nil {
		logrus.Errorf("更新价格预估状态失败: %v", err)
		return
	}

	// 通过WebSocket广播价格预估更新
	go utils.BroadcastSymbolEstimatesUpdate()
}

// broadcastEstimateTriggerEvent 广播预估触发事件到WebSocket客户端
func (pm *PriceMonitor) broadcastEstimateTriggerEvent(estimate *models.PriceEstimate, currentPrice float64) {
	// 获取WebSocket管理器
	wsManager := websocket.GetGlobalWebSocketManager()
	if wsManager == nil {
		return
	}

	// 准备事件数据
	event := map[string]interface{}{
		"event_type": "estimate_trigger",
		"timestamp":  time.Now().Unix(),
		"data": map[string]interface{}{
			"type": "estimate_trigger",
			"message": fmt.Sprintf("预估价已触发: %s %s %.4f -> %.4f",
				estimate.Symbol, estimate.Side, estimate.TargetPrice, currentPrice),
			"data": map[string]interface{}{
				"estimate_id":   estimate.ID,
				"symbol":        estimate.Symbol,
				"side":          estimate.Side,
				"action_type":   estimate.ActionType,
				"target_price":  estimate.TargetPrice,
				"current_price": currentPrice,
				"trigger_time":  time.Now().Unix(),
			},
		},
	}

	// 广播事件
	wsManager.BroadcastEvent(event)
	logrus.Infof("通过WebSocket广播预估触发事件: %s %s %.4f",
		estimate.Symbol, estimate.Side, estimate.TargetPrice)
}

// getActionText 获取操作类型的中文描述
func getActionText(actionType string) string {
	switch actionType {
	case models.ActionTypeOpen:
		return "开仓"
	case models.ActionTypeClose:
		return "平仓"
	case models.ActionTypeAddition:
		return "加仓"
	case models.ActionTypeTakeProfit:
		return "止盈"
	case models.ActionTypeStopLoss:
		return "止损"
	default:
		return "交易"
	}
}

// getPositionText 获取仓位方向的中文描述
func getPositionText(side string) string {
	switch side {
	case types.PositionSideLong:
		return "做多"
	case types.PositionSideShort:
		return "做空"
	default:
		return "未知"
	}
}
