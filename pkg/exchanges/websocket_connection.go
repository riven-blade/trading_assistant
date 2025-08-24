package exchanges

import (
	"fmt"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// connectMarketData 连接市场数据WebSocket
func (bws *BinanceWebSocketManager) connectMarketData() error {
	if bws.marketDataConn != nil {
		bws.marketDataConn.Close()
		bws.marketDataConn = nil
	}

	// 构建WebSocket URL
	streams := make([]string, 0, len(bws.marketDataSymbols))
	for symbol := range bws.marketDataSymbols {
		streams = append(streams, fmt.Sprintf("%s@depth20@100ms", symbol))
	}

	if len(streams) == 0 {
		return fmt.Errorf("没有交易对需要连接")
	}

	baseURL := "wss://fstream.binance.com/stream"
	if GlobalBinanceClient != nil && GlobalBinanceClient.client != nil {
		// 检查是否是测试网
		if GlobalBinanceClient.client.BaseURL == "https://testnet.binancefuture.com" {
			baseURL = "wss://stream.binancefuture.com/stream"
		}
	}

	// 构建参数
	streamParam := ""
	for i, stream := range streams {
		if i > 0 {
			streamParam += "/"
		}
		streamParam += stream
	}

	url := fmt.Sprintf("%s?streams=%s", baseURL, streamParam)
	logrus.Infof("正在连接市场数据WebSocket: %s", url)

	// 建立WebSocket连接
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("WebSocket连接失败: %v", err)
	}

	bws.marketDataConn = conn
	bws.marketDataLastPong = time.Now()

	logrus.Infof("市场数据WebSocket连接成功，订阅了 %d 个交易对", len(streams))
	return nil
}

// reconnectMarketData 重新连接市场数据流
func (bws *BinanceWebSocketManager) reconnectMarketData() error {
	bws.marketDataReconnectAttempts++
	delay := time.Duration(bws.marketDataReconnectAttempts) * 2 * time.Second
	if delay > 60*time.Second {
		delay = 60 * time.Second
	}

	logrus.Warnf("市场数据WebSocket重连中... (第%d次尝试，延迟%v)",
		bws.marketDataReconnectAttempts, delay)
	time.Sleep(delay)

	if err := bws.connectMarketData(); err != nil {
		return err
	}

	bws.marketDataReconnectAttempts = 0
	return nil
}

// isMarketDataConnected 检查市场数据连接状态
func (bws *BinanceWebSocketManager) isMarketDataConnected() bool {
	bws.marketDataMu.RLock()
	defer bws.marketDataMu.RUnlock()

	if bws.marketDataConn == nil {
		return false
	}

	// 检查是否超过心跳超时时间
	return time.Since(bws.marketDataLastPong) < 35*time.Second
}
