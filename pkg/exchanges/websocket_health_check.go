package exchanges

import (
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// healthCheckMarketData 市场数据健康检查
func (bws *BinanceWebSocketManager) healthCheckMarketData() {
	ticker := time.NewTicker(30 * time.Second) // 每30秒检查一次
	defer ticker.Stop()

	for {
		select {
		case <-bws.marketDataStopChan:
			return
		case <-ticker.C:
			if !bws.isMarketDataConnected() {
				logrus.Warn("市场数据连接健康检查失败，尝试重连")
				if err := bws.reconnectMarketData(); err != nil {
					logrus.Errorf("市场数据健康检查重连失败: %v", err)
				}
				continue
			}

			// 发送ping消息
			if bws.marketDataConn != nil {
				bws.marketDataMu.Lock()
				err := bws.marketDataConn.WriteMessage(websocket.PingMessage, []byte{})
				bws.marketDataMu.Unlock()

				if err != nil {
					logrus.Errorf("发送市场数据ping失败: %v", err)
					if err := bws.reconnectMarketData(); err != nil {
						logrus.Errorf("市场数据ping重连失败: %v", err)
					}
				} else {
					logrus.Debug("市场数据ping发送成功")
				}
			}
		}
	}
}
