package core

import (
	"time"
	"trading_assistant/models"
	"trading_assistant/pkg/exchanges/binance"
	"trading_assistant/pkg/redis"
	"trading_assistant/pkg/telegram"

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

	// 发送Telegram通知
	if telegram.GlobalTelegramClient != nil {
		err := telegram.GlobalTelegramClient.SendMessage("price monitor started")
		if err != nil {
			logrus.Errorf("发送Telegram通知失败: %v", err)
		}
	}

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
		err := telegram.GlobalTelegramClient.SendMessage("app listening stopped")
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
	createdBy := estimate.CreatedBy

	// 统一使用markPrice
	var shouldTrigger bool
	switch estimate.Side {
	case "long":
		shouldTrigger = shouldTriggerLong(actionType, triggerType, createdBy, currentPrice, estimate.TargetPrice)
	case "short":
		shouldTrigger = shouldTriggerShort(actionType, triggerType, createdBy, currentPrice, estimate.TargetPrice)
	default:
		logrus.Errorf("无效的交易方向: %s", estimate.Side)
		return
	}

	if shouldTrigger {
		logrus.Infof("价格目标触发: %s %s %s(%s), 当前标记价格: %f, 目标价格: %f",
			estimate.Symbol, estimate.Side, actionType, createdBy, currentPrice, estimate.TargetPrice)

		pm.triggerEstimate(estimate, currentPrice)
	}
}

// triggerEstimate 触发价格预估
func (pm *PriceMonitor) triggerEstimate(estimate *models.PriceEstimate, currentPrice float64) {
	// 发送价格警报
	if telegram.GlobalTelegramClient != nil {
		err := telegram.GlobalTelegramClient.SendPriceAlert(
			estimate.Symbol, currentPrice, estimate.TargetPrice, estimate.Side)
		if err != nil {
			logrus.Errorf("发送价格警报失败: %v", err)
		}
	}

	// 执行自动下单
	err := pm.orderExecutor.ExecuteOrder(estimate, currentPrice)
	if err != nil {
		logrus.Errorf("双向持仓订单执行失败: %v", err)

		// 发送错误通知
		if telegram.GlobalTelegramClient != nil {
			telegram.GlobalTelegramClient.SendError("双向持仓自动下单", err)
		}

		// 更新预估状态为失败
		estimate.Status = models.EstimateStatusFailed
	} else {
		// 更新预估状态为已触发
		estimate.Status = models.EstimateStatusTriggered
	}

	estimate.UpdatedAt = time.Now()
	err = redis.GlobalRedisClient.SetPriceEstimate(estimate)
	if err != nil {
		logrus.Errorf("更新价格预估状态失败: %v", err)
		return
	}
}
