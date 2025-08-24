package redis

import (
	"context"
	"fmt"
	"time"
	"trading_assistant/pkg/config"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

type Client struct {
	rdb *redis.Client
	ctx context.Context
}

var GlobalRedisClient *Client

// InitRedis 初始化Redis客户端
func InitRedis() error {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", config.GlobalConfig.RedisHost, config.GlobalConfig.RedisPort),
		Password: config.GlobalConfig.RedisPassword,
		DB:       config.GlobalConfig.RedisDB,
	})

	ctx := context.Background()

	// 测试连接
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("redis连接失败: %v", err)
	}

	GlobalRedisClient = &Client{
		rdb: rdb,
		ctx: ctx,
	}

	logrus.Info("Redis连接成功")
	return nil
}

// Redis键名常量
const (
	KeyCoin          = "coin"
	KeyPriceEstimate = "price_estimate"
	KeyPosition      = "position"
	KeyOrderBook     = "orderbook"

	CacheKeyKLines         = "cache:klines"          // K线缓存
	CacheKeyBalances       = "cache:balances"        // 余额缓存
	CacheKeyOrders         = "cache:orders"          // 订单缓存
	CacheKeyPositions      = "cache:positions"       // 持仓缓存
	CacheKeyFuturesSymbols = "cache:futures_symbols" // 交易对缓存
)

// Get 基础Redis操作方法
func (c *Client) Get(key string) *redis.StringCmd {
	return c.rdb.Get(c.ctx, key)
}

func (c *Client) Set(key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	return c.rdb.Set(c.ctx, key, value, expiration)
}

func (c *Client) Info(section ...string) *redis.StringCmd {
	return c.rdb.Info(c.ctx, section...)
}
