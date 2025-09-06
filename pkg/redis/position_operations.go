package redis

import (
	"encoding/json"
	"fmt"
	"strings"
	"trading_assistant/models"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

// SetPosition 设置持仓信息
func (c *Client) SetPosition(position *models.Position) error {
	if position.Size == 0 {
		key := fmt.Sprintf("%s:%s:%s", KeyPosition, position.Symbol, position.Side)
		return c.rdb.Del(c.ctx, key).Err()
	}

	key := fmt.Sprintf("%s:%s:%s", KeyPosition, position.Symbol, position.Side)
	data, err := json.Marshal(position)
	if err != nil {
		return err
	}
	return c.rdb.Set(c.ctx, key, data, 0).Err() // 永不过期
}

// GetPosition 获取特定持仓信息
func (c *Client) GetPosition(symbol, side string) (*models.Position, error) {
	// 将side转换为大写以匹配存储格式
	sideUpper := strings.ToUpper(side)
	key := fmt.Sprintf("%s:%s:%s", KeyPosition, symbol, sideUpper)

	data, err := c.rdb.Get(c.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	var position models.Position
	err = json.Unmarshal([]byte(data), &position)
	return &position, err
}

// GetAllPositions 获取所有持仓信息
func (c *Client) GetAllPositions() ([]*models.Position, error) {
	keys, err := c.rdb.Keys(c.ctx, fmt.Sprintf("%s:*", KeyPosition)).Result()
	if err != nil {
		return nil, err
	}

	var positions []*models.Position
	for i := range keys {
		key := keys[i]
		data, err := c.rdb.Get(c.ctx, key).Result()
		if err != nil {
			logrus.Errorf("获取持仓数据失败 %s: %v", key, err)
			continue
		}

		var position models.Position
		if err := json.Unmarshal([]byte(data), &position); err != nil {
			logrus.Errorf("解析持仓数据失败 %s: %v", key, err)
			continue
		}

		// 只返回持仓大小不为0的记录
		if position.Size != 0 {
			positions = append(positions, &position)
		}
	}

	return positions, nil
}

// ClearAllPositions 清除所有持仓信息
func (c *Client) ClearAllPositions() error {
	keys, err := c.rdb.Keys(c.ctx, fmt.Sprintf("%s:*", KeyPosition)).Result()
	if err != nil {
		return err
	}

	if len(keys) == 0 {
		logrus.Info("没有旧的持仓数据需要清除")
		return nil
	}

	// 批量删除所有持仓相关的key
	err = c.rdb.Del(c.ctx, keys...).Err()
	if err != nil {
		return err
	}

	logrus.Infof("已清除 %d 个旧的持仓数据", len(keys))
	return nil
}
