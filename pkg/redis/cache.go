package redis

import (
	"encoding/json"
	"time"

	"github.com/sirupsen/logrus"
)

// 缓存相关常量
const (
	CacheExpirationDefault   = 5 * time.Minute  // 默认5分钟缓存
	CacheExpirationBalances  = 15 * time.Minute // 余额缓存15分钟
	CacheExpirationOrders    = 15 * time.Minute // 订单缓存15分钟
	CacheExpirationPositions = 1 * time.Minute  // 持仓缓存1分钟
)

// SetCache 设置缓存
func (c *Client) SetCache(key string, value interface{}) error {
	return c.SetCacheWithExpiration(key, value, CacheExpirationDefault)
}

// SetCacheWithExpiration 设置缓存
func (c *Client) SetCacheWithExpiration(key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.rdb.Set(c.ctx, key, data, expiration).Err()
}

// GetCache 获取缓存
func (c *Client) GetCache(key string, dest interface{}) error {
	data, err := c.rdb.Get(c.ctx, key).Result()
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(data), dest)
}

// DeleteCache 删除缓存
func (c *Client) DeleteCache(pattern string) error {
	keys, err := c.rdb.Keys(c.ctx, pattern).Result()
	if err != nil {
		return err
	}
	if len(keys) > 0 {
		return c.rdb.Del(c.ctx, keys...).Err()
	}
	return nil
}

// ClearOrderRelatedCache 清理订单相关的缓存
func (c *Client) ClearOrderRelatedCache() error {
	// 删除余额缓存
	if err := c.DeleteCache(CacheKeyBalances + "*"); err != nil {
		logrus.Errorf("清理余额缓存失败: %v", err)
	}
	// 删除订单缓存
	if err := c.DeleteCache(CacheKeyOrders + "*"); err != nil {
		logrus.Errorf("清理订单缓存失败: %v", err)
	}
	// 删除持仓缓存
	if err := c.DeleteCache(CacheKeyPositions + "*"); err != nil {
		logrus.Errorf("清理持仓缓存失败: %v", err)
	}
	logrus.Info("已清理订单相关缓存")
	return nil
}

// ClearAllTradingCache 清理所有交易相关缓存
func (c *Client) ClearAllTradingCache() error {
	// 清理订单相关缓存
	if err := c.ClearOrderRelatedCache(); err != nil {
		return err
	}

	// 删除K线缓存
	if err := c.DeleteCache(CacheKeyKLines + "*"); err != nil {
		logrus.Errorf("清理K线缓存失败: %v", err)
	}

	logrus.Info("已清理所有交易相关缓存")
	return nil
}
