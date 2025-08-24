package core

import (
	"strconv"
	"time"
	"trading_assistant/models"
	"trading_assistant/pkg/redis"
	"trading_assistant/pkg/telegram"

	"github.com/sirupsen/logrus"
)

type PriceMonitor struct {
	running      bool
	stopChan     chan bool
	tickInterval time.Duration
}

var GlobalPriceMonitor *PriceMonitor

// InitPriceMonitor 初始化价格监控器
func InitPriceMonitor() {
	GlobalPriceMonitor = &PriceMonitor{
		running:      false,
		stopChan:     make(chan bool),
		tickInterval: 1 * time.Second,
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
		telegram.GlobalTelegramClient.SendMessage("price monitor started")
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
		telegram.GlobalTelegramClient.SendMessage("app listening stopped")
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
	// 获取当前订单薄
	orderBook, err := redis.GlobalRedisClient.GetOrderBook(estimate.Symbol)
	if err != nil {
		logrus.Debugf("未找到 %s 的订单薄数据", estimate.Symbol)
		return
	}

	// 获取最优价格
	if len(orderBook.Bids) == 0 || len(orderBook.Asks) == 0 {
		logrus.Errorf("订单薄数据不完整 %s", estimate.Symbol)
		return
	}

	bestBid, err := strconv.ParseFloat(orderBook.Bids[0].Price, 64)
	if err != nil {
		logrus.Errorf("解析最优买价失败 %s: %v", estimate.Symbol, err)
		return
	}

	bestAsk, err := strconv.ParseFloat(orderBook.Asks[0].Price, 64)
	if err != nil {
		logrus.Errorf("解析最优卖价失败 %s: %v", estimate.Symbol, err)
		return
	}

	var currentPrice float64
	var shouldTrigger bool

	// 根据操作类型和交易方向判断触发条件
	actionType := estimate.ActionType
	triggerType := estimate.TriggerType
	createdBy := estimate.CreatedBy

	switch estimate.Side {
	case "long":
		currentPrice = bestAsk // 做多关注卖价
		shouldTrigger = shouldTriggerLong(actionType, triggerType, createdBy, currentPrice, estimate.TargetPrice)
	case "short":
		currentPrice = bestBid // 做空关注买价
		shouldTrigger = shouldTriggerShort(actionType, triggerType, createdBy, currentPrice, estimate.TargetPrice)
	default:
		logrus.Errorf("无效的交易方向: %s", estimate.Side)
		return
	}

	if shouldTrigger {
		logrus.Infof("价格目标触发: %s %s %s(%s), 当前价格: %f, 目标价格: %f",
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
	err := executeOrder(estimate, currentPrice)
	if err != nil {
		logrus.Errorf("执行订单失败: %v", err)

		// 发送错误通知
		if telegram.GlobalTelegramClient != nil {
			telegram.GlobalTelegramClient.SendError("自动下单", err)
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
