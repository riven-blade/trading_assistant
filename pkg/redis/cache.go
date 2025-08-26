package redis

import (
	"encoding/json"
	"time"
)

// 缓存相关常量
const (
	CacheExpirationDefault   = 5 * time.Minute // 默认5分钟缓存
	CacheExpirationOrders    = 1 * time.Minute // 订单缓存1分钟
	CacheExpirationPositions = 0               // 持仓缓存永不过期
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

// SetBalancesRealtime 设置实时余额数据
func (c *Client) SetBalancesRealtime(balances interface{}) error {
	data, err := json.Marshal(balances)
	if err != nil {
		return err
	}
	// 使用永不过期设置
	return c.rdb.Set(c.ctx, CacheKeyBalancesRealtime, data, 0).Err()
}

// GetBalancesRealtime 获取实时余额数据
func (c *Client) GetBalancesRealtime(dest interface{}) error {
	data, err := c.rdb.Get(c.ctx, CacheKeyBalancesRealtime).Result()
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(data), dest)
}
