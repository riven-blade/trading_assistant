package exchanges

import (
	"github.com/sirupsen/logrus"
)

// NewBinanceWebSocketManager 创建Binance WebSocket管理器
func NewBinanceWebSocketManager(config *WebSocketConfig) *BinanceWebSocketManager {
	if config == nil {
		config = DefaultWebSocketConfig()
	}

	return &BinanceWebSocketManager{
		config:             config,
		marketDataSymbols:  make(map[string]bool),
		marketDataStopChan: make(chan bool),
		stopChan:           make(chan struct{}),
	}
}

// Stop 停止所有WebSocket连接
func (bws *BinanceWebSocketManager) Stop() error {
	bws.mu.Lock()
	defer bws.mu.Unlock()

	if !bws.running {
		return nil
	}

	bws.running = false

	// 发送停止信号
	close(bws.stopChan)

	// 停止市场数据流
	bws.StopMarketData()

	// 停止用户数据流
	bws.userDataMu.Lock()
	bws.userDataRunning = false
	bws.userDataMu.Unlock()

	logrus.Info("所有WebSocket连接已停止")
	return nil
}

// GetStatus 获取状态
func (bws *BinanceWebSocketManager) GetStatus() map[string]interface{} {
	bws.mu.RLock()
	defer bws.mu.RUnlock()

	bws.marketDataMu.RLock()
	marketDataConnected := bws.marketDataConn != nil && bws.isMarketDataConnected()
	marketDataSymbolsCount := len(bws.marketDataSymbols)
	bws.marketDataMu.RUnlock()

	bws.userDataMu.RLock()
	userDataRunning := bws.userDataRunning
	bws.userDataMu.RUnlock()

	return map[string]interface{}{
		"running":                        bws.running,
		"market_data_connected":          marketDataConnected,
		"market_data_symbols_count":      marketDataSymbolsCount,
		"user_data_running":              userDataRunning,
		"market_data_reconnect_attempts": bws.marketDataReconnectAttempts,
		"user_data_reconnect_attempts":   bws.userDataReconnectAttempts,
	}
}

// IsRunning 检查是否运行中
func (bws *BinanceWebSocketManager) IsRunning() bool {
	bws.mu.RLock()
	defer bws.mu.RUnlock()
	return bws.running
}
