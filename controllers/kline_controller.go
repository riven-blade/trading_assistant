package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"trading_assistant/pkg/exchanges/binance"
	"trading_assistant/pkg/exchanges/types"
	"trading_assistant/pkg/redis"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type KlineController struct {
	binanceClient *binance.Binance
}

// NewKlineController 创建K线控制器
func NewKlineController(binanceClient *binance.Binance) *KlineController {
	return &KlineController{
		binanceClient: binanceClient,
	}
}

// GetKlines 获取K线数据
func (k *KlineController) GetKlines(ctx *gin.Context) {
	if k.binanceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Binance客户端未初始化",
		})
		return
	}

	// 获取参数
	symbol := ctx.Query("symbol")
	if symbol == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "symbol参数不能为空",
		})
		return
	}

	interval := ctx.DefaultQuery("interval", "5m")
	limitStr := ctx.DefaultQuery("limit", "1000")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "limit参数格式错误",
		})
		return
	}

	// 获取可选参数
	var since int64
	if sinceStr := ctx.Query("since"); sinceStr != "" {
		if parsed, err := strconv.ParseInt(sinceStr, 10, 64); err == nil {
			since = parsed
		}
	}

	// 构建缓存键
	cacheKey := fmt.Sprintf("%s:%s:%s:%d:%d", redis.CacheKeyKLines, symbol, interval, limit, since)

	// 检查Redis缓存
	var cachedKlines []*types.Kline
	if redis.GlobalRedisClient != nil {
		if err := redis.GlobalRedisClient.GetCache(cacheKey, &cachedKlines); err == nil {
			logrus.Debugf("从缓存获取K线数据: %s", cacheKey)
			ctx.JSON(http.StatusOK, gin.H{
				"success": true,
				"data":    cachedKlines,
				"count":   len(cachedKlines),
				"cached":  true,
				"source":  "cache",
				"params": gin.H{
					"symbol":   symbol,
					"interval": interval,
					"limit":    limit,
					"since":    since,
				},
			})
			return
		}
	}

	logrus.Infof("缓存中无K线数据，实时获取: symbol=%s, interval=%s, limit=%d, since=%d", symbol, interval, limit, since)

	// 从Binance获取K线数据
	klines, err := k.binanceClient.FetchKlines(ctx.Request.Context(), symbol, interval, since, limit, nil)
	if err != nil {
		logrus.Errorf("获取K线数据失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error":   "获取K线数据失败",
			"details": err.Error(),
		})
		return
	}

	// 缓存K线数据
	if redis.GlobalRedisClient != nil {
		if err := redis.GlobalRedisClient.SetCache(cacheKey, klines); err != nil {
			logrus.Errorf("缓存K线数据失败: %v", err)
		} else {
			logrus.Debugf("已缓存K线数据5分钟: %s", cacheKey)
		}
	}

	logrus.Infof("成功获取K线数据: %d条", len(klines))

	// 返回K线数据
	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    klines,
		"count":   len(klines),
		"cached":  false,
		"source":  "real_time",
		"params": gin.H{
			"symbol":   symbol,
			"interval": interval,
			"limit":    limit,
			"since":    since,
		},
	})
}
