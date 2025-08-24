package exchanges

import (
	"encoding/json"
	"strconv"
	"time"
	"trading_assistant/models"
	"trading_assistant/pkg/redis"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// listenMarketData 监听市场数据
func (bws *BinanceWebSocketManager) listenMarketData() {
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("市场数据监听异常: %v", r)
		}
	}()

	for {
		select {
		case <-bws.marketDataStopChan:
			logrus.Info("市场数据监听已停止")
			return
		default:
			if bws.marketDataConn == nil {
				logrus.Warn("市场数据连接为空，尝试重连")
				if err := bws.reconnectMarketData(); err != nil {
					logrus.Errorf("市场数据重连失败: %v", err)
					time.Sleep(5 * time.Second)
					continue
				}
				continue
			}

			// 设置读取超时
			bws.marketDataConn.SetReadDeadline(time.Now().Add(30 * time.Second))

			messageType, message, err := bws.marketDataConn.ReadMessage()
			if err != nil {
				logrus.Errorf("读取市场数据消息失败: %v", err)
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					logrus.Error("市场数据WebSocket意外关闭，尝试重连")
				}
				if err := bws.reconnectMarketData(); err != nil {
					logrus.Errorf("市场数据重连失败: %v", err)
					time.Sleep(5 * time.Second)
				}
				continue
			}

			if messageType == websocket.PongMessage {
				bws.marketDataLastPong = time.Now()
				continue
			}

			if messageType != websocket.TextMessage {
				continue
			}

			// 解析消息
			orderBook, err := bws.parseMarketDataMessage(message)
			if err != nil {
				logrus.Debugf("解析市场数据消息失败: %v", err)
				continue
			}

			if orderBook != nil {
				// 保存到Redis
				if redis.GlobalRedisClient != nil {
					if err := redis.GlobalRedisClient.SetOrderBook(orderBook); err != nil {
						logrus.Errorf("保存订单薄失败: %v", err)
					}
				}
			}
		}
	}
}

// parseMarketDataMessage 解析市场数据消息
func (bws *BinanceWebSocketManager) parseMarketDataMessage(message []byte) (*models.OrderBook, error) {
	var streamData struct {
		Stream string `json:"stream"`
		Data   struct {
			EventType     string     `json:"e"`
			EventTime     int64      `json:"E"`
			Symbol        string     `json:"s"`
			FirstUpdateId int64      `json:"U"`
			FinalUpdateId int64      `json:"u"`
			Bids          [][]string `json:"b"`
			Asks          [][]string `json:"a"`
		} `json:"data"`
	}

	if err := json.Unmarshal(message, &streamData); err != nil {
		return nil, err
	}

	if streamData.Data.EventType != "depthUpdate" {
		return nil, nil
	}

	symbol := streamData.Data.Symbol
	if symbol == "" {
		return nil, nil
	}

	orderBook := &models.OrderBook{
		Symbol:    symbol,
		Bids:      make([]models.PriceData, 0, len(streamData.Data.Bids)),
		Asks:      make([]models.PriceData, 0, len(streamData.Data.Asks)),
		Timestamp: time.Now().Unix(),
	}

	// 转换买单
	for _, bid := range streamData.Data.Bids {
		if len(bid) >= 2 {
			quantity, _ := strconv.ParseFloat(bid[1], 64)
			if quantity > 0 { // 只保留数量大于0的订单
				orderBook.Bids = append(orderBook.Bids, models.PriceData{
					Price:    bid[0],
					Quantity: bid[1],
				})
			}
		}
	}

	// 转换卖单
	for _, ask := range streamData.Data.Asks {
		if len(ask) >= 2 {
			quantity, _ := strconv.ParseFloat(ask[1], 64)
			if quantity > 0 { // 只保留数量大于0的订单
				orderBook.Asks = append(orderBook.Asks, models.PriceData{
					Price:    ask[0],
					Quantity: ask[1],
				})
			}
		}
	}

	return orderBook, nil
}
