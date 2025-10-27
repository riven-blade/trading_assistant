package core

import (
	"context"
	"fmt"
	"sync"
	"time"
	"trading_assistant/models"
	"trading_assistant/pkg/exchanges/binance"
	"trading_assistant/pkg/telegram"

	"github.com/sirupsen/logrus"
)

// OrderTask 订单任务
type OrderTask struct {
	Estimate     *models.PriceEstimate
	CurrentPrice float64
	TriggerTime  time.Time
}

// OrderQueue 订单队列管理器
type OrderQueue struct {
	taskChan      chan *OrderTask
	orderExecutor *OrderExecutor
	ctx           context.Context
	cancel        context.CancelFunc
	running       bool
	mutex         sync.RWMutex
	waitDuration  time.Duration
}

// GlobalOrderQueue 全局订单队列实例
var GlobalOrderQueue *OrderQueue

// InitOrderQueue 初始化订单队列
func InitOrderQueue(binanceClient *binance.Binance) {
	ctx, cancel := context.WithCancel(context.Background())

	GlobalOrderQueue = &OrderQueue{
		taskChan:      make(chan *OrderTask, 100), // 缓冲区大小为100
		orderExecutor: NewOrderExecutor(binanceClient),
		ctx:           ctx,
		cancel:        cancel,
		running:       false,
		waitDuration:  1 * time.Second, // 默认等待5秒
	}
}

// Start 启动订单队列处理器
func (oq *OrderQueue) Start() error {
	oq.mutex.Lock()
	defer oq.mutex.Unlock()

	if oq.running {
		return fmt.Errorf("订单队列已在运行")
	}

	oq.running = true
	logrus.Info("启动订单队列处理器")

	// 发送Telegram通知
	if telegram.GlobalTelegramClient != nil {
		err := telegram.GlobalTelegramClient.SendMessage("订单队列处理器已启动")
		if err != nil {
			logrus.Errorf("发送Telegram通知失败: %v", err)
		}
	}

	// 启动处理goroutine
	go oq.processQueue()

	return nil
}

// Stop 停止订单队列处理器
func (oq *OrderQueue) Stop() {
	oq.mutex.Lock()
	defer oq.mutex.Unlock()

	if !oq.running {
		return
	}

	logrus.Info("停止订单队列处理器...")
	oq.running = false
	oq.cancel()

	// 发送Telegram通知
	if telegram.GlobalTelegramClient != nil {
		err := telegram.GlobalTelegramClient.SendMessage("订单队列处理器已停止")
		if err != nil {
			logrus.Errorf("发送Telegram通知失败: %v", err)
		}
	}

	logrus.Info("订单队列处理器已停止")
}

// IsRunning 检查队列是否在运行
func (oq *OrderQueue) IsRunning() bool {
	oq.mutex.RLock()
	defer oq.mutex.RUnlock()
	return oq.running
}

// SetWaitDuration 设置等待时间
func (oq *OrderQueue) SetWaitDuration(duration time.Duration) {
	oq.mutex.Lock()
	defer oq.mutex.Unlock()
	oq.waitDuration = duration
	logrus.Infof("订单队列等待时间已设置为: %v", duration)
}

// GetWaitDuration 获取等待时间
func (oq *OrderQueue) GetWaitDuration() time.Duration {
	oq.mutex.RLock()
	defer oq.mutex.RUnlock()
	return oq.waitDuration
}

// EnqueueOrder 将订单任务加入队列
func (oq *OrderQueue) EnqueueOrder(estimate *models.PriceEstimate, currentPrice float64) error {
	oq.mutex.RLock()
	running := oq.running
	oq.mutex.RUnlock()

	if !running {
		return fmt.Errorf("订单队列未启动")
	}

	task := &OrderTask{
		Estimate:     estimate,
		CurrentPrice: currentPrice,
		TriggerTime:  time.Now(),
	}

	// 非阻塞发送到队列
	select {
	case oq.taskChan <- task:
		logrus.WithFields(logrus.Fields{
			"symbol":        estimate.Symbol,
			"action_type":   estimate.ActionType,
			"side":          estimate.Side,
			"target_price":  estimate.TargetPrice,
			"current_price": currentPrice,
			"queue_time":    task.TriggerTime.Format("15:04:05"),
		}).Info("订单任务已加入队列")
		return nil
	default:
		// 队列满了，拒绝任务
		logrus.WithFields(logrus.Fields{
			"symbol":        estimate.Symbol,
			"action_type":   estimate.ActionType,
			"side":          estimate.Side,
			"target_price":  estimate.TargetPrice,
			"current_price": currentPrice,
		}).Error("订单队列已满，拒绝任务")
		return fmt.Errorf("订单队列已满")
	}
}

// GetQueueSize 获取队列长度
func (oq *OrderQueue) GetQueueSize() int {
	return len(oq.taskChan)
}

// GetStatus 获取队列状态
func (oq *OrderQueue) GetStatus() map[string]interface{} {
	oq.mutex.RLock()
	defer oq.mutex.RUnlock()

	return map[string]interface{}{
		"running":       oq.running,
		"queue_size":    len(oq.taskChan),
		"wait_duration": oq.waitDuration.String(),
		"buffer_size":   cap(oq.taskChan),
	}
}

// processQueue 处理队列中的订单任务
func (oq *OrderQueue) processQueue() {
	logrus.Info("订单队列处理器开始运行")

	for {
		select {
		case <-oq.ctx.Done():
			logrus.Info("订单队列处理器收到停止信号")
			return

		case task := <-oq.taskChan:
			oq.processOrderTask(task)
		}
	}
}

// processOrderTask 处理单个订单任务
func (oq *OrderQueue) processOrderTask(task *OrderTask) {
	// 记录开始处理时间
	processStartTime := time.Now()
	waitTime := processStartTime.Sub(task.TriggerTime)

	logrus.WithFields(logrus.Fields{
		"symbol":        task.Estimate.Symbol,
		"action_type":   task.Estimate.ActionType,
		"side":          task.Estimate.Side,
		"target_price":  task.Estimate.TargetPrice,
		"current_price": task.CurrentPrice,
		"trigger_time":  task.TriggerTime.Format("15:04:05"),
		"process_time":  processStartTime.Format("15:04:05"),
		"wait_in_queue": waitTime.String(),
	}).Info("开始处理订单任务")

	oq.mutex.RLock()
	waitDuration := oq.waitDuration
	oq.mutex.RUnlock()

	logrus.WithFields(logrus.Fields{
		"symbol":        task.Estimate.Symbol,
		"wait_duration": waitDuration.String(),
	}).Info("订单执行前等待...")

	// 使用context控制的等待，支持优雅停机
	select {
	case <-oq.ctx.Done():
		logrus.WithFields(logrus.Fields{
			"symbol": task.Estimate.Symbol,
		}).Info("订单处理被中断（队列停止）")
		return
	case <-time.After(waitDuration):
		// 等待时间结束，继续执行
	}

	// 执行订单
	logrus.WithFields(logrus.Fields{
		"symbol":       task.Estimate.Symbol,
		"action_type":  task.Estimate.ActionType,
		"side":         task.Estimate.Side,
		"target_price": task.Estimate.TargetPrice,
	}).Info("开始执行订单...")

	err := oq.orderExecutor.ExecuteOrder(task.Estimate, task.CurrentPrice)

	processEndTime := time.Now()
	totalProcessTime := processEndTime.Sub(processStartTime)

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"symbol":             task.Estimate.Symbol,
			"action_type":        task.Estimate.ActionType,
			"side":               task.Estimate.Side,
			"error":              err.Error(),
			"total_process_time": totalProcessTime.String(),
		}).Error("订单执行失败")

		// 发送错误通知
		if telegram.GlobalTelegramClient != nil {
			actionText := getQueueActionText(task.Estimate.ActionType)
			positionText := getQueuePositionText(task.Estimate.Side)

			errorMessage := fmt.Sprintf("订单执行失败: %s %s %s %.6f @ %.4f\n错误: %s",
				task.Estimate.Symbol, actionText, positionText,
				task.Estimate.Quantity, task.CurrentPrice, err.Error())

			telegram.GlobalTelegramClient.SendError(errorMessage, err)
		}
	} else {
		logrus.WithFields(logrus.Fields{
			"symbol":             task.Estimate.Symbol,
			"action_type":        task.Estimate.ActionType,
			"side":               task.Estimate.Side,
			"total_process_time": totalProcessTime.String(),
			"wait_in_queue":      waitTime.String(),
			"wait_before_exec":   waitDuration.String(),
		}).Info("订单执行成功")

		// 发送成功通知
		if telegram.GlobalTelegramClient != nil {
			actionText := getQueueActionText(task.Estimate.ActionType)
			positionText := getQueuePositionText(task.Estimate.Side)

			message := fmt.Sprintf("%s %s %s %.6f @ %.4f",
				task.Estimate.Symbol, actionText, positionText,
				task.Estimate.Quantity, task.CurrentPrice)

			err := telegram.GlobalTelegramClient.SendMessage(message)
			if err != nil {
				logrus.Errorf("发送订单成功通知失败: %v", err)
			}
		}
	}
}
