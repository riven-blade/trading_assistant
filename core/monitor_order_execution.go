package core

import (
	"fmt"
	"math"
	"trading_assistant/models"
	"trading_assistant/pkg/exchanges"
	"trading_assistant/pkg/redis"
	"trading_assistant/pkg/telegram"

	"github.com/sirupsen/logrus"
)

// executeOrder 执行订单
func executeOrder(estimate *models.PriceEstimate, currentPrice float64) error {
	if exchanges.GlobalBinanceClient == nil {
		return fmt.Errorf("Binance客户端未初始化")
	}

	// 在下单前设置该交易对的保证金模式
	err := exchanges.GlobalBinanceClient.SetSymbolMarginMode(estimate.Symbol, estimate.MarginMode)
	if err != nil {
		logrus.Warnf("设置 %s 保证金模式失败: %v", estimate.Symbol, err)
		// 继续执行，因为保证金模式可能已经设置过
	}

	// 使用标准化类型
	positionSide, err := exchanges.StringToPositionSide(estimate.Side)
	if err != nil {
		return fmt.Errorf("解析持仓方向失败: %v", err)
	}
	var actionType exchanges.ActionType // 稍后根据estimate.ActionType设置

	// 根据预估的订单类型确定实际订单类型
	var orderType exchanges.OrderType
	switch estimate.OrderType {
	case "market":
		orderType = exchanges.OrderTypeMarket
	default:
		// 所有其他情况（包括limit、止盈止损）都使用限价单
		orderType = exchanges.OrderTypeLimit
	}

	marginMode, err := exchanges.StringToMarginMode(estimate.MarginMode)
	if err != nil {
		return fmt.Errorf("解析保证金模式失败: %v", err)
	}

	var orderQuantity float64

	switch estimate.ActionType {
	case models.ActionTypeOpen:
		// 开仓（包含加仓）
		orderQuantity = estimate.Quantity
		actionType = exchanges.ActionTypeOpen
	case models.ActionTypeClose:
		// 平仓（包含减仓、止盈、止损）：检查持仓并确定数量
		position, err := redis.GlobalRedisClient.GetPosition(estimate.Symbol, string(positionSide))
		if err != nil || position == nil {
			return fmt.Errorf("没有找到 %s %s 持仓", estimate.Symbol, positionSide)
		}

		if position.Size == 0 {
			return fmt.Errorf("%s %s 持仓数量为0", estimate.Symbol, positionSide)
		}

		// 平仓数量不能超过实际持仓
		maxCloseQuantity := math.Abs(position.Size)
		if estimate.Quantity > maxCloseQuantity {
			orderQuantity = maxCloseQuantity
			logrus.Warnf("平仓数量 %.6f 超过实际持仓 %.6f，调整为实际持仓数量", estimate.Quantity, maxCloseQuantity)
		} else {
			orderQuantity = estimate.Quantity
		}
		actionType = exchanges.ActionTypeClose
	default:
		return fmt.Errorf("无效的操作类型: %s", estimate.ActionType)
	}

	// 根据持仓方向和操作类型确定订单方向（在actionType设置之后调用）
	orderSide, err := exchanges.GetOrderSide(positionSide, actionType)
	if err != nil {
		return fmt.Errorf("确定订单方向失败: %v", err)
	}

	// 使用市价单快速成交
	order, err := exchanges.GlobalBinanceClient.PlaceOrder(
		estimate.Symbol, orderSide, positionSide, orderQuantity, currentPrice, orderType, marginMode)
	if err != nil {
		return fmt.Errorf("下单失败: %v", err)
	}

	// 关联价格预估ID
	order.EstimateID = estimate.ID

	// 记录订单创建日志，订单状态通过WebSocket同步
	logrus.Infof("已创建订单: %s %s %.8g @ %.8g", order.Symbol, order.Side, order.Quantity, order.Price)

	// 发送下单确认
	if telegram.GlobalTelegramClient != nil {
		confirmMsg := fmt.Sprintf("ORDER %s %s | %.8g @ %.8g | PLACED",
			order.Symbol, order.Side, order.Quantity, order.Price)
		err = telegram.GlobalTelegramClient.SendMessage(confirmMsg)
		if err != nil {
			logrus.Errorf("发送下单确认失败: %v", err)
		}
	}

	logrus.Infof("订单执行成功: %s %s %s %f @ %f",
		string(actionType), estimate.Symbol, string(orderSide), orderQuantity, currentPrice)

	return nil
}
