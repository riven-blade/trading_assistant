package redis

import (
	"encoding/json"
	"fmt"
	"trading_assistant/models"

	"github.com/sirupsen/logrus"
)

// SetPriceEstimate 设置价格预估
func (c *Client) SetPriceEstimate(estimate *models.PriceEstimate) error {
	key := fmt.Sprintf("%s:%s", KeyPriceEstimate, estimate.ID)
	data, err := json.Marshal(estimate)
	if err != nil {
		return err
	}
	return c.rdb.Set(c.ctx, key, data, 0).Err()
}

// GetEstimateById 获取价格预估
func (c *Client) GetEstimateById(id string) (*models.PriceEstimate, error) {
	key := fmt.Sprintf("%s:%s", KeyPriceEstimate, id)
	data, err := c.rdb.Get(c.ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var estimate models.PriceEstimate
	err = json.Unmarshal([]byte(data), &estimate)
	return &estimate, err
}

// GetActiveEstimates 获取待处理的价格预估（enabled=true且status=listening）
func (c *Client) GetActiveEstimates() ([]*models.PriceEstimate, error) {
	keys, err := c.rdb.Keys(c.ctx, fmt.Sprintf("%s:*", KeyPriceEstimate)).Result()
	if err != nil {
		return nil, err
	}

	var estimates []*models.PriceEstimate
	for i := range keys {
		key := keys[i]
		data, err := c.rdb.Get(c.ctx, key).Result()
		if err != nil {
			continue
		}

		var estimate models.PriceEstimate
		if err := json.Unmarshal([]byte(data), &estimate); err != nil {
			continue
		}

		// 只返回enabled=true且status=listening的预估
		if estimate.Enabled && estimate.Status == models.EstimateStatusListening {
			estimates = append(estimates, &estimate)
		}
	}

	return estimates, nil
}

// GetEstimates 获取所有待处理价格预估
func (c *Client) GetEstimates() ([]*models.PriceEstimate, error) {
	keys, err := c.rdb.Keys(c.ctx, fmt.Sprintf("%s:*", KeyPriceEstimate)).Result()
	if err != nil {
		return nil, err
	}

	var estimates []*models.PriceEstimate
	for i := range keys {
		key := keys[i]
		data, err := c.rdb.Get(c.ctx, key).Result()
		if err != nil {
			continue
		}

		var estimate models.PriceEstimate
		if err := json.Unmarshal([]byte(data), &estimate); err != nil {
			continue
		}
		// 返回所有未完成的预估
		if estimate.Status == models.EstimateStatusListening {
			estimates = append(estimates, &estimate)
		}
	}

	return estimates, nil
}

// GetEstimatesBySymbol 根据交易对获取价格预估
func (c *Client) GetEstimatesBySymbol(symbol string) ([]*models.PriceEstimate, error) {
	keys, err := c.rdb.Keys(c.ctx, fmt.Sprintf("%s:*", KeyPriceEstimate)).Result()
	if err != nil {
		return nil, err
	}

	var estimates []*models.PriceEstimate
	for i := range keys {
		key := keys[i]
		data, err := c.rdb.Get(c.ctx, key).Result()
		if err != nil {
			logrus.Errorf("获取价格预估数据失败 %s: %v", key, err)
			continue
		}

		var estimate models.PriceEstimate
		if err := json.Unmarshal([]byte(data), &estimate); err != nil {
			logrus.Errorf("解析价格预估数据失败 %s: %v", key, err)
			continue
		}

		if estimate.Symbol == symbol && estimate.Status == models.EstimateStatusListening {
			estimates = append(estimates, &estimate)
		}
	}

	return estimates, nil
}

// DeletePriceEstimate 删除价格预估
func (c *Client) DeletePriceEstimate(id string) error {
	key := fmt.Sprintf("%s:%s", KeyPriceEstimate, id)
	return c.rdb.Del(c.ctx, key).Err()
}
