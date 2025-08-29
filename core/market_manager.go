package core

import (
	"context"
	"fmt"
	"time"
	"trading_assistant/models"
	"trading_assistant/pkg/exchanges/binance"
	"trading_assistant/pkg/redis"

	"github.com/sirupsen/logrus"
)

// MarketManager 市场数据管理器
type MarketManager struct {
	binanceClient *binance.Binance
	priceManager  *PriceManager
}

// NewMarketManager 创建市场数据管理器
func NewMarketManager(binanceClient *binance.Binance) *MarketManager {
	return &MarketManager{
		binanceClient: binanceClient,
		priceManager:  NewPriceManager(binanceClient),
	}
}

// GetPriceManager 获取价格管理器
func (mm *MarketManager) GetPriceManager() *PriceManager {
	return mm.priceManager
}

// InitializeMarketData 初始化市场数据
func (mm *MarketManager) InitializeMarketData() error {
	return mm.syncMarketData()
}

// SyncPriceData 同步价格数据
func (mm *MarketManager) SyncPriceData() error {
	return mm.syncPriceData()
}

// StartOrderBookSubscriptions 启动选中币种的OrderBook订阅
func (mm *MarketManager) StartOrderBookSubscriptions() error {
	logrus.Info("开始启动选中币种的OrderBook订阅...")

	// 启动OrderBook管理器
	if err := mm.priceManager.Start(); err != nil {
		return fmt.Errorf("启动OrderBook管理器失败: %v", err)
	}

	// 同步订阅状态
	if err := mm.priceManager.SyncSubscriptions(); err != nil {
		return fmt.Errorf("同步OrderBook订阅状态失败: %v", err)
	}

	logrus.Info("OrderBook订阅启动完成")
	return nil
}

// StopOrderBookSubscriptions 停止OrderBook订阅
func (mm *MarketManager) StopOrderBookSubscriptions() {
	if mm.priceManager != nil {
		mm.priceManager.Stop()
		logrus.Info("价格订阅已停止")
	}
}

// SyncOrderBookSubscriptions 同步OrderBook订阅状态
func (mm *MarketManager) SyncOrderBookSubscriptions() error {
	if mm.priceManager == nil || !mm.priceManager.IsRunning() {
		return fmt.Errorf("价格管理器未运行")
	}

	return mm.priceManager.SyncSubscriptions()
}

// GetOrderBookSubscriptionStatus 获取价格订阅状态
func (mm *MarketManager) GetOrderBookSubscriptionStatus() map[string]*PriceSubscription {
	if mm.priceManager == nil {
		return make(map[string]*PriceSubscription)
	}

	return mm.priceManager.GetSubscriptionStatus()
}

// SyncMarketAndPriceData 同步市场数据和价格数据
func (mm *MarketManager) SyncMarketAndPriceData() error {
	logrus.Info("开始同步市场数据和价格数据...")

	if err := mm.syncMarketData(); err != nil {
		return fmt.Errorf("同步市场数据失败: %w", err)
	}

	if err := mm.syncPriceData(); err != nil {
		return fmt.Errorf("同步价格数据失败: %w", err)
	}

	logrus.Info("市场数据和价格数据同步完成")
	return nil
}

// syncMarketData 同步市场数据
func (mm *MarketManager) syncMarketData() error {
	logrus.Info("开始同步市场数据...")

	// 获取所有USDT期货交易对
	markets, err := mm.binanceClient.FetchMarkets(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("获取市场数据失败: %v", err)
	}

	// 统计计数器
	var syncedCount int
	var usdtCount int
	validSymbols := make(map[string]bool) // 记录有效的symbol

	for i := range markets {
		market := markets[i]
		// 只处理活跃的USDT永续合约
		if !market.Active || market.Quote != "USDT" || !market.Swap {
			logrus.Debugf("跳过非永续合约: %s (Active: %v, Quote: %s, Swap: %v)",
				market.ID, market.Active, market.Quote, market.Swap)
			continue
		}

		usdtCount++
		validSymbols[market.ID] = true

		// 先创建基础币种信息
		coin := &models.Coin{
			Symbol:     market.ID,
			BaseAsset:  market.Base,
			QuoteAsset: market.Quote,
			Status:     "active",
			TickSize:   fmt.Sprintf("%.8f", market.Limits.Price.Step),
			StepSize:   fmt.Sprintf("%.8f", market.Limits.Amount.Step),
			MinPrice:   fmt.Sprintf("%.8f", market.Limits.Price.Min),
			MaxPrice:   fmt.Sprintf("%.8f", market.Limits.Price.Max),
			MinQty:     fmt.Sprintf("%.8f", market.Limits.Amount.Min),
			MaxQty:     fmt.Sprintf("%.8f", market.Limits.Amount.Max),
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		// 计算并设置正确的精度值
		coin.PricePrecision = coin.GetPricePrecisionFromTickSize()
		coin.QuantityPrecision = coin.GetQuantityPrecisionFromStepSize()

		logrus.WithFields(logrus.Fields{
			"symbol":             coin.Symbol,
			"tick_size":          coin.TickSize,
			"price_precision":    coin.PricePrecision,
			"step_size":          coin.StepSize,
			"quantity_precision": coin.QuantityPrecision,
		}).Debug("币种精度计算完成")

		// 保存到Redis
		if err := redis.GlobalRedisClient.SetCoin(coin); err != nil {
			logrus.Errorf("保存币种 %s 失败: %v", market.ID, err)
			continue
		}

		syncedCount++
	}

	if err := mm.cleanupInvalidCoins(validSymbols); err != nil {
		logrus.Warnf("清理无效币种失败: %v", err)
	}

	logrus.WithFields(logrus.Fields{
		"total_markets": len(markets),
		"usdt_markets":  usdtCount,
		"synced_count":  syncedCount,
	}).Info("市场数据同步完成")

	return nil
}

// cleanupInvalidCoins 清理不再有效的币种
func (mm *MarketManager) cleanupInvalidCoins(validSymbols map[string]bool) error {
	// 获取所有现有币种
	existingCoins, err := redis.GlobalRedisClient.GetAllCoins()
	if err != nil {
		return err
	}

	var deletedCount int
	for _, coin := range existingCoins {
		if !validSymbols[coin.Symbol] {
			// 这个币种不再有效，删除它
			if err := redis.GlobalRedisClient.DeleteCoin(coin.Symbol); err != nil {
				logrus.Errorf("删除无效币种 %s 失败: %v", coin.Symbol, err)
			} else {
				deletedCount++
			}
		}
	}

	if deletedCount > 0 {
		logrus.WithFields(logrus.Fields{
			"deleted_count": deletedCount,
		}).Info("清理无效币种完成")
	}

	return nil
}

// syncPriceData 同步价格数据
func (mm *MarketManager) syncPriceData() error {
	logrus.Info("开始同步价格数据...")

	// 获取所有币种列表
	coins, err := redis.GlobalRedisClient.GetAllCoins()
	if err != nil {
		return fmt.Errorf("获取币种列表失败: %v", err)
	}

	if len(coins) == 0 {
		logrus.Warn("没有找到币种数据，请先初始化市场数据")
		return nil
	}

	// 提取所有symbol
	var symbols []string
	coinMap := make(map[string]*models.Coin)
	for i := range coins {
		coin := coins[i]
		symbols = append(symbols, coin.Symbol)
		coinMap[coin.Symbol] = coin
	}

	logrus.WithFields(logrus.Fields{
		"total_symbols": len(symbols),
	}).Info("开始批量获取ticker数据...")

	if len(symbols) != len(coinMap) {
		logrus.Warnf("symbols和coinMap数量不一致: symbols=%d, coinMap=%d", len(symbols), len(coinMap))
	}

	tickers, err := mm.binanceClient.FetchTickers(context.Background(), symbols, nil)
	if err != nil {
		logrus.Errorf("批量获取ticker数据失败: %v", err)
		return fmt.Errorf("批量获取ticker数据失败: %v", err)
	}

	logrus.WithFields(logrus.Fields{
		"received_tickers":  len(tickers),
		"requested_symbols": len(symbols),
		"coins_from_redis":  len(coins),
	}).Info("ticker数据获取完成")

	// 更新币种价格信息
	var successCount, errorCount int
	now := time.Now()

	for symbol, coin := range coinMap {
		ticker, exists := tickers[symbol]
		if !exists {
			logrus.Warnf("币种 %s 未找到ticker数据", symbol)
			errorCount++
			continue
		}

		// 更新币种的价格和交易信息（从 ticker 数据获取）
		coin.Price = fmt.Sprintf("%.8f", ticker.Last)
		coin.PriceChange = fmt.Sprintf("%.8f", ticker.Change)
		coin.PriceChangePercent = fmt.Sprintf("%.2f", ticker.Percentage)
		coin.Volume = fmt.Sprintf("%.8f", ticker.BaseVolume)
		coin.QuoteVolume = fmt.Sprintf("%.8f", ticker.QuoteVolume)
		coin.UpdatedAt = now

		// 确保精度信息仍然正确（防止被覆盖）
		if coin.PricePrecision == 0 {
			coin.PricePrecision = coin.GetPricePrecisionFromTickSize()
		}
		if coin.QuantityPrecision == 0 {
			coin.QuantityPrecision = coin.GetQuantityPrecisionFromStepSize()
		}

		// 保存更新后的币种信息
		if err := redis.GlobalRedisClient.SetCoin(coin); err != nil {
			logrus.Errorf("保存 %s 价格数据失败: %v", symbol, err)
			errorCount++
			continue
		}

		successCount++
	}

	logrus.WithFields(logrus.Fields{
		"total_coins":   len(coins),
		"success_count": successCount,
		"error_count":   errorCount,
		"api_requests":  1, // 只用了1次API请求
	}).Info("价格数据同步完成")

	return nil
}
