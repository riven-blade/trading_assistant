package controllers

import (
	"net/http"
	"trading_assistant/models"
	"trading_assistant/pkg/exchanges"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type KLineController struct{}

// GetKLines 获取K线数据
func (k *KLineController) GetKLines(ctx *gin.Context) {
	var req models.KLineRequest

	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	// 设置默认值
	if req.Interval == "" {
		req.Interval = "15m"
	}
	if req.Limit <= 0 || req.Limit > 1000 {
		req.Limit = 1000
	}

	if exchanges.GlobalBinanceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "交易所客户端未初始化"})
		return
	}

	// 直接从交易所获取K线数据
	klines, err := exchanges.GlobalBinanceClient.GetKLines(req.Symbol, req.Interval, req.Limit)
	if err != nil {
		logrus.Errorf("获取K线数据失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "获取K线数据失败"})
		return
	}

	k.buildAndSendResponse(ctx, req, klines)
}

// buildAndSendResponse 构建并发送响应
func (k *KLineController) buildAndSendResponse(ctx *gin.Context, req models.KLineRequest, klines []*models.KLineData) {
	var startTime, endTime int64
	if len(klines) > 0 {
		startTime = klines[0].Timestamp
		endTime = klines[len(klines)-1].Timestamp
	}

	response := &models.KLineResponse{
		Data: klines,
		Meta: &models.KLineMetadata{
			Symbol:    req.Symbol,
			Interval:  req.Interval,
			Count:     len(klines),
			StartTime: startTime,
			EndTime:   endTime,
		},
	}

	ctx.JSON(http.StatusOK, response)
}
