package controllers

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
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
	MarginMode  string  `json:"margin_mode"` // CROSS, ISOLATED (默认ISOLATED)
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
		req.MarginMode = "ISOLATED" // 默认逐仓
	}
	if req.MarginMode != "CROSS" && req.MarginMode != "ISOLATED" {
		return fmt.Errorf("保证金模式必须是 CROSS 或 ISOLATED")
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

	// 格式化价格精度（使用从TickSize计算的精度）
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

// parseFloat 辅助函数，解析格式化后的浮点数
func parseFloat(s string) float64 {
	val, _ := strconv.ParseFloat(s, 64)
	return val
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

	ctx.JSON(http.StatusOK, gin.H{
		"message": "价格预估创建成功",
		"data":    estimate,
	})
}

// GetPriceEstimates 获取可用的价格预估列表
func (p *PriceController) GetPriceEstimates(ctx *gin.Context) {
	symbol := ctx.Query("symbol")

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
		estimates, err = redis.GlobalRedisClient.GetEstimatesBySymbol(symbol)
	} else {
		estimates, err = redis.GlobalRedisClient.GetEstimates()
	}

	if err != nil {
		logrus.Errorf("获取价格预估失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取价格预估失败",
		})
		return
	}
	
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

	ctx.JSON(http.StatusOK, gin.H{
		"message": "价格预估状态更新成功",
		"data":    estimate,
	})
}

// GetAllSelectedCoinsPrices 获取所有选中币的markPrice数据
func (p *PriceController) GetAllSelectedCoinsPrices(ctx *gin.Context) {
	// 获取所有选中的币种
	selectedCoins, err := redis.GlobalRedisClient.GetSelectedCoins()
	if err != nil {
		logrus.Errorf("获取选中币种失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取选中币种失败",
		})
		return
	}

	if len(selectedCoins) == 0 {
		ctx.JSON(http.StatusOK, gin.H{
			"data":  []interface{}{},
			"count": 0,
		})
		return
	}

	var priceDataList []models.CoinPriceData
	var successCount, errorCount int

	for i := range selectedCoins {
		coin := selectedCoins[i]
		// 从Redis获取markPrice数据
		markPrice, err := redis.GlobalRedisClient.GetMarkPrice(coin.Symbol)
		if err != nil {
			logrus.Debugf("获取 %s markPrice失败: %v", coin.Symbol, err)
			errorCount++
			continue
		}

		// 构造返回数据
		priceData := models.CoinPriceData{
			Symbol:       markPrice.Symbol,
			MarkPrice:    markPrice.MarkPrice,
			IndexPrice:   markPrice.IndexPrice,
			FundingRate:  markPrice.FundingRate,
			FundingTime:  markPrice.FundingTime,
			UpdateTime:   markPrice.TimeStamp,
			PriceChange:  coin.PriceChange,        // 从币种信息获取
			PricePercent: coin.PriceChangePercent, // 从币种信息获取
		}

		priceDataList = append(priceDataList, priceData)
		successCount++
	}

	logrus.WithFields(logrus.Fields{
		"total_coins":    len(selectedCoins),
		"success_count":  successCount,
		"error_count":    errorCount,
		"returned_count": len(priceDataList),
	}).Debug("批量获取选中币种价格数据完成")

	ctx.JSON(http.StatusOK, gin.H{
		"data":  priceDataList,
		"count": len(priceDataList),
		"stats": gin.H{
			"total_coins":   len(selectedCoins),
			"success_count": successCount,
			"error_count":   errorCount,
		},
	})
}
