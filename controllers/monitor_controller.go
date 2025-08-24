package controllers

import (
	"fmt"
	"net/http"
	"trading_assistant/core"
	"trading_assistant/pkg/exchanges"
	"trading_assistant/pkg/redis"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type MonitorController struct{}

// GetPositions 获取持仓信息
func (m *MonitorController) GetPositions(ctx *gin.Context) {
	if exchanges.GlobalBinanceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Binance客户端未初始化",
		})
		return
	}

	// 使用交易所客户端获取持仓信息
	positions, err := exchanges.GlobalBinanceClient.GetPositions()
	if err != nil {
		logrus.Errorf("获取持仓信息失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取持仓信息失败: " + err.Error(),
		})
		return
	}

	// 同时更新Redis中的持仓数据
	if redis.GlobalRedisClient != nil {
		for i := range positions {
			position := positions[i]
			if err := redis.GlobalRedisClient.SetPosition(position); err != nil {
				logrus.Errorf("更新Redis持仓数据失败: %v", err)
			}
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data":  positions,
		"total": len(positions),
	})
}

// GetBalances 获取余额信息
func (m *MonitorController) GetBalances(ctx *gin.Context) {
	if exchanges.GlobalBinanceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Binance客户端未初始化",
		})
		return
	}

	// 获取余额信息
	balances, err := exchanges.GlobalBinanceClient.GetBalances()
	if err != nil {
		logrus.Errorf("获取余额信息失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取余额信息失败: " + err.Error(),
		})
		return
	}

	var totalValue float64 = 0
	var usdtAmount float64 = 0
	var usdtFree float64 = 0
	var otherAssetsValue float64 = 0

	assetDetails := make([]map[string]interface{}, 0)

	for i := range balances {
		balance := balances[i]
		if balance.Total <= 0 {
			continue
		}

		if balance.Asset == "USDT" {
			// USDT直接统计
			usdtAmount = balance.Total
			usdtFree = balance.Free
			totalValue += balance.Total

			assetDetails = append(assetDetails, map[string]interface{}{
				"asset":      "USDT",
				"amount":     balance.Total,
				"free":       balance.Free,
				"locked":     balance.Locked,
				"price_usdt": 1.0,
				"value_usdt": balance.Total,
			})
		}
	}

	// 获取持仓信息
	positions, err := redis.GlobalRedisClient.GetAllPositions()
	totalPnl := 0.0
	if err == nil {
		for _, pos := range positions {
			totalPnl += pos.UnrealizedPnl
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"total_value":        totalValue,
			"usdt_total":         usdtAmount,
			"usdt_free":          usdtFree,
			"other_assets_value": otherAssetsValue,
			"total_pnl":          totalPnl,
			"asset_details":      assetDetails,
			"net_value":          totalValue + totalPnl,
		},
		"total": len(balances),
	})
}

// GetAccountStatus 获取账户状态（包含WebSocket状态）
func (m *MonitorController) GetAccountStatus(ctx *gin.Context) {
	status := make(map[string]interface{})

	// 基础连接状态
	status["redis_connected"] = redis.GlobalRedisClient != nil
	status["binance_connected"] = exchanges.GlobalBinanceClient != nil
	status["websocket_active"] = exchanges.GlobalWebSocketManager != nil

	// 账户监控状态
	if core.GlobalAccountManager != nil {
		accountStatus := core.GlobalAccountManager.GetAccountStatus()
		for key, value := range accountStatus {
			status[key] = value
		}
	}

	// 价格监控状态
	if core.GlobalPriceMonitor != nil {
		status["price_monitor_running"] = core.GlobalPriceMonitor.IsRunning()
	}

	// 获取选中的币种数量
	selectedCoins, err := redis.GlobalRedisClient.GetSelectedCoins()
	if err != nil {
		status["selected_coins"] = 0
		status["error"] = "获取选中币种失败"
	} else {
		status["selected_coins"] = len(selectedCoins)
	}

	// 获取待处理的价格预估数量
	pendingEstimates, err := redis.GlobalRedisClient.GetActiveEstimates()
	if err != nil {
		status["pending_estimates"] = 0
	} else {
		status["pending_estimates"] = len(pendingEstimates)
	}

	ctx.JSON(http.StatusOK, status)
}

// GetOrders 获取订单信息
func (m *MonitorController) GetOrders(ctx *gin.Context) {
	if exchanges.GlobalBinanceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Binance客户端未初始化",
		})
		return
	}

	// 可选的交易对过滤参数
	symbol := ctx.Query("symbol")

	orders, err := exchanges.GlobalBinanceClient.GetOrders(symbol)
	if err != nil {
		logrus.Errorf("获取订单信息失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取订单信息失败: " + err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data":  orders,
		"total": len(orders),
	})
}

// GetOrderBook 获取指定币种的订单薄
func (m *MonitorController) GetOrderBook(ctx *gin.Context) {
	symbol := ctx.Param("symbol")

	orderBook, err := redis.GlobalRedisClient.GetOrderBook(symbol)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{
			"error": "未找到订单薄数据",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data": orderBook,
	})
}

// RefreshCache 手动刷新所有缓存
func (m *MonitorController) RefreshCache(ctx *gin.Context) {
	if exchanges.GlobalBinanceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Binance客户端未初始化",
		})
		return
	}

	if redis.GlobalRedisClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Redis客户端未初始化",
		})
		return
	}

	// 清理所有交易相关缓存
	if err := redis.GlobalRedisClient.ClearAllTradingCache(); err != nil {
		logrus.Errorf("清理缓存失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "清理缓存失败",
		})
		return
	}

	// 预热缓存 - 重新获取一次数据
	refreshResults := make(map[string]interface{})

	// 获取持仓信息（会自动缓存）
	positions, err := exchanges.GlobalBinanceClient.GetPositions()
	if err != nil {
		refreshResults["positions"] = "获取失败: " + err.Error()
	} else {
		refreshResults["positions"] = fmt.Sprintf("成功刷新 %d 个持仓", len(positions))
	}

	// 获取余额信息（会自动缓存）
	balances, err := exchanges.GlobalBinanceClient.GetBalances()
	if err != nil {
		refreshResults["balances"] = "获取失败: " + err.Error()
	} else {
		refreshResults["balances"] = fmt.Sprintf("成功刷新 %d 个资产余额", len(balances))
	}

	// 获取订单信息（会自动缓存）
	orders, err := exchanges.GlobalBinanceClient.GetOrders("")
	if err != nil {
		refreshResults["orders"] = "获取失败: " + err.Error()
	} else {
		refreshResults["orders"] = fmt.Sprintf("成功刷新 %d 个订单", len(orders))
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "缓存刷新完成",
		"results": refreshResults,
	})
}
