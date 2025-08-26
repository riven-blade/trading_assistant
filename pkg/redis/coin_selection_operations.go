package redis

import (
	"encoding/json"
	"fmt"
	"time"
	"trading_assistant/models"

	"github.com/sirupsen/logrus"
)

// SetCoinSelection 设置币种选择状态
func (c *Client) SetCoinSelection(symbol string, status string) error {
	selection := &models.CoinSelection{
		Symbol:    symbol,
		Status:    status,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	key := fmt.Sprintf("%s:%s", KeyCoinSelection, symbol)
	data, err := json.Marshal(selection)
	if err != nil {
		return fmt.Errorf("序列化币种选择状态失败: %v", err)
	}

	err = c.rdb.Set(c.ctx, key, data, 0).Err()
	if err != nil {
		return fmt.Errorf("保存币种选择状态失败: %v", err)
	}

	logrus.WithFields(logrus.Fields{
		"symbol": symbol,
		"status": status,
	}).Info("币种选择状态已更新")

	return nil
}

// GetCoinSelection 获取币种选择状态
func (c *Client) GetCoinSelection(symbol string) (*models.CoinSelection, error) {
	key := fmt.Sprintf("%s:%s", KeyCoinSelection, symbol)
	data, err := c.rdb.Get(c.ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var selection models.CoinSelection
	err = json.Unmarshal([]byte(data), &selection)
	return &selection, err
}

// IsCoinSelected 检查币种是否选中
func (c *Client) IsCoinSelected(symbol string) bool {
	selection, err := c.GetCoinSelection(symbol)
	if err != nil {
		return false
	}
	return selection.Status == models.CoinSelectionActive
}

// GetSelectedCoinSymbols 获取所有选中的币种符号
func (c *Client) GetSelectedCoinSymbols() ([]string, error) {
	keys, err := c.rdb.Keys(c.ctx, fmt.Sprintf("%s:*", KeyCoinSelection)).Result()
	if err != nil {
		return nil, err
	}

	var selectedSymbols []string
	for _, key := range keys {
		data, err := c.rdb.Get(c.ctx, key).Result()
		if err != nil {
			continue
		}

		var selection models.CoinSelection
		if err := json.Unmarshal([]byte(data), &selection); err != nil {
			continue
		}

		if selection.Status == models.CoinSelectionActive {
			selectedSymbols = append(selectedSymbols, selection.Symbol)
		}
	}

	return selectedSymbols, nil
}

// GetSelectedCoinsWithDetails 获取选中的币种及其详细信息
func (c *Client) GetSelectedCoinsWithDetails() ([]*models.Coin, error) {
	selectedSymbols, err := c.GetSelectedCoinSymbols()
	if err != nil {
		return nil, fmt.Errorf("获取选中币种符号失败: %v", err)
	}

	var selectedCoins []*models.Coin
	for i := range selectedSymbols {
		symbol := selectedSymbols[i]
		coin, err := c.GetCoin(symbol)
		if err != nil {
			logrus.Warnf("获取币种详情失败 %s: %v", symbol, err)
			continue
		}
		selectedCoins = append(selectedCoins, coin)
	}

	return selectedCoins, nil
}

// RemoveCoinSelection 移除币种选择状态
func (c *Client) RemoveCoinSelection(symbol string) error {
	key := fmt.Sprintf("%s:%s", KeyCoinSelection, symbol)
	err := c.rdb.Del(c.ctx, key).Err()
	if err != nil {
		return fmt.Errorf("删除币种选择状态失败: %v", err)
	}

	logrus.WithFields(logrus.Fields{
		"symbol": symbol,
	}).Info("币种选择状态已移除")

	return nil
}

// GetAllCoinSelections 获取所有币种选择状态
func (c *Client) GetAllCoinSelections() ([]*models.CoinSelection, error) {
	keys, err := c.rdb.Keys(c.ctx, fmt.Sprintf("%s:*", KeyCoinSelection)).Result()
	if err != nil {
		return nil, err
	}

	var selections []*models.CoinSelection
	for _, key := range keys {
		data, err := c.rdb.Get(c.ctx, key).Result()
		if err != nil {
			logrus.Errorf("获取币种选择状态失败 %s: %v", key, err)
			continue
		}

		var selection models.CoinSelection
		if err := json.Unmarshal([]byte(data), &selection); err != nil {
			logrus.Errorf("解析币种选择状态失败 %s: %v", key, err)
			continue
		}
		selections = append(selections, &selection)
	}

	return selections, nil
}
