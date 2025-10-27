package core

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"trading_assistant/models"
	"trading_assistant/pkg/config"
	"trading_assistant/pkg/exchanges/binance"
	"trading_assistant/pkg/exchanges/types"
	"trading_assistant/pkg/redis"
	"trading_assistant/pkg/telegram"
	"trading_assistant/pkg/utils"

	"github.com/sirupsen/logrus"
)

// OrderExecutor 订单执行器
type OrderExecutor struct {
	binanceClient *binance.Binance
	ctx           context.Context
}

// NewOrderExecutor 创建订单执行器
func NewOrderExecutor(binanceClient *binance.Binance) *OrderExecutor {
	return &OrderExecutor{
		binanceClient: binanceClient,
		ctx:           context.Background(),
	}
}

// ExecuteOrder 执行订单
func (oe *OrderExecutor) ExecuteOrder(estimate *models.PriceEstimate, currentPrice float64) error {
	if oe.binanceClient == nil {
		return fmt.Errorf("binance客户端未初始化")
	}

	logrus.WithFields(logrus.Fields{
		"symbol":        estimate.Symbol,
		"action_type":   estimate.ActionType,
		"side":          estimate.Side,
		"quantity":      estimate.Quantity,
		"target_price":  estimate.TargetPrice,
		"current_price": currentPrice,
	}).Info("开始执行双向持仓订单")

	// 仓位数量检查
	if estimate.ActionType == models.ActionTypeOpen {
		if err := oe.checkPositionCount(estimate); err != nil {
			return err
		}
	}

	// 验证和准备订单参数
	orderParams, err := oe.prepareOrderParams(estimate, currentPrice)
	if err != nil {
		return fmt.Errorf("准备订单参数失败: %v", err)
	}

	// 设置杠杆和保证金模式
	if err := oe.setupTradingEnvironment(estimate); err != nil {
		return fmt.Errorf("设置交易环境失败: %v", err)
	}

	// 最终验证数量
	if orderParams.Quantity <= 0 {
		return fmt.Errorf("下单数量不能为0或负数: %f", orderParams.Quantity)
	}

	// 验证持仓
	if orderParams.ReduceOnly {
		if err := oe.validatePositionForReduce(estimate.Symbol, orderParams.PositionSide, orderParams.Quantity); err != nil {
			return fmt.Errorf("持仓验证失败: %v", err)
		}
	}

	// 执行下单
	order, err := oe.placeOrder(orderParams)
	if err != nil {
		return fmt.Errorf("下单失败: %v", err)
	}

	// 更新预估状态
	if err := oe.updateEstimateStatus(estimate, models.EstimateStatusTriggered); err != nil {
		logrus.Errorf("更新预估状态失败: %v", err)
	}

	logrus.WithFields(logrus.Fields{
		"order_id":     order.ExchangeID,
		"quantity":     order.Quantity,
		"executed_qty": order.ExecutedQty,
		"price":        order.Price,
	}).Info("双向持仓订单执行成功")

	return nil
}

// OrderParams 订单参数
type OrderParams struct {
	Symbol       string  // 交易对
	Side         string  // 订单方向: BUY/SELL
	PositionSide string  // 仓位方向: LONG/SHORT/BOTH
	Type         string  // 订单类型: MARKET/LIMIT
	Quantity     float64 // 数量
	Price        float64 // 价格 (限价单使用)
	TimeInForce  string  // 有效期类型
	ReduceOnly   bool    // 是否只减仓
}

// prepareOrderParams 准备订单参数
func (oe *OrderExecutor) prepareOrderParams(estimate *models.PriceEstimate, currentPrice float64) (*OrderParams, error) {
	// 1. 使用已格式化的交易数量，再次确保精度
	quantity, err := utils.AdjustQuantityPrecision(estimate.Symbol, estimate.Quantity)
	if err != nil {
		return nil, fmt.Errorf("调整数量精度失败: %v", err)
	}

	// 2. 确定订单方向和仓位方向
	orderSide, positionSide, reduceOnly := oe.getDualPositionParams(estimate.ActionType, estimate.Side)

	// 3. 确定订单类型和价格
	orderType, orderPrice := oe.getOrderTypeAndPrice(estimate.OrderType, estimate.TargetPrice, currentPrice)

	return &OrderParams{
		Symbol:       estimate.Symbol,
		Side:         orderSide,
		PositionSide: positionSide,
		Type:         orderType,
		Quantity:     quantity,
		Price:        orderPrice,
		TimeInForce:  "GTC", // Good Till Canceled
		ReduceOnly:   reduceOnly,
	}, nil
}

// getDualPositionParams 获取双向持仓参数
func (oe *OrderExecutor) getDualPositionParams(actionType, side string) (orderSide, positionSide string, reduceOnly bool) {
	switch actionType {
	case models.ActionTypeOpen: // 开仓
		reduceOnly = false
		if side == types.PositionSideLong {
			return types.OrderSideBuy, "LONG", reduceOnly // 开多仓：买入+多头方向
		} else {
			return types.OrderSideSell, "SHORT", reduceOnly // 开空仓：卖出+空头方向
		}
	case models.ActionTypeClose: // 平仓
		reduceOnly = true // 平仓时只减仓
		if side == types.PositionSideLong {
			return types.OrderSideSell, "LONG", reduceOnly // 平多仓：卖出+多头方向
		} else {
			return types.OrderSideBuy, "SHORT", reduceOnly // 平空仓：买入+空头方向
		}
	case models.ActionTypeTakeProfit: // 止盈
		reduceOnly = true // 止盈时只减仓
		if side == types.PositionSideLong {
			return types.OrderSideSell, "LONG", reduceOnly
		} else {
			return types.OrderSideBuy, "SHORT", reduceOnly
		}
	case models.ActionTypeStopLoss: //
		reduceOnly = true // 止损时只减仓
		if side == types.PositionSideLong {
			return types.OrderSideSell, "LONG", reduceOnly // 多头止损：卖出+多头方向
		} else {
			return types.OrderSideBuy, "SHORT", reduceOnly // 空头止损：买入+空头方向
		}
	case models.ActionTypeAddition: // 加仓
		reduceOnly = false
		if side == types.PositionSideLong {
			return types.OrderSideBuy, "LONG", reduceOnly // 加多仓：买入+多头方向
		} else {
			return types.OrderSideSell, "SHORT", reduceOnly // 加空仓：卖出+空头方向
		}
	default:
		// 不应该到达这里，记录错误并返回安全的默认值
		logrus.Errorf("未知的操作类型: %s, 使用默认开多仓", actionType)
		return types.OrderSideBuy, "LONG", false
	}
}

// getOrderTypeAndPrice 获取订单类型和价格
func (oe *OrderExecutor) getOrderTypeAndPrice(orderType string, targetPrice, currentPrice float64) (string, float64) {
	switch orderType {
	case types.OrderTypeMarket:
		return types.OrderTypeMarket, 0 // 市价单不需要价格
	case types.OrderTypeLimit:
		return types.OrderTypeLimit, targetPrice
	default:
		return types.OrderTypeMarket, 0
	}
}

// setupTradingEnvironment 设置交易环境（杠杆、保证金模式）
func (oe *OrderExecutor) setupTradingEnvironment(estimate *models.PriceEstimate) error {
	// 设置杠杆
	if estimate.Leverage > 0 {
		if err := oe.binanceClient.SetLeverage(estimate.Symbol, estimate.Leverage); err != nil {
			logrus.Warnf("设置杠杆失败 %s %dx: %v", estimate.Symbol, estimate.Leverage, err)
			// 不返回错误，因为杠杆可能已经设置过
		} else {
			logrus.Infof("杠杆设置成功 %s %dx", estimate.Symbol, estimate.Leverage)
		}
	}

	// 设置保证金模式
	if estimate.MarginMode != "" {
		marginType := types.MarginModeCrossed // 默认全仓
		if estimate.MarginMode == types.MarginModeIsolated {
			marginType = types.MarginModeIsolated
		}

		if err := oe.binanceClient.SetMarginType(estimate.Symbol, marginType); err != nil {
			logrus.Warnf("设置保证金模式失败 %s %s: %v", estimate.Symbol, marginType, err)
			// 不返回错误，因为保证金模式可能已经设置过
		} else {
			logrus.Infof("保证金模式设置成功 %s %s", estimate.Symbol, marginType)
		}
	}

	return nil
}

// placeOrder 执行下单
func (oe *OrderExecutor) placeOrder(params *OrderParams) (*models.Order, error) {
	// 构建Binance期货下单参数
	orderParams := map[string]interface{}{
		"symbol":       params.Symbol,
		"side":         params.Side,
		"positionSide": params.PositionSide,
		"type":         params.Type,
		"quantity":     oe.formatQuantityString(params.Quantity),
	}

	// 如果是限价单，添加价格
	if params.Type == types.OrderTypeLimit {
		orderParams["price"] = oe.formatPriceString(params.Price)
		orderParams["timeInForce"] = params.TimeInForce
	}

	// 在双向持仓模式下，不使用reduceOnly参数
	if params.ReduceOnly && params.PositionSide == "BOTH" {
		orderParams["reduceOnly"] = "true"
		logrus.WithFields(logrus.Fields{
			"symbol":        params.Symbol,
			"position_side": params.PositionSide,
			"side":          params.Side,
		}).Info("设置reduceOnly=true进行单向持仓平仓操作")
	} else if params.ReduceOnly {
		logrus.WithFields(logrus.Fields{
			"symbol":        params.Symbol,
			"position_side": params.PositionSide,
			"side":          params.Side,
		}).Info("双向持仓模式下跳过reduceOnly参数，使用positionSide确定操作类型")
	}

	logrus.WithFields(logrus.Fields{
		"symbol":        params.Symbol,
		"side":          params.Side,
		"position_side": params.PositionSide,
		"type":          params.Type,
		"quantity":      params.Quantity,
		"price":         params.Price,
		"reduce_only":   params.ReduceOnly,
	}).Info("执行双向持仓下单")

	// 调用Binance期货下单API
	logrus.WithFields(logrus.Fields{
		"request_params": orderParams,
	}).Info("发送Binance期货下单请求")

	response, err := oe.binanceClient.FuturesNewOrder(orderParams)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error":          err.Error(),
			"request_params": orderParams,
		}).Error("Binance期货下单失败")
		return nil, fmt.Errorf("binance期货下单失败: %v", err)
	}

	// 打印响应用于调试
	logrus.WithFields(logrus.Fields{
		"response": response,
	}).Debug("Binance下单响应")

	// 转换响应为内部Order模型
	order := oe.convertToOrder(response, params)

	logrus.WithFields(logrus.Fields{
		"binance_order_id": response.OrderID,
		"client_order_id":  response.ClientOrderID,
		"status":           response.Status,
		"executed_qty":     response.ExecutedQty,
		"avg_price":        response.AvgPrice,
		"symbol":           response.Symbol,
		"side":             response.Side,
		"position_side":    response.PositionSide,
		"orig_qty":         response.OrigQty,
	}).Info("双向持仓订单提交成功")

	return order, nil
}

// convertToOrder 转换Binance响应为内部Order模型
func (oe *OrderExecutor) convertToOrder(response *binance.FuturesOrderResponse, params *OrderParams) *models.Order {
	executedQty, _ := strconv.ParseFloat(response.ExecutedQty, 64)
	avgPrice, _ := strconv.ParseFloat(response.AvgPrice, 64)
	if avgPrice == 0 {
		// 如果没有成交价格，使用订单价格
		avgPrice = params.Price
	}

	// 使用真正的OrderID作为ExchangeID
	exchangeID := fmt.Sprintf("%d", response.OrderID)
	if exchangeID == "0" && response.ClientOrderID != "" {
		// 如果OrderID为0，则回退到ClientOrderID
		exchangeID = response.ClientOrderID
	}

	return &models.Order{
		ID:           response.ClientOrderID,
		Symbol:       response.Symbol,
		Side:         response.Side,
		PositionSide: response.PositionSide,
		Type:         response.Type,
		Quantity:     params.Quantity,
		ExecutedQty:  executedQty,
		Price:        avgPrice,
		Status:       response.Status,
		ExchangeID:   exchangeID,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// updateEstimateStatus 更新预估状态
func (oe *OrderExecutor) updateEstimateStatus(estimate *models.PriceEstimate, status string) error {
	// 更新Redis中的预估状态为已触发
	logrus.WithFields(logrus.Fields{
		"estimate_id": estimate.ID,
		"old_status":  estimate.Status,
		"new_status":  status,
	}).Debug("更新预估状态")

	// 更新预估状态
	estimate.Status = status
	estimate.UpdatedAt = time.Now()

	// 保存到Redis
	err := redis.GlobalRedisClient.SetPriceEstimate(estimate)
	if err != nil {
		logrus.Errorf("更新预估状态失败: %v", err)
		return err
	}

	// 通过WebSocket广播价格预估更新
	go utils.BroadcastSymbolEstimatesUpdate()

	return nil
}

// sendOrderNotification 发送订单通知
func (oe *OrderExecutor) sendOrderNotification(estimate *models.PriceEstimate, order *models.Order, params *OrderParams) {
	if telegram.GlobalTelegramClient == nil {
		return
	}

	actionText := "开仓"
	switch estimate.ActionType {
	case models.ActionTypeClose:
		actionText = "平仓"
	case models.ActionTypeTakeProfit:
		actionText = "止盈"
	case models.ActionTypeStopLoss:
		actionText = "止损"
	case models.ActionTypeAddition:
		actionText = "加仓"
	default:
		actionText = "开仓"
	}

	positionText := "多头"
	if estimate.Side == types.PositionSideShort {
		positionText = "空头"
	}

	message := fmt.Sprintf("%s %s %s %.6f @ %.4f",
		estimate.Symbol,
		actionText,
		positionText,
		params.Quantity,
		order.Price)

	if err := telegram.GlobalTelegramClient.SendMessage(message); err != nil {
		logrus.Errorf("发送订单通知失败: %v", err)
	}
}

// formatQuantityString 格式化数量为Binance API要求的字符串格式
func (oe *OrderExecutor) formatQuantityString(quantity float64) string {
	// 去除尾随零，确保格式正确
	str := fmt.Sprintf("%.8f", quantity)
	str = strings.TrimRight(str, "0")
	str = strings.TrimRight(str, ".")
	return str
}

// formatPriceString 格式化价格为Binance API要求的字符串格式
func (oe *OrderExecutor) formatPriceString(price float64) string {
	// 去除尾随零，确保格式正确
	str := fmt.Sprintf("%.8f", price)
	str = strings.TrimRight(str, "0")
	str = strings.TrimRight(str, ".")
	return str
}

// validatePositionForReduce 验证减仓操作的持仓是否足够
func (oe *OrderExecutor) validatePositionForReduce(symbol, positionSide string, quantity float64) error {
	if redis.GlobalRedisClient == nil {
		return fmt.Errorf("Redis客户端未初始化")
	}

	// 记录调试信息
	logrus.WithFields(logrus.Fields{
		"symbol":        symbol,
		"position_side": positionSide,
		"quantity":      quantity,
	}).Info("开始验证减仓持仓")

	// 确保使用大写格式查询
	upperPositionSide := strings.ToUpper(positionSide)
	position, err := redis.GlobalRedisClient.GetPosition(symbol, upperPositionSide)
	if err != nil {
		return fmt.Errorf("获取持仓信息失败: %v", err)
	}

	// 检查持仓是否存在
	if position == nil {
		return fmt.Errorf("没有找到 %s %s 方向的持仓记录", symbol, upperPositionSide)
	}

	// 检查持仓数量是否足够
	if position.Size == 0 {
		return fmt.Errorf("持仓数量为0，无法减仓 %s %s", symbol, upperPositionSide)
	}

	if math.Abs(position.Size) < quantity {
		return fmt.Errorf("持仓数量不足: 当前持仓 %.6f，尝试减仓 %.6f",
			math.Abs(position.Size), quantity)
	}

	logrus.WithFields(logrus.Fields{
		"symbol":          symbol,
		"position_side":   upperPositionSide,
		"current_size":    position.Size,
		"reduce_quantity": quantity,
	}).Info("持仓验证通过")

	return nil
}

// checkPositionCount 检查持仓数量是否超过限制
func (oe *OrderExecutor) checkPositionCount(estimate *models.PriceEstimate) error {
	// 获取最大持仓数量限制
	maxPositionCount := config.GlobalConfig.MaxPositionCount
	if maxPositionCount <= 0 {
		logrus.Warn("最大持仓数量未配置或配置错误，跳过仓位数量检查")
		return nil
	}

	// 获取当前所有持仓
	positions, err := oe.binanceClient.FetchPositions(oe.ctx, nil, nil)
	if err != nil {
		logrus.Warnf("获取持仓信息失败: %v，跳过仓位数量检查", err)
		return nil
	}

	// 统计当前有持仓的币对数量
	currentPositionCount := len(positions)

	// 检查是否超过最大持仓数量
	if currentPositionCount >= maxPositionCount {
		positionText := "多头"
		if estimate.Side == types.PositionSideShort {
			positionText = "空头"
		}

		// 发送Telegram通知
		if telegram.GlobalTelegramClient != nil {
			message := fmt.Sprintf("持仓数量已达上限，开仓操作取消\n"+
				"币对: %s\n"+
				"方向: 开仓 %s\n"+
				"原因: 当前持仓数量 %d 已达到上限 %d",
				estimate.Symbol, positionText,
				currentPositionCount, maxPositionCount)

			if err := telegram.GlobalTelegramClient.SendMessage(message); err != nil {
				logrus.Errorf("发送持仓数量限制通知失败: %v", err)
			}
		}

		logrus.WithFields(logrus.Fields{
			"symbol":                 estimate.Symbol,
			"side":                   estimate.Side,
			"current_position_count": currentPositionCount,
			"max_position_count":     maxPositionCount,
		}).Warn("持仓数量已达上限，取消开仓")

		return fmt.Errorf("当前持仓数量 %d 已达到上限 %d，开仓操作已取消",
			currentPositionCount, maxPositionCount)
	}

	logrus.WithFields(logrus.Fields{
		"symbol":                 estimate.Symbol,
		"current_position_count": currentPositionCount,
		"max_position_count":     maxPositionCount,
	}).Info("仓位数量检查通过")

	return nil
}
