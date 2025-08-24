package exchanges

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"trading_assistant/models"
	"trading_assistant/pkg/redis"

	"github.com/sirupsen/logrus"
)

// GetPositions 获取持仓信息
func (b *BinanceClient) GetPositions() ([]*models.Position, error) {
	// 尝试从缓存获取
	var cachedResult []*models.Position
	if redis.GlobalRedisClient != nil {
		if err := redis.GlobalRedisClient.GetCache(redis.CacheKeyPositions, &cachedResult); err == nil {
			logrus.Debugf("从缓存获取持仓信息")
			return cachedResult, nil
		}
	}

	// 缓存未命中，从交易所获取
	// 使用PositionRisk API获取完整的持仓风险信息
	positionRisks, err := b.client.NewGetPositionRiskService().Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("获取持仓风险信息失败: %v", err)
	}

	var positions []*models.Position
	for i := range positionRisks {
		pos := positionRisks[i]
		size, _ := strconv.ParseFloat(pos.PositionAmt, 64)
		entryPrice, _ := strconv.ParseFloat(pos.EntryPrice, 64)
		unrealizedPnl, _ := strconv.ParseFloat(pos.UnRealizedProfit, 64)
		leverage, _ := strconv.Atoi(pos.Leverage)
		notional, _ := strconv.ParseFloat(pos.Notional, 64)   // 名义价值(持仓价值)
		markPrice, _ := strconv.ParseFloat(pos.MarkPrice, 64) // 标记价格

		if size != 0 { // 只返回有持仓的
			position := &models.Position{
				Symbol:        pos.Symbol,
				Side:          pos.PositionSide, // LONG 或 SHORT
				Size:          size,
				EntryPrice:    entryPrice,
				UnrealizedPnl: unrealizedPnl,
				Leverage:      leverage,
				MarginMode:    strings.ToLower(pos.MarginType), // cross 或 isolated
				MarkPrice:     markPrice,                       // 标记价格
				Notional:      math.Abs(notional),              // 持仓价值 (取绝对值)
				UpdatedAt:     time.Now(),
			}
			positions = append(positions, position)
		}
	}

	// 存储到缓存
	if redis.GlobalRedisClient != nil {
		if err := redis.GlobalRedisClient.SetCacheWithExpiration(redis.CacheKeyPositions, positions, redis.CacheExpirationPositions); err != nil {
			logrus.Errorf("缓存持仓信息失败: %v", err)
		}
	}

	logrus.Infof("获取到 %d 个持仓", len(positions))
	return positions, nil
}

// GetBalances 获取余额信息（带缓存）
func (b *BinanceClient) GetBalances() ([]*models.Balance, error) {
	// 尝试从缓存获取
	var cachedResult []*models.Balance
	if redis.GlobalRedisClient != nil {
		if err := redis.GlobalRedisClient.GetCache(redis.CacheKeyBalances, &cachedResult); err == nil {
			logrus.Debugf("从缓存获取余额信息")
			return cachedResult, nil
		}
	}

	account, err := b.client.NewGetAccountService().Do(context.Background())
	if err != nil {
		logrus.Errorf("获取账户信息失败: %v", err)
		return nil, err
	}

	var balances []*models.Balance
	for _, bal := range account.Assets {
		// 使用Binance Futures API的正确字段
		walletBalance, _ := strconv.ParseFloat(bal.WalletBalance, 64)
		crossWalletBalance, _ := strconv.ParseFloat(bal.CrossWalletBalance, 64)

		// 使用AvailableBalance作为可用余额
		var availableBalance float64
		if bal.AvailableBalance != "" {
			availableBalance, _ = strconv.ParseFloat(bal.AvailableBalance, 64)
		} else {
			availableBalance = crossWalletBalance
		}

		free := availableBalance
		total := walletBalance
		locked := total - free
		if locked < 0 {
			locked = 0
		}

		// 只返回有余额的资产
		if total > 0 || free > 0 {
			balance := &models.Balance{
				Asset:     bal.Asset,
				Free:      free,
				Locked:    locked,
				Total:     total,
				UpdatedAt: time.Now(),
			}
			balances = append(balances, balance)
		}
	}

	// 存储到缓存
	if redis.GlobalRedisClient != nil {
		if err := redis.GlobalRedisClient.SetCacheWithExpiration(redis.CacheKeyBalances, balances, redis.CacheExpirationBalances); err != nil {
			logrus.Errorf("缓存余额信息失败: %v", err)
		}
	}

	return balances, nil
}
