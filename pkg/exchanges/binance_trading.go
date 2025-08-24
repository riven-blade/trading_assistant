package exchanges

import (
	"context"
	"fmt"
	"strconv"
	"time"
	"trading_assistant/models"
	"trading_assistant/pkg/redis"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/sirupsen/logrus"
)

// PlaceOrder 下单 - 使用标准化类型
func (b *BinanceClient) PlaceOrder(symbol string, orderSide OrderSide, positionSide PositionSide, quantity, price float64, orderType OrderType, marginMode MarginMode) (*models.Order, error) {
	// 转换参数，确保所有参数都有效
	binanceSide, err := orderSide.ToBinanceSideType()
	if err != nil {
		return nil, fmt.Errorf("转换订单方向失败: %v", err)
	}

	binanceOrderType, err := orderType.ToBinanceOrderType()
	if err != nil {
		return nil, fmt.Errorf("转换订单类型失败: %v", err)
	}

	binancePositionSide, err := positionSide.ToBinancePositionSideType()
	if err != nil {
		return nil, fmt.Errorf("转换持仓方向失败: %v", err)
	}

	// 构建订单服务
	service := b.client.NewCreateOrderService().
		Symbol(symbol).
		Side(binanceSide).
		Type(binanceOrderType).
		Quantity(strconv.FormatFloat(quantity, 'f', -1, 64)).
		PositionSide(binancePositionSide)

	// 如果是限价单，设置价格
	if orderType == OrderTypeLimit {
		service = service.Price(strconv.FormatFloat(price, 'f', -1, 64)).TimeInForce(futures.TimeInForceTypeGTC)
	}

	orderResult, err := service.Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("下单失败: %v", err)
	}

	order := &models.Order{
		ID:         fmt.Sprintf("%d", orderResult.OrderID),
		Symbol:     symbol,
		Side:       string(orderSide),
		Type:       string(orderType),
		Quantity:   quantity,
		Price:      price,
		MarginMode: string(marginMode),
		Status:     string(orderResult.Status),
		ExchangeID: fmt.Sprintf("%d", orderResult.OrderID),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// 立即清理相关缓存，确保下次查询获取最新数据
	if redis.GlobalRedisClient != nil {
		// 清理该交易对的订单缓存
		orderCacheKey := fmt.Sprintf("%s:%s", redis.CacheKeyOrders, symbol)
		if err := redis.GlobalRedisClient.DeleteCache(orderCacheKey); err != nil {
			logrus.Errorf("清理订单缓存失败: %v", err)
		}

		// 清理全量订单缓存
		emptySymbolCacheKey := fmt.Sprintf("%s:", redis.CacheKeyOrders)
		if err := redis.GlobalRedisClient.DeleteCache(emptySymbolCacheKey); err != nil {
			logrus.Errorf("清理全量订单缓存失败: %v", err)
		}

		// 清理余额缓存（因为下单会冻结资金）
		if err := redis.GlobalRedisClient.DeleteCache(redis.CacheKeyBalances + "*"); err != nil {
			logrus.Errorf("清理余额缓存失败: %v", err)
		}

		logrus.Debugf("已清理下单相关缓存: %s", symbol)
	}

	logrus.Infof("订单创建成功: %s %s %f @ %f", symbol, string(orderSide), quantity, price)
	return order, nil
}

// GetOrders 获取订单状态（带缓存）
func (b *BinanceClient) GetOrders(symbol string) ([]*models.Order, error) {
	// 构建缓存键
	cacheKey := fmt.Sprintf("%s:%s", redis.CacheKeyOrders, symbol)

	// 尝试从缓存获取
	var cachedResult []*models.Order
	if redis.GlobalRedisClient != nil {
		if err := redis.GlobalRedisClient.GetCache(cacheKey, &cachedResult); err == nil {
			logrus.Debugf("从缓存获取订单信息: %s", symbol)
			return cachedResult, nil
		}
	}

	// 缓存未命中，从交易所获取
	var orders []*models.Order

	if symbol != "" {
		// 获取指定交易对的所有订单（包括历史订单）
		binanceOrders, err := b.client.NewListOrdersService().Symbol(symbol).Do(context.Background())
		if err != nil {
			return nil, fmt.Errorf("获取 %s 订单失败: %v", symbol, err)
		}

		for _, order := range binanceOrders {
			price, _ := strconv.ParseFloat(order.Price, 64)
			quantity, _ := strconv.ParseFloat(order.OrigQuantity, 64)
			executedQty, _ := strconv.ParseFloat(order.ExecutedQuantity, 64)

			modelOrder := &models.Order{
				ID:          fmt.Sprintf("%d", order.OrderID),
				Symbol:      order.Symbol,
				Side:        string(order.Side),
				Type:        string(order.Type),
				Quantity:    quantity,
				ExecutedQty: executedQty,
				Price:       price,
				Status:      string(order.Status),
				ExchangeID:  fmt.Sprintf("%d", order.OrderID),
				CreatedAt:   time.Unix(order.Time/1000, 0),
				UpdatedAt:   time.Unix(order.UpdateTime/1000, 0),
			}

			orders = append(orders, modelOrder)
		}
	} else {
		// 获取所有活跃订单
		binanceOrders, err := b.client.NewListOpenOrdersService().Do(context.Background())
		if err != nil {
			return nil, fmt.Errorf("获取活跃订单失败: %v", err)
		}

		for _, order := range binanceOrders {
			price, _ := strconv.ParseFloat(order.Price, 64)
			quantity, _ := strconv.ParseFloat(order.OrigQuantity, 64)
			executedQty, _ := strconv.ParseFloat(order.ExecutedQuantity, 64)

			modelOrder := &models.Order{
				ID:          fmt.Sprintf("%d", order.OrderID),
				Symbol:      order.Symbol,
				Side:        string(order.Side),
				Type:        string(order.Type),
				Quantity:    quantity,
				ExecutedQty: executedQty,
				Price:       price,
				Status:      string(order.Status),
				ExchangeID:  fmt.Sprintf("%d", order.OrderID),
				CreatedAt:   time.Unix(order.Time/1000, 0),
				UpdatedAt:   time.Unix(order.UpdateTime/1000, 0),
			}

			orders = append(orders, modelOrder)
		}
	}

	// 存储到缓存（使用最短的TTL）
	if redis.GlobalRedisClient != nil {
		if err := redis.GlobalRedisClient.SetCacheWithExpiration(cacheKey, orders, redis.CacheExpirationOrders); err != nil {
			logrus.Errorf("缓存订单信息失败: %v", err)
		}
	}

	logrus.Infof("获取到 %d 个订单", len(orders))
	return orders, nil
}

// CancelOrder 取消订单
func (b *BinanceClient) CancelOrder(symbol, orderID string) error {
	id, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return fmt.Errorf("订单ID格式错误: %v", err)
	}

	_, err = b.client.NewCancelOrderService().Symbol(symbol).OrderID(id).Do(context.Background())
	if err != nil {
		return fmt.Errorf("取消订单失败: %v", err)
	}

	// 立即清理相关缓存
	if redis.GlobalRedisClient != nil {
		// 清理该交易对的订单缓存
		orderCacheKey := fmt.Sprintf("%s:%s", redis.CacheKeyOrders, symbol)
		if err := redis.GlobalRedisClient.DeleteCache(orderCacheKey); err != nil {
			logrus.Errorf("清理订单缓存失败: %v", err)
		}

		// 清理全量订单缓存
		emptySymbolCacheKey := fmt.Sprintf("%s:", redis.CacheKeyOrders)
		if err := redis.GlobalRedisClient.DeleteCache(emptySymbolCacheKey); err != nil {
			logrus.Errorf("清理全量订单缓存失败: %v", err)
		}

		// 清理余额缓存（因为取消订单会释放冻结资金）
		if err := redis.GlobalRedisClient.DeleteCache(redis.CacheKeyBalances + "*"); err != nil {
			logrus.Errorf("清理余额缓存失败: %v", err)
		}

		logrus.Debugf("已清理取消订单相关缓存: %s", symbol)
	}

	logrus.Infof("订单 %s 已取消", orderID)
	return nil
}
