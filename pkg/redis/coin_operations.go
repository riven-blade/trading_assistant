package redis

import (
	"encoding/json"
	"fmt"
	"trading_assistant/models"

	"github.com/sirupsen/logrus"
)

// SetCoin 设置币种信息
func (c *Client) SetCoin(coin *models.Coin) error {
	key := fmt.Sprintf("%s:%s", KeyCoin, coin.Symbol)
	data, err := json.Marshal(coin)
	if err != nil {
		return err
	}
	return c.rdb.Set(c.ctx, key, data, 0).Err()
}

// GetCoin 获取币种信息
func (c *Client) GetCoin(symbol string) (*models.Coin, error) {
	key := fmt.Sprintf("%s:%s", KeyCoin, symbol)
	data, err := c.rdb.Get(c.ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var coin models.Coin
	err = json.Unmarshal([]byte(data), &coin)
	return &coin, err
}

// GetAllCoins 获取所有币种信息
func (c *Client) GetAllCoins() ([]*models.Coin, error) {
	keys, err := c.rdb.Keys(c.ctx, fmt.Sprintf("%s:*", KeyCoin)).Result()
	if err != nil {
		return nil, err
	}

	var coins []*models.Coin
	for i := range keys {
		key := keys[i]
		data, err := c.rdb.Get(c.ctx, key).Result()
		if err != nil {
			logrus.Errorf("获取币种数据失败 %s: %v", key, err)
			continue
		}

		var coin models.Coin
		if err := json.Unmarshal([]byte(data), &coin); err != nil {
			logrus.Errorf("解析币种数据失败 %s: %v", key, err)
			continue
		}
		coins = append(coins, &coin)
	}
	return coins, nil
}

// DeleteCoin 删除币种信息
func (c *Client) DeleteCoin(symbol string) error {
	key := fmt.Sprintf("%s:%s", KeyCoin, symbol)
	return c.rdb.Del(c.ctx, key).Err()
}

// GetSelectedCoins 获取选中的币种
func (c *Client) GetSelectedCoins() ([]*models.Coin, error) {
	return c.GetSelectedCoinsWithDetails()
}
