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

// GetFuturesSymbols 获取期货交易对列表
func (b *BinanceClient) GetFuturesSymbols() ([]*models.Coin, error) {
	// 尝试从缓存获取
	var cachedResult []*models.Coin
	if redis.GlobalRedisClient != nil {
		if err := redis.GlobalRedisClient.GetCache(redis.CacheKeyFuturesSymbols, &cachedResult); err == nil {
			logrus.Debugf("从缓存获取交易对列表")
			return cachedResult, nil
		}
	}
	exchangeInfo, err := b.client.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("获取交易所信息失败: %v", err)
	}

	// 获取24小时统计信息
	tickers, err := b.client.NewListPriceChangeStatsService().Do(context.Background())
	if err != nil {
		logrus.Warnf("获取24小时统计信息失败: %v", err)
		// 不返回错误，继续处理基本信息
	}

	// 创建统计信息映射
	tickerMap := make(map[string]*futures.PriceChangeStats)
	for _, ticker := range tickers {
		tickerMap[ticker.Symbol] = ticker
	}

	var coins []*models.Coin
	for _, symbol := range exchangeInfo.Symbols {
		if symbol.Status == "TRADING" && symbol.ContractType == "PERPETUAL" {
			coin := &models.Coin{
				Symbol:            symbol.Symbol,
				BaseAsset:         symbol.BaseAsset,
				QuoteAsset:        symbol.QuoteAsset,
				Status:            "active",
				IsSelected:        false,
				PricePrecision:    symbol.PricePrecision,
				QuantityPrecision: symbol.QuantityPrecision,
				CreatedAt:         time.Now(),
				UpdatedAt:         time.Now(),
			}

			// 添加24小时统计信息
			if ticker, exists := tickerMap[symbol.Symbol]; exists {
				coin.Price = ticker.LastPrice
				coin.PriceChange = ticker.PriceChange
				coin.PriceChangePercent = ticker.PriceChangePercent
				coin.Volume = ticker.Volume
				coin.QuoteVolume = ticker.QuoteVolume
				coin.HighPrice = ticker.HighPrice
				coin.LowPrice = ticker.LowPrice
				coin.OpenPrice = ticker.OpenPrice
				coin.ClosePrice = ticker.LastPrice
				coin.Count = ticker.Count
			}

			// 从过滤器中提取精度信息
			for _, filter := range symbol.Filters {
				switch filter["filterType"] {
				case "PRICE_FILTER":
					if tickSize, ok := filter["tickSize"].(string); ok {
						coin.TickSize = tickSize
					}
					if minPrice, ok := filter["minPrice"].(string); ok {
						coin.MinPrice = minPrice
					}
					if maxPrice, ok := filter["maxPrice"].(string); ok {
						coin.MaxPrice = maxPrice
					}
				case "LOT_SIZE":
					if stepSize, ok := filter["stepSize"].(string); ok {
						coin.StepSize = stepSize
					}
					if minQty, ok := filter["minQty"].(string); ok {
						coin.MinQty = minQty
					}
					if maxQty, ok := filter["maxQty"].(string); ok {
						coin.MaxQty = maxQty
					}
				}
			}

			coins = append(coins, coin)
		}
	}

	// 存储到缓存
	if redis.GlobalRedisClient != nil {
		if err := redis.GlobalRedisClient.SetCache(redis.CacheKeyFuturesSymbols, coins); err != nil {
			logrus.Errorf("缓存交易对列表失败: %v", err)
		}
	}

	logrus.Infof("获取到 %d 个期货交易对，包含24小时统计信息", len(coins))
	return coins, nil
}

// GetOrderBook 获取订单薄
func (b *BinanceClient) GetOrderBook(symbol string, limit int) (*models.OrderBook, error) {
	depth, err := b.client.NewDepthService().Symbol(symbol).Limit(limit).Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("获取订单薄失败: %v", err)
	}

	orderBook := &models.OrderBook{
		Symbol:    symbol,
		Timestamp: time.Now().Unix(),
		Bids:      make([]models.PriceData, 0),
		Asks:      make([]models.PriceData, 0),
	}

	// 转换买单数据
	for _, bid := range depth.Bids {
		orderBook.Bids = append(orderBook.Bids, models.PriceData{
			Price:    bid.Price,
			Quantity: bid.Quantity,
		})
	}

	// 转换卖单数据
	for _, ask := range depth.Asks {
		orderBook.Asks = append(orderBook.Asks, models.PriceData{
			Price:    ask.Price,
			Quantity: ask.Quantity,
		})
	}

	return orderBook, nil
}

// GetSymbolPrecision 获取指定交易对的精度信息（从Redis中的coin数据读取）
func (b *BinanceClient) GetSymbolPrecision(symbol string) (*models.Coin, error) {
	// 直接从Redis中读取coin数据，精度信息在同步交易对时已经获取
	if redis.GlobalRedisClient != nil {
		coin, err := redis.GlobalRedisClient.GetCoin(symbol)
		if err == nil {
			logrus.Debugf("从Redis获取精度信息: %s", symbol)
			return coin, nil
		}
		logrus.Debugf("Redis中未找到交易对 %s，可能需要先同步交易对列表", symbol)
	}

	// 如果Redis中没有数据，返回错误提示需要先同步
	return nil, fmt.Errorf("未找到交易对 %s 的精度信息，请先同步交易对列表", symbol)
}

// GetKLines 获取K线数据
func (b *BinanceClient) GetKLines(symbol, interval string, limit int) ([]*models.KLineData, error) {
	// 构建缓存键
	cacheKey := fmt.Sprintf("%s:%s:%s:%d", redis.CacheKeyKLines, symbol, interval, limit)

	// 尝试从缓存获取
	var cachedResult []*models.KLineData
	if redis.GlobalRedisClient != nil {
		if err := redis.GlobalRedisClient.GetCache(cacheKey, &cachedResult); err == nil {
			logrus.Debugf("从缓存获取K线数据: %s %s", symbol, interval)
			return cachedResult, nil
		}
	}

	// 缓存未命中，从交易所获取
	klines, err := b.client.NewKlinesService().
		Symbol(symbol).
		Interval(interval).
		Limit(limit).
		Do(context.Background())

	if err != nil {
		return nil, fmt.Errorf("获取K线数据失败: %v", err)
	}

	var result []*models.KLineData
	for _, kline := range klines {
		open, _ := strconv.ParseFloat(kline.Open, 64)
		high, _ := strconv.ParseFloat(kline.High, 64)
		low, _ := strconv.ParseFloat(kline.Low, 64)
		close, _ := strconv.ParseFloat(kline.Close, 64)
		volume, _ := strconv.ParseFloat(kline.Volume, 64)

		result = append(result, &models.KLineData{
			Timestamp: kline.OpenTime,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
		})
	}

	// 存储到缓存
	if redis.GlobalRedisClient != nil {
		if err := redis.GlobalRedisClient.SetCache(cacheKey, result); err != nil {
			logrus.Errorf("缓存K线数据失败: %v", err)
		}
	}

	logrus.Infof("获取到 %d 条K线数据 %s %s", len(result), symbol, interval)
	return result, nil
}
