package utils

import (
	"math"
	"strconv"
	"trading_assistant/pkg/redis"

	"github.com/sirupsen/logrus"
)

// AdjustQuantityPrecision 调整数量精度的通用函数
func AdjustQuantityPrecision(symbol string, quantity float64) (float64, error) {
	coin, err := redis.GlobalRedisClient.GetCoin(symbol)
	if err != nil {
		// 使用默认精度
		logrus.WithFields(logrus.Fields{
			"symbol": symbol,
			"error":  err.Error(),
		}).Warn("获取币种精度信息失败，使用默认精度6位")
		return RoundToDecimalPlaces(quantity, 6), nil
	}

	// 首先调整小数位精度
	quantityPrecision := coin.GetQuantityPrecisionFromStepSize()
	adjustedQuantity := RoundToDecimalPlaces(quantity, quantityPrecision)

	// 然后验证和调整步长约束
	if coin.StepSize != "" {
		stepSize := ParseFloat(coin.StepSize)
		if stepSize > 0 {
			// 使用数学上更精确的步长调整算法
			steps := adjustedQuantity / stepSize
			if math.Abs(steps-math.Round(steps)) > 1e-8 { // 使用容差避免浮点数精度问题
				// 向上舍入到最近的步长，确保数量不会变为0
				adjustedSteps := math.Ceil(steps)
				if adjustedSteps < 1 {
					adjustedSteps = 1
				}
				adjustedQuantity = adjustedSteps * stepSize

				// 确保调整后的数量仍满足最小数量要求
				minQty := ParseFloat(coin.MinQty)
				if minQty > 0 && adjustedQuantity < minQty {
					// 如果调整后仍小于最小数量，计算需要的最小步数
					minSteps := math.Ceil(minQty / stepSize)
					adjustedQuantity = minSteps * stepSize
				}

				// 重新应用小数位精度
				adjustedQuantity = RoundToDecimalPlaces(adjustedQuantity, quantityPrecision)

				logrus.WithFields(logrus.Fields{
					"symbol":            symbol,
					"original_quantity": quantity,
					"adjusted_quantity": adjustedQuantity,
					"step_size":         stepSize,
					"steps":             adjustedSteps,
				}).Debug("数量步长调整")
			}
		}
	}

	// 最终验证：确保调整后的数量不为0
	if adjustedQuantity <= 0 {
		// 如果数量仍然为0，使用最小有效数量
		minQty := ParseFloat(coin.MinQty)
		stepSize := ParseFloat(coin.StepSize)

		if minQty > 0 {
			adjustedQuantity = minQty
		} else if stepSize > 0 {
			adjustedQuantity = stepSize
		} else {
			adjustedQuantity = math.Pow(10, -float64(quantityPrecision))
		}

		logrus.WithFields(logrus.Fields{
			"symbol":   symbol,
			"original": quantity,
			"adjusted": adjustedQuantity,
			"reason":   "避免数量为0",
		}).Warn("数量调整后为0，使用最小有效数量")
	}

	logrus.WithFields(logrus.Fields{
		"symbol":    symbol,
		"original":  quantity,
		"precision": quantityPrecision,
		"adjusted":  adjustedQuantity,
		"min_qty":   coin.MinQty,
		"step_size": coin.StepSize,
	}).Debug("数量精度调整完成")

	return adjustedQuantity, nil
}

// RoundToDecimalPlaces 四舍五入到指定小数位
func RoundToDecimalPlaces(value float64, places int) float64 {
	multiplier := math.Pow(10, float64(places))
	return math.Round(value*multiplier) / multiplier
}

// ParseFloat 辅助函数，安全地解析浮点数
func ParseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return val
}
