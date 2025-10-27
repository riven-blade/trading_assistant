package controllers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"trading_assistant/core"
	"trading_assistant/models"
	"trading_assistant/pkg/exchanges/binance"
	"trading_assistant/pkg/exchanges/types"
	"trading_assistant/pkg/redis"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type MonitorController struct {
	binanceClient *binance.Binance
}

// NewMonitorController 创建监控控制器
func NewMonitorController(binanceClient *binance.Binance) *MonitorController {
	return &MonitorController{
		binanceClient: binanceClient,
	}
}

// GetOrders 获取订单信息
func (m *MonitorController) GetOrders(ctx *gin.Context) {
	if m.binanceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Binance客户端未初始化",
		})
		return
	}

	symbol := ctx.Query("symbol")

	// 缓存键
	cacheKey := fmt.Sprintf("%s:%s", redis.CacheKeyOrders, symbol)

	// 检查1分钟缓存
	var cachedOrders []*models.Order
	if redis.GlobalRedisClient != nil {
		if err := redis.GlobalRedisClient.GetCache(cacheKey, &cachedOrders); err == nil {
			logrus.Debugf("从缓存获取订单数据: %s", cacheKey)
			ctx.JSON(http.StatusOK, gin.H{
				"data":   cachedOrders,
				"cached": true,
				"source": "cache",
			})
			return
		}
	}

	// 缓存中没有数据，实时获取
	logrus.Debugf("缓存中无订单数据，实时获取: %s", symbol)
	exchangeOrders, err := m.binanceClient.FetchOrders(context.Background(), symbol, 0, 100, nil)
	if err != nil {
		logrus.Errorf("获取订单数据失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取订单数据失败: " + err.Error(),
		})
		return
	}

	// 转换为models.Order格式
	orders := m.convertOrders(exchangeOrders)

	// 缓存数据
	if redis.GlobalRedisClient != nil {
		if err := redis.GlobalRedisClient.SetCacheWithExpiration(cacheKey, orders, redis.CacheExpirationOrders); err != nil {
			logrus.Errorf("缓存订单数据失败: %v", err)
		} else {
			logrus.Debugf("已缓存订单数据1分钟: %s", cacheKey)
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data":   orders,
		"cached": false,
		"source": "real_time",
	})
}

// convertOrders 转换订单格式
func (m *MonitorController) convertOrders(exchangeOrders []*types.Order) []*models.Order {
	orders := make([]*models.Order, 0, len(exchangeOrders))

	for _, exOrder := range exchangeOrders {
		order := &models.Order{
			ID:           exOrder.ID,
			Symbol:       exOrder.Symbol,
			Side:         strings.ToUpper(exOrder.Side),
			PositionSide: strings.ToUpper(exOrder.PositionSide), // 关键修复：添加持仓方向
			Type:         strings.ToUpper(exOrder.Type),
			Quantity:     exOrder.Amount,
			ExecutedQty:  exOrder.Filled,
			Price:        exOrder.Price,
			Status:       strings.ToUpper(exOrder.Status),
			ExchangeID:   exOrder.ID,
			CreatedAt:    time.UnixMilli(exOrder.Timestamp),
			UpdatedAt:    time.UnixMilli(exOrder.LastTradeTimestamp),
		}
		orders = append(orders, order)
	}

	return orders
}

// CancelOrder 取消订单
func (m *MonitorController) CancelOrder(ctx *gin.Context) {
	if m.binanceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": "Binance客户端未初始化",
		})
		return
	}

	// 从请求体获取所有参数
	var requestBody struct {
		OrderID    string `json:"order_id"`
		ExchangeID string `json:"exchange_id"`
		Symbol     string `json:"symbol"`
	}

	if err := ctx.ShouldBindJSON(&requestBody); err != nil {
		logrus.Warnf("取消订单参数解析失败: %v", err)
		ctx.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数格式错误",
		})
		return
	}

	if requestBody.OrderID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "订单ID不能为空",
		})
		return
	}

	if requestBody.ExchangeID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "交易所订单ID不能为空",
		})
		return
	}

	logrus.Infof("准备取消订单: orderID=%s, exchangeID=%s, symbol=%s", requestBody.OrderID, requestBody.ExchangeID, requestBody.Symbol)

	// 调用币安API取消订单
	err := m.binanceClient.CancelOrder(context.Background(), requestBody.Symbol, requestBody.ExchangeID, "")
	if err != nil {
		logrus.Errorf("取消订单失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("取消订单失败: %v", err),
		})
		return
	}

	logrus.Infof("订单取消成功: orderID=%s, exchangeID=%s", requestBody.OrderID, requestBody.ExchangeID)

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "订单取消成功",
		"data": gin.H{
			"order_id":    requestBody.OrderID,
			"exchange_id": requestBody.ExchangeID,
		},
	})
}

// GetOrderQueueStatus 获取订单队列状态
func (m *MonitorController) GetOrderQueueStatus(ctx *gin.Context) {
	if core.GlobalOrderQueue == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "订单队列未初始化",
		})
		return
	}

	status := core.GlobalOrderQueue.GetStatus()
	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
	})
}

// StartOrderQueue 启动订单队列
func (m *MonitorController) StartOrderQueue(ctx *gin.Context) {
	if core.GlobalOrderQueue == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "订单队列未初始化",
		})
		return
	}

	if core.GlobalOrderQueue.IsRunning() {
		ctx.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "订单队列已在运行",
		})
		return
	}

	err := core.GlobalOrderQueue.Start()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("启动订单队列失败: %v", err),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "订单队列启动成功",
	})
}

// StopOrderQueue 停止订单队列
func (m *MonitorController) StopOrderQueue(ctx *gin.Context) {
	if core.GlobalOrderQueue == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "订单队列未初始化",
		})
		return
	}

	if !core.GlobalOrderQueue.IsRunning() {
		ctx.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "订单队列已停止",
		})
		return
	}

	core.GlobalOrderQueue.Stop()
	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "订单队列停止成功",
	})
}

// SetOrderQueueWaitTime 设置订单队列等待时间
func (m *MonitorController) SetOrderQueueWaitTime(ctx *gin.Context) {
	if core.GlobalOrderQueue == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "订单队列未初始化",
		})
		return
	}

	secondsStr := ctx.Query("seconds")
	if secondsStr == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "请提供等待时间（秒）",
		})
		return
	}

	seconds, err := strconv.Atoi(secondsStr)
	if err != nil || seconds < 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "等待时间必须是非负整数",
		})
		return
	}

	duration := time.Duration(seconds) * time.Second
	core.GlobalOrderQueue.SetWaitDuration(duration)

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("订单队列等待时间已设置为%d秒", seconds),
		"data": gin.H{
			"wait_duration": duration.String(),
		},
	})
}
