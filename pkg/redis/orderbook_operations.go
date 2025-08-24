package redis

import (
	"encoding/json"
	"fmt"
	"trading_assistant/models"
)

// SetOrderBook 设置订单薄
func (c *Client) SetOrderBook(orderBook *models.OrderBook) error {
	key := fmt.Sprintf("%s:%s", KeyOrderBook, orderBook.Symbol)
	data, err := json.Marshal(orderBook)
	if err != nil {
		return err
	}
	return c.rdb.Set(c.ctx, key, data, 0).Err()
}

// GetOrderBook 获取订单薄
func (c *Client) GetOrderBook(symbol string) (*models.OrderBook, error) {
	key := fmt.Sprintf("%s:%s", KeyOrderBook, symbol)
	data, err := c.rdb.Get(c.ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var orderBook models.OrderBook
	err = json.Unmarshal([]byte(data), &orderBook)
	return &orderBook, err
}
