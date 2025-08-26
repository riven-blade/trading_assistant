package core

import "trading_assistant/models"

// shouldTriggerLong 判断多头是否应该触发
func shouldTriggerLong(actionType, triggerType string, currentPrice, targetPrice float64) bool {
	// 立即执行的订单总是触发
	if triggerType == models.TriggerTypeImmediate {
		return true
	}

	// 条件触发的订单根据操作类型判断
	switch actionType {
	case models.ActionTypeOpen:
		// 开仓：当前价格 <= 目标价格时触发（低价买入）
		return currentPrice <= targetPrice
	case models.ActionTypeAddition:
		// 加仓：当前价格 <= 目标价格时触发（低价加仓）
		return currentPrice <= targetPrice
	case models.ActionTypeClose:
		// 平仓：当前价格 >= 目标价格时触发（高价卖出）
		return currentPrice >= targetPrice
	case models.ActionTypeTakeProfit:
		// 止盈：当前价格 >= 目标价格时触发（高价卖出获利）
		return currentPrice >= targetPrice
	case models.ActionTypeStopLoss:
		// 止损：当前价格 <= 目标价格时触发（低价卖出止损）
		return currentPrice <= targetPrice
	default:
		return false
	}
}

// shouldTriggerShort 判断空头是否应该触发
func shouldTriggerShort(actionType, triggerType string, currentPrice, targetPrice float64) bool {
	// 立即执行的订单总是触发
	if triggerType == models.TriggerTypeImmediate {
		return true
	}

	// 条件触发的订单根据操作类型判断
	switch actionType {
	case models.ActionTypeOpen:
		// 开仓：当前价格 >= 目标价格时触发（高价卖出）
		return currentPrice >= targetPrice
	case models.ActionTypeAddition:
		// 加仓：当前价格 >= 目标价格时触发（高价加仓）
		return currentPrice >= targetPrice
	case models.ActionTypeClose:
		// 平仓：当前价格 <= 目标价格时触发（低价买入）
		return currentPrice <= targetPrice
	case models.ActionTypeTakeProfit:
		// 止盈：当前价格 <= 目标价格时触发（低价买入获利）
		return currentPrice <= targetPrice
	case models.ActionTypeStopLoss:
		// 止损：当前价格 >= 目标价格时触发（高价买入止损）
		return currentPrice >= targetPrice
	default:
		return false
	}
}
