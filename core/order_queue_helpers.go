package core

import (
	"trading_assistant/models"
	"trading_assistant/pkg/exchanges/types"
)

// getQueueActionText 获取操作类型的中文描述（订单队列专用）
func getQueueActionText(actionType string) string {
	switch actionType {
	case models.ActionTypeOpen:
		return "开仓"
	case models.ActionTypeClose:
		return "平仓"
	case models.ActionTypeAddition:
		return "加仓"
	case models.ActionTypeTakeProfit:
		return "止盈"
	case models.ActionTypeStopLoss:
		return "止损"
	default:
		return "交易"
	}
}

// getQueuePositionText 获取仓位方向的中文描述（订单队列专用）
func getQueuePositionText(side string) string {
	switch side {
	case types.PositionSideLong:
		return "做多"
	case types.PositionSideShort:
		return "做空"
	default:
		return "未知"
	}
}
