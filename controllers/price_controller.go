package controllers

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"
	"trading_assistant/models"
	"trading_assistant/pkg/exchanges/types"
	"trading_assistant/pkg/redis"
	"trading_assistant/pkg/utils"

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
	Leverage    int     `json:"leverage"`     // 杠杆倍数
	OrderType   string  `json:"order_type"`   // 订单类型：market, limit
	MarginMode  string  `json:"margin_mode"`  // CROSS, ISOLATED (默认CROSS)
	TriggerType string  `json:"trigger_type"` // 触发类型
	Enabled     *bool   `json:"enabled"`      // 是否启用，使用指针以区分未传和false
}

// validatePriceEstimateRequest 验证价格预估请求
func (p *PriceController) validatePriceEstimateRequest(req *PriceEstimateRequest) error {
	// 验证交易方向
	if req.Side != types.PositionSideLong && req.Side != types.PositionSideShort {
		return fmt.Errorf("交易方向必须是 %s 或 %s", types.PositionSideLong, types.PositionSideShort)
	}

	// 验证操作类型
	validActionTypes := []string{
		models.ActionTypeOpen,
		models.ActionTypeAddition,
		models.ActionTypeClose,
		models.ActionTypeTakeProfit,
		models.ActionTypeStopLoss,
	}
	isValidActionType := false
	for i := range validActionTypes {
		validType := validActionTypes[i]
		if req.ActionType == validType {
			isValidActionType = true
			break
		}
	}
	if !isValidActionType {
		return fmt.Errorf("操作类型必须是: %v", validActionTypes)
	}

	// 设置默认值并验证保证金模式
	if req.MarginMode == "" {
		req.MarginMode = types.MarginModeCross // 默认全仓
	}
	if req.MarginMode != types.MarginModeCross && req.MarginMode != types.MarginModeIsolated {
		return fmt.Errorf("保证金模式必须是 %s 或 %s", types.MarginModeCross, types.MarginModeIsolated)
	}

	// 设置默认值并验证订单类型
	if req.OrderType == "" {
		req.OrderType = types.OrderTypeLimit // 默认限价单
	}
	if req.OrderType != types.OrderTypeMarket && req.OrderType != types.OrderTypeLimit {
		return fmt.Errorf("订单类型必须是 %s 或 %s", types.OrderTypeMarket, types.OrderTypeLimit)
	}

	// 设置默认值并验证触发类型
	if req.TriggerType == "" {
		req.TriggerType = models.TriggerTypeCondition // 默认条件触发
	}
	if req.TriggerType != models.TriggerTypeCondition && req.TriggerType != models.TriggerTypeImmediate {
		return fmt.Errorf("触发类型必须是 %s 或 %s", models.TriggerTypeCondition, models.TriggerTypeImmediate)
	}

	// 设置默认杠杆
	if req.Leverage <= 0 {
		req.Leverage = 10 // 默认10倍杠杆
	}

	return nil
}

// formatPriceEstimatePrecision 格式化价格预估的精度
func (p *PriceController) formatPriceEstimatePrecision(req *PriceEstimateRequest) error {
	// 获取币种信息
	coin, err := redis.GlobalRedisClient.GetCoin(req.Symbol)
	if err != nil {
		logrus.Warnf("获取币种信息失败，使用默认精度: %s, error: %v", req.Symbol, err)
		// 使用默认精度
		req.Quantity = parseFloat(fmt.Sprintf("%.6f", req.Quantity))
		req.TargetPrice = parseFloat(fmt.Sprintf("%.4f", req.TargetPrice))
		return nil
	}

	// 格式化数量精度
	quantityPrecision := coin.GetQuantityPrecisionFromStepSize()
	if quantityPrecision > 0 {
		quantityFormat := fmt.Sprintf("%%.%df", quantityPrecision)
		req.Quantity = parseFloat(fmt.Sprintf(quantityFormat, req.Quantity))

		// 验证最小数量
		if coin.MinQty != "" {
			minQty := parseFloat(coin.MinQty)
			if minQty > 0 && req.Quantity < minQty {
				return fmt.Errorf("交易数量 %.6f 小于最小数量 %.6f", req.Quantity, minQty)
			}
		}

		// 验证步长
		if coin.StepSize != "" {
			stepSize := parseFloat(coin.StepSize)
			if stepSize > 0 {
				// 使用数学上更精确的步长调整算法
				steps := req.Quantity / stepSize
				if math.Abs(steps-math.Round(steps)) > 1e-8 {
					// 向上舍入到最近的步长，确保数量不会变为0
					adjustedSteps := math.Ceil(steps)
					if adjustedSteps < 1 {
						adjustedSteps = 1
					}
					adjustedQuantity := adjustedSteps * stepSize

					// 确保调整后的数量仍满足最小数量要求
					minQty := parseFloat(coin.MinQty)
					if minQty > 0 && adjustedQuantity < minQty {
						// 如果调整后仍小于最小数量，计算需要的最小步数
						minSteps := math.Ceil(minQty / stepSize)
						adjustedQuantity = minSteps * stepSize
					}

					req.Quantity = parseFloat(fmt.Sprintf(quantityFormat, adjustedQuantity))

					logrus.WithFields(logrus.Fields{
						"symbol":            req.Symbol,
						"original_quantity": steps * stepSize,
						"adjusted_quantity": adjustedQuantity,
						"step_size":         stepSize,
						"steps":             adjustedSteps,
					}).Debug("数量步长调整")
				}
			}
		}
	}

	// 格式化价格精度
	pricePrecision := coin.GetPricePrecisionFromTickSize()
	if pricePrecision > 0 {
		priceFormat := fmt.Sprintf("%%.%df", pricePrecision)
		req.TargetPrice = parseFloat(fmt.Sprintf(priceFormat, req.TargetPrice))

		// 验证最小价格
		if coin.MinPrice != "" {
			minPrice := parseFloat(coin.MinPrice)
			if minPrice > 0 && req.TargetPrice < minPrice {
				return fmt.Errorf("目标价格 %.6f 小于最小价格 %.6f", req.TargetPrice, minPrice)
			}
		}

		// 验证价格步长
		if coin.TickSize != "" {
			tickSize := parseFloat(coin.TickSize)
			if tickSize > 0 {
				steps := req.TargetPrice / tickSize
				if steps != float64(int(steps)) {
					adjustedPrice := float64(int(steps)) * tickSize
					req.TargetPrice = parseFloat(fmt.Sprintf(priceFormat, adjustedPrice))
				}
			}
		}
	}

	logrus.WithFields(logrus.Fields{
		"symbol":       req.Symbol,
		"quantity":     req.Quantity,
		"target_price": req.TargetPrice,
		"min_quantity": coin.MinQty,
		"step_size":    coin.StepSize,
		"min_price":    coin.MinPrice,
		"tick_size":    coin.TickSize,
	}).Debug("精度格式化完成")

	return nil
}

// parseFloat 解析格式化后的浮点数
func parseFloat(s string) float64 {
	val, _ := strconv.ParseFloat(s, 64)
	return val
}

// createPriceEstimateModel 创建价格预估模型
func (p *PriceController) createPriceEstimateModel(req *PriceEstimateRequest) *models.PriceEstimate {
	// 确定是否启用：如果请求中提供了enabled值则使用，否则默认为true
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

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
		Enabled:     enabled,                        // 默认启用，可通过请求参数控制
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// CreatePriceEstimate 创建价格预估
func (p *PriceController) CreatePriceEstimate(ctx *gin.Context) {
	var req PriceEstimateRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		logrus.Warnf("价格预估参数错误: %v", err)
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数格式错误",
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

	// 格式化数量和价格精度
	if err := p.formatPriceEstimatePrecision(&req); err != nil {
		logrus.Errorf("格式化精度失败: %v", err)
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "格式化精度失败: " + err.Error(),
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

	// 通过WebSocket广播价格预估更新
	go utils.BroadcastSymbolEstimatesUpdate()

	ctx.JSON(http.StatusOK, gin.H{
		"message": "价格预估创建成功",
		"data":    estimate,
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

	// 通过WebSocket广播价格预估更新
	go utils.BroadcastSymbolEstimatesUpdate()

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
		logrus.Warnf("价格预估切换参数错误: %v", err)
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数格式错误",
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
	estimate, err := redis.GlobalRedisClient.GetEstimateById(id)
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

	// 通过WebSocket广播价格预估更新
	go utils.BroadcastSymbolEstimatesUpdate()

	ctx.JSON(http.StatusOK, gin.H{
		"message": "价格预估状态更新成功",
		"data":    estimate,
	})
}

// GetAllPriceEstimates 获取所有价格预估
func (p *PriceController) GetAllPriceEstimates(ctx *gin.Context) {
	symbol := ctx.Query("symbol")

	var estimates []*models.PriceEstimate
	var err error

	// 根据是否有symbol参数选择获取方法
	if symbol != "" {
		estimates, err = redis.GlobalRedisClient.GetAllEstimatesBySymbol(symbol)
	} else {
		estimates, err = redis.GlobalRedisClient.GetAllEstimates()
	}

	if err != nil {
		logrus.Errorf("获取价格预估失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取价格预估失败",
		})
		return
	}

	logrus.Debugf("获取到 %d 条价格预估数据 (symbol: %s)", len(estimates), symbol)

	ctx.JSON(http.StatusOK, gin.H{
		"data": estimates,
	})
}
