package controllers

import (
	"fmt"
	"net/http"
	"time"
	"trading_assistant/models"
	"trading_assistant/pkg/redis"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type PriceController struct{}

// PriceEstimateRequest 价格预估请求结构
type PriceEstimateRequest struct {
	Symbol      string  `json:"symbol" binding:"required"`
	Side        string  `json:"side" binding:"required"`        // long, short
	ActionType  string  `json:"action_type" binding:"required"` // open, close
	TargetPrice float64 `json:"target_price" binding:"required"`
	Quantity    float64 `json:"quantity" binding:"required"`
	Leverage    int     `json:"leverage"`    // 杠杆倍数
	OrderType   string  `json:"order_type"`  // 订单类型：market, limit
	MarginMode  string  `json:"margin_mode"` // cross, isolated (默认cross)
	CreatedBy   string  `json:"created_by"`
	TriggerType string  `json:"trigger_type"` // 触发类型
}

// validatePriceEstimateRequest 验证价格预估请求
func (p *PriceController) validatePriceEstimateRequest(req *PriceEstimateRequest) error {
	// 验证交易方向
	if req.Side != "long" && req.Side != "short" {
		return fmt.Errorf("交易方向必须是 long 或 short")
	}

	// 验证操作类型
	validActionTypes := []string{"open", "add", "take_profit", "stop_loss", "close"}
	isValidActionType := false
	for _, validType := range validActionTypes {
		if req.ActionType == validType {
			isValidActionType = true
			break
		}
	}
	if !isValidActionType {
		return fmt.Errorf("操作类型必须是 open, add, take_profit, stop_loss 或 close")
	}

	// 设置默认值并验证保证金模式
	if req.MarginMode == "" {
		req.MarginMode = "isolated" // 默认逐仓
	}
	if req.MarginMode != "cross" && req.MarginMode != "isolated" {
		return fmt.Errorf("保证金模式必须是 cross 或 isolated")
	}

	// 设置默认值并验证订单类型
	if req.OrderType == "" {
		req.OrderType = "limit" // 默认限价单
	}
	if req.OrderType != "market" && req.OrderType != "limit" {
		return fmt.Errorf("订单类型必须是 market 或 limit")
	}

	// 设置默认值并验证触发类型
	if req.TriggerType == "" {
		req.TriggerType = "condition" // 默认条件触发
	}
	if req.TriggerType != "condition" && req.TriggerType != "time" {
		return fmt.Errorf("触发类型必须是 condition 或 time")
	}

	// 设置默认杠杆
	if req.Leverage <= 0 {
		req.Leverage = 5 // 默认5倍杠杆
	}

	return nil
}

// createPriceEstimateModel 创建价格预估模型
func (p *PriceController) createPriceEstimateModel(req *PriceEstimateRequest) *models.PriceEstimate {
	// 初始状态为待激活，等待用户手动启用
	return &models.PriceEstimate{
		ID:          uuid.New().String(),
		Symbol:      req.Symbol,
		Side:        req.Side,
		ActionType:  req.ActionType,
		TargetPrice: req.TargetPrice,
		Quantity:    req.Quantity,
		Leverage:    req.Leverage,
		OrderType:   req.OrderType,
		MarginMode:  req.MarginMode,
		TriggerType: req.TriggerType,
		Status:      models.EstimateStatusListening, // 初始状态为监听状态
		Enabled:     false,                          // 默认未启用，需要用户手动启用
		CreatedBy:   req.CreatedBy,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// CreatePriceEstimate 创建价格预估
func (p *PriceController) CreatePriceEstimate(ctx *gin.Context) {
	var req PriceEstimateRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "参数错误: " + err.Error(),
		})
		return
	}

	// 验证请求参数
	if err := p.validatePriceEstimateRequest(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// 创建价格预估模型
	estimate := p.createPriceEstimateModel(&req)

	// 保存到Redis
	if redis.GlobalRedisClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Redis服务不可用",
		})
		return
	}

	if err := redis.GlobalRedisClient.SetPriceEstimate(estimate); err != nil {
		logrus.Errorf("保存价格预估失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "保存价格预估失败",
		})
		return
	}

	logrus.Infof("创建价格预估成功: %s %s %s %.4f",
		estimate.Symbol, estimate.Side, estimate.ActionType, estimate.TargetPrice)

	ctx.JSON(http.StatusOK, gin.H{
		"message": "价格预估创建成功",
		"data":    estimate,
	})
}

// GetPriceEstimates 获取可用的价格预估列表
func (p *PriceController) GetPriceEstimates(ctx *gin.Context) {
	symbol := ctx.Query("symbol")
	status := ctx.Query("status")

	logrus.Infof("查询价格预估: symbol=%s, status=%s", symbol, status)

	if redis.GlobalRedisClient == nil {
		logrus.Error("Redis客户端未初始化")
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Redis服务不可用",
		})
		return
	}

	var estimates []*models.PriceEstimate
	var err error

	if symbol != "" {
		logrus.Infof("按交易对查询: %s", symbol)
		estimates, err = redis.GlobalRedisClient.GetEstimatesBySymbol(symbol)
		logrus.Infof("按交易对查询结果: 找到 %d 条记录", len(estimates))
	} else {
		estimates, err = redis.GlobalRedisClient.GetEstimates()
		logrus.Infof("查询所有pending结果: 找到 %d 条记录", len(estimates))
	}

	if err != nil {
		logrus.Errorf("获取价格预估失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取价格预估失败",
		})
		return
	}

	logrus.Infof("最终返回 %d 条价格预估记录", len(estimates))
	ctx.JSON(http.StatusOK, gin.H{
		"data":  estimates,
		"total": len(estimates),
	})
}

// DeletePriceEstimate 删除价格预估
func (p *PriceController) DeletePriceEstimate(ctx *gin.Context) {
	id := ctx.Param("id")

	if redis.GlobalRedisClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Redis服务不可用",
		})
		return
	}

	// 直接删除预估记录
	err := redis.GlobalRedisClient.DeletePriceEstimate(id)
	if err != nil {
		logrus.Errorf("删除价格预估失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "删除价格预估失败",
		})
		return
	}

	logrus.Infof("删除价格预估成功: %s", id)

	ctx.JSON(http.StatusOK, gin.H{
		"message": "价格预估删除成功",
	})
}

// TogglePriceEstimate 切换价格预估监听状态
func (p *PriceController) TogglePriceEstimate(ctx *gin.Context) {
	id := ctx.Param("id")

	var req struct {
		Enabled bool `json:"enabled"`
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "参数错误: " + err.Error(),
		})
		return
	}

	if redis.GlobalRedisClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Redis服务不可用",
		})
		return
	}

	// 获取价格预估
	estimate, err := redis.GlobalRedisClient.GetPriceEstimate(id)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{
			"error": "价格预估不存在",
		})
		return
	}

	estimate.Enabled = req.Enabled
	estimate.UpdatedAt = time.Now()

	if err := redis.GlobalRedisClient.SetPriceEstimate(estimate); err != nil {
		logrus.Errorf("更新价格预估状态失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "更新价格预估状态失败",
		})
		return
	}

	statusText := "暂停"
	if req.Enabled {
		statusText = "激活"
	}

	logrus.Infof("价格预估状态已更新: %s -> %s", id, statusText)

	ctx.JSON(http.StatusOK, gin.H{
		"message": "价格预估状态更新成功",
		"data":    estimate,
	})
}
