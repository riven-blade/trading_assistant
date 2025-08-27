package utils

import (
	"fmt"
	"time"
	"trading_assistant/models"
	"trading_assistant/pkg/redis"
	"trading_assistant/pkg/websocket"

	"github.com/sirupsen/logrus"
)

// BroadcastSymbolEstimatesUpdate 广播币种预估数据更新
func BroadcastSymbolEstimatesUpdate() {
	wsManager := websocket.GetGlobalWebSocketManager()
	if wsManager == nil {
		return
	}

	// 获取按币种分组的预估数据
	symbolEstimates, err := getSymbolEstimatesData()
	if err != nil {
		logrus.Errorf("获取币种预估数据失败: %v", err)
		return
	}

	// 推送按币种分组的具体预估数据
	updateData := map[string]interface{}{
		"symbolEstimates": symbolEstimates,
		"lastUpdate":      time.Now().Unix(),
	}

	wsManager.BroadcastEstimates(updateData)
	logrus.Debugf("通过WebSocket广播币种预估数据更新，包含 %d 个币种", len(symbolEstimates))
}

// getSymbolEstimatesData 获取按币种分组的监听预估数据
func getSymbolEstimatesData() (map[string][]*models.PriceEstimate, error) {
	if redis.GlobalRedisClient == nil {
		return nil, fmt.Errorf("redis客户端未初始化")
	}

	estimates, err := redis.GlobalRedisClient.GetAllEstimates()
	if err != nil {
		return nil, err
	}

	symbolEstimates := make(map[string][]*models.PriceEstimate)
	for i := range estimates {
		estimate := estimates[i]
		if estimate.Status == models.EstimateStatusListening {
			if symbolEstimates[estimate.Symbol] == nil {
				symbolEstimates[estimate.Symbol] = make([]*models.PriceEstimate, 0)
			}
			symbolEstimates[estimate.Symbol] = append(symbolEstimates[estimate.Symbol], estimate)
		}
	}

	return symbolEstimates, nil
}
