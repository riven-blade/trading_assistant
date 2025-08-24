package controllers

import (
	"net/http"
	"time"
	"trading_assistant/models"
	"trading_assistant/pkg/exchanges"
	"trading_assistant/pkg/redis"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type CoinController struct{}

// GetCoins 获取所有币种
func (c *CoinController) GetCoins(ctx *gin.Context) {
	coins, err := redis.GlobalRedisClient.GetAllCoins()
	if err != nil {
		logrus.Errorf("获取币种列表失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取币种列表失败",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data":  coins,
		"total": len(coins),
	})
}

// GetSelectedCoins 获取已筛选的币种
func (c *CoinController) GetSelectedCoins(ctx *gin.Context) {
	coins, err := redis.GlobalRedisClient.GetSelectedCoins()
	if err != nil {
		logrus.Errorf("获取筛选币种失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取筛选币种失败",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data":  coins,
		"total": len(coins),
	})
}

// SelectCoin 筛选币种
func (c *CoinController) SelectCoin(ctx *gin.Context) {
	var req struct {
		Symbol     string `json:"symbol" binding:"required"`
		IsSelected bool   `json:"is_selected"`
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "参数错误: " + err.Error(),
		})
		return
	}

	// 获取现有币种信息
	coin, err := redis.GlobalRedisClient.GetCoin(req.Symbol)
	if err != nil {
		// 如果币种不存在，创建新的
		coin = &models.Coin{
			Symbol:     req.Symbol,
			Status:     "active",
			IsSelected: req.IsSelected,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
	} else {
		// 更新筛选状态
		coin.IsSelected = req.IsSelected
		coin.UpdatedAt = time.Now()
	}

	err = redis.GlobalRedisClient.SetCoin(coin)
	if err != nil {
		logrus.Errorf("更新币种失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "更新币种失败",
		})
		return
	}

	// 如果是筛选币种，开始监听订单薄
	if req.IsSelected && exchanges.GlobalWebSocketManager != nil {
		err = exchanges.GlobalWebSocketManager.AddMarketDataSymbols([]string{req.Symbol})
		if err != nil {
			logrus.Errorf("订阅订单薄失败: %v", err)
		}
	} else if !req.IsSelected && exchanges.GlobalWebSocketManager != nil {
		// 如果取消筛选，停止监听
		err = exchanges.GlobalWebSocketManager.RemoveMarketDataSymbol(req.Symbol)
		if err != nil {
			return
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "币种状态更新成功",
		"data":    coin,
	})
}

// SyncCoins 从交易所同步币种列表
func (c *CoinController) SyncCoins(ctx *gin.Context) {
	if exchanges.GlobalBinanceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Binance客户端未初始化",
		})
		return
	}

	// 从Binance获取期货交易对
	coins, err := exchanges.GlobalBinanceClient.GetFuturesSymbols()
	if err != nil {
		logrus.Errorf("同步币种失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "同步币种失败: " + err.Error(),
		})
		return
	}

	// 保存到Redis
	savedCount := 0
	for _, coin := range coins {
		// 检查是否已存在
		existingCoin, err := redis.GlobalRedisClient.GetCoin(coin.Symbol)
		if err == nil {
			// 保持已有的筛选状态
			coin.IsSelected = existingCoin.IsSelected
		}

		err = redis.GlobalRedisClient.SetCoin(coin)
		if err != nil {
			logrus.Errorf("保存币种失败 %s: %v", coin.Symbol, err)
		} else {
			savedCount++
		}
	}

	logrus.Infof("同步完成，保存了 %d 个币种", savedCount)

	ctx.JSON(http.StatusOK, gin.H{
		"message": "币种同步完成",
		"total":   len(coins),
		"saved":   savedCount,
	})
}

// GetPrecision 获取币种精度信息
func (c *CoinController) GetPrecision(ctx *gin.Context) {
	symbol := ctx.Param("symbol")

	if exchanges.GlobalBinanceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Binance客户端未初始化",
		})
		return
	}

	// 从Binance获取精度信息
	coinInfo, err := exchanges.GlobalBinanceClient.GetSymbolPrecision(symbol)
	if err != nil {
		logrus.Errorf("获取币种精度信息失败 %s: %v", symbol, err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取精度信息失败: " + err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data": coinInfo,
	})
}
