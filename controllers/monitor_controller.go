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

// calculateBalanceSummary 计算余额汇总信息
func calculateBalanceSummary(account *types.Account) map[string]interface{} {
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

		// 计算USDT价值（非USDT资产使用标记价格转换）
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

type MonitorController struct {
	binanceClient *binance.Binance
}

// NewMonitorController 创建监控控制器
func NewMonitorController(binanceClient *binance.Binance) *MonitorController {
	return &MonitorController{
		binanceClient: binanceClient,
	}
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

// GetPositions 获取持仓信息
func (m *MonitorController) GetPositions(ctx *gin.Context) {
	// 优先从Redis缓存获取持仓数据
	if redis.GlobalRedisClient != nil {
		var cachedPositions []*types.Position
		cacheKey := redis.CacheKeyPositions
		if err := redis.GlobalRedisClient.GetCache(cacheKey, &cachedPositions); err == nil {
			// 转换为前端需要的格式
			positions := make([]*models.Position, 0, len(cachedPositions))
			for i := range cachedPositions {
				pos := cachedPositions[i]

				// 转换保证金模式格式
				marginMode := types.MarginModeCross
				if pos.MarginType == types.MarginModeIsolated {
					marginMode = types.MarginModeIsolated
				}

				position := &models.Position{
					Symbol:            pos.Symbol,
					Side:              strings.ToUpper(pos.Side),
					Size:              pos.Size,
					EntryPrice:        pos.EntryPrice,
					MarkPrice:         pos.MarkPrice,
					UnrealizedPnl:     pos.UnrealizedPnl,
					Leverage:          int(pos.Leverage),
					MarginMode:        marginMode,
					IsolatedMargin:    pos.IsolatedMargin,
					InitialMargin:     pos.InitialMargin,
					MaintenanceMargin: pos.MaintenanceMargin,
					Notional:          pos.NotionalValue,
					UpdatedAt:         time.UnixMilli(pos.Timestamp),
				}
				positions = append(positions, position)
			}

			ctx.JSON(http.StatusOK, gin.H{
				"data":   positions,
				"cached": true,
				"source": "redis_cache",
			})
			return
		}

		// 尝试从Redis持仓操作中获取
		if allPositions, err := redis.GlobalRedisClient.GetAllPositions(); err == nil {
			ctx.JSON(http.StatusOK, gin.H{
				"data":   allPositions,
				"cached": true,
				"source": "redis_positions",
			})
			return
		}
	}

	// 缓存中没有数据，返回空数据
	ctx.JSON(http.StatusOK, gin.H{
		"data":    []*models.Position{},
		"cached":  false,
		"source":  "no_cache",
		"message": "持仓数据不在缓存中，请等待自动刷新",
	})
}

// GetBalances 获取余额信息
func (m *MonitorController) GetBalances(ctx *gin.Context) {
	// 优先从Redis获取实时余额汇总
	var balanceSummary interface{}
	if redis.GlobalRedisClient != nil {
		if err := redis.GlobalRedisClient.GetBalancesRealtime(&balanceSummary); err == nil {
			ctx.JSON(http.StatusOK, gin.H{
				"data":   balanceSummary,
				"cached": true,
				"source": "realtime_cache",
			})
			return
		}
	}

	// 尝试从普通缓存获取详细余额
	if redis.GlobalRedisClient != nil {
		var cachedAccount *types.Account
		cacheKey := redis.CacheKeyBalances
		if err := redis.GlobalRedisClient.GetCache(cacheKey, &cachedAccount); err == nil {
			// 计算余额汇总
			summary := calculateBalanceSummary(cachedAccount)
			ctx.JSON(http.StatusOK, gin.H{
				"data":   summary,
				"cached": true,
				"source": "detailed_cache",
			})
			return
		}
	}

	// 缓存中没有数据，返回空数据
	ctx.JSON(http.StatusOK, gin.H{
		"data": map[string]interface{}{
			"total_value":        0.0,
			"usdt_total":         0.0,
			"usdt_free":          0.0,
			"other_assets_value": 0.0,
			"total_pnl":          0.0,
			"net_value":          0.0,
		},
		"cached":  false,
		"source":  "no_cache",
		"message": "余额数据不在缓存中，请等待自动刷新",
	})
}

// GetOrderbook 获取订单簿数据
func (m *MonitorController) GetOrderbook(ctx *gin.Context) {
	symbol := ctx.Param("symbol")
	if symbol == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "交易对参数不能为空",
		})
		return
	}

	// 标准化交易对格式
	symbol = strings.ToUpper(symbol)

	// 从Redis获取订单簿数据
	orderbook, err := redis.GlobalRedisClient.GetOrderBook(symbol)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{
			"error":  "未找到订单簿数据",
			"symbol": symbol,
			"detail": err.Error(),
		})
		return
	}

	// 检查数据时效性
	if orderbook.Timestamp > 0 {
		now := time.Now().UnixMilli()
		age := now - orderbook.Timestamp

		// 如果数据超过30秒，标记为过时
		if age > 30000 {
			ctx.JSON(http.StatusOK, gin.H{
				"data":   orderbook,
				"stale":  true,
				"age_ms": age,
			})
			return
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data": orderbook,
	})
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
