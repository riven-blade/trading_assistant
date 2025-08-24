package redis

import (
	"encoding/json"
	"fmt"
	"sort"
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
	for _, key := range keys {
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

// GetSelectedCoins 获取选中的币种
func (c *Client) GetSelectedCoins() ([]*models.Coin, error) {
	keys, err := c.rdb.Keys(c.ctx, fmt.Sprintf("%s:*", KeyCoin)).Result()
	if err != nil {
		return nil, err
	}

	var selectedCoins []*models.Coin
	for _, key := range keys {
		data, err := c.rdb.Get(c.ctx, key).Result()
		if err != nil {
			continue
		}

		var coin models.Coin
		if err := json.Unmarshal([]byte(data), &coin); err != nil {
			continue
		}

		if coin.IsSelected {
			selectedCoins = append(selectedCoins, &coin)
		}
	}

	// 按Symbol排序
	sort.Slice(selectedCoins, func(i, j int) bool {
		return selectedCoins[i].Symbol < selectedCoins[j].Symbol
	})

	return selectedCoins, nil
}
