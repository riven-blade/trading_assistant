package controllers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
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
