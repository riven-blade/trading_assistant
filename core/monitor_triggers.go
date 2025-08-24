package core

import "trading_assistant/models"

// shouldTriggerLong 判断多头是否应该触发
func shouldTriggerLong(actionType, triggerType, createdBy string, currentPrice, targetPrice float64) bool {
	// 立即执行的订单总是触发
	if triggerType == models.TriggerTypeImmediate {
		return true
	}

	// 条件触发的订单根据操作类型判断
	switch actionType {
	case models.ActionTypeOpen:
		// 开仓（包含加仓）：当前价格 <= 目标价格时触发（低价买入）
		return currentPrice <= targetPrice
	case models.ActionTypeClose:
		// 平仓操作，根据createdBy区分具体类型
		switch createdBy {
		case models.CreatedByTakeProfit:
			// 止盈：当前价格 >= 目标价格时触发（高价卖出获利）
			return currentPrice >= targetPrice
		case models.CreatedByStopLoss:
			// 止损：当前价格 <= 目标价格时触发（低价卖出止损）
			return currentPrice <= targetPrice
		default:
			return false
		}
	default:
		return false
	}
}

// shouldTriggerShort 判断空头是否应该触发
func shouldTriggerShort(actionType, triggerType, createdBy string, currentPrice, targetPrice float64) bool {
	// 立即执行的订单总是触发
	if triggerType == models.TriggerTypeImmediate {
		return true
	}

	// 条件触发的订单根据操作类型判断
	switch actionType {
	case models.ActionTypeOpen:
		// 开仓（包含加仓）：当前价格 >= 目标价格时触发（高价卖出）
		return currentPrice >= targetPrice
	case models.ActionTypeClose:
		// 平仓操作，根据createdBy区分具体类型
		switch createdBy {
		case models.CreatedByTakeProfit:
			// 止盈：当前价格 <= 目标价格时触发（低价买入获利）
			return currentPrice <= targetPrice
		case models.CreatedByStopLoss:
			// 止损：当前价格 >= 目标价格时触发（高价买入止损）
			return currentPrice >= targetPrice
		default:
			return false
		}
	default:
		// 默认按开仓逻辑
		return currentPrice >= targetPrice
	}
}
