package controllers

import (
	"net/http"
	"strconv"
	"trading_assistant/pkg/exchanges/binance"

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

	logrus.Infof("请求K线数据: symbol=%s, interval=%s, limit=%d, since=%d", symbol, interval, limit, since)

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

	logrus.Infof("成功获取K线数据: %d条", len(klines))

	// 返回K线数据
	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    klines,
		"count":   len(klines),
		"params": gin.H{
			"symbol":   symbol,
			"interval": interval,
			"limit":    limit,
			"since":    since,
		},
	})
}
