package redis

import (
	"fmt"
	"strconv"
	"trading_assistant/pkg/exchanges/types"
)

// KeyMarkPrice markPrice相关的Redis键
const (
	KeyMarkPrice = "mark_price" // markPrice键前缀
)

// SetMarkPrice 保存标记价格数据
func (c *Client) SetMarkPrice(markPrice *types.WatchMarkPrice) error {
	key := fmt.Sprintf("%s:%s", KeyMarkPrice, markPrice.Symbol)

	// 保存markPrice数据
	err := c.rdb.HMSet(c.ctx, key, map[string]interface{}{
		"symbol":       markPrice.Symbol,
		"mark_price":   markPrice.MarkPrice,
		"index_price":  markPrice.IndexPrice,
		"funding_rate": markPrice.FundingRate,
		"funding_time": markPrice.FundingTime,
		"timestamp":    markPrice.TimeStamp,
	}).Err()

	if err != nil {
		return fmt.Errorf("保存标记价格数据失败: %v", err)
	}

	return nil
}

// GetMarkPrice 获取标记价格数据
func (c *Client) GetMarkPrice(symbol string) (*types.WatchMarkPrice, error) {
	key := fmt.Sprintf("%s:%s", KeyMarkPrice, symbol)

	// 获取markPrice数据
	result, err := c.rdb.HMGet(c.ctx, key,
		"symbol", "mark_price", "index_price", "funding_rate", "funding_time", "timestamp").Result()
	if err != nil {
		return nil, fmt.Errorf("获取标记价格数据失败: %v", err)
	}

	// 检查数据是否存在
	if result[0] == nil {
		return nil, fmt.Errorf("标记价格数据不存在")
	}

	// 解析数据
	markPrice := &types.WatchMarkPrice{
		Symbol: result[0].(string),
	}

	if result[1] != nil {
		if markPriceStr, ok := result[1].(string); ok {
			if markPriceFloat, err := parseFloat64(markPriceStr); err == nil {
				markPrice.MarkPrice = markPriceFloat
			}
		}
	}

	if result[2] != nil {
		if indexPriceStr, ok := result[2].(string); ok {
			if indexPriceFloat, err := parseFloat64(indexPriceStr); err == nil {
				markPrice.IndexPrice = indexPriceFloat
			}
		}
	}

	if result[3] != nil {
		if fundingRateStr, ok := result[3].(string); ok {
			if fundingRateFloat, err := parseFloat64(fundingRateStr); err == nil {
				markPrice.FundingRate = fundingRateFloat
			}
		}
	}

	if result[4] != nil {
		if fundingTimeStr, ok := result[4].(string); ok {
			if fundingTimeInt, err := parseInt64(fundingTimeStr); err == nil {
				markPrice.FundingTime = fundingTimeInt
			}
		}
	}

	if result[5] != nil {
		if timestampStr, ok := result[5].(string); ok {
			if timestampInt, err := parseInt64(timestampStr); err == nil {
				markPrice.TimeStamp = timestampInt
			}
		}
	}

	return markPrice, nil
}

// DeleteMarkPrice 删除标记价格数据
func (c *Client) DeleteMarkPrice(symbol string) error {
	key := fmt.Sprintf("%s:%s", KeyMarkPrice, symbol)
	return c.rdb.Del(c.ctx, key).Err()
}

// 辅助函数：解析字符串到float64
func parseFloat64(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// 辅助函数：解析字符串到int64
func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
