package exchanges

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// StartMarketData 启动市场数据流
func (bws *BinanceWebSocketManager) StartMarketData(symbols []string) error {
	bws.marketDataMu.Lock()
	defer bws.marketDataMu.Unlock()

	if bws.marketDataRunning {
		return fmt.Errorf("市场数据流已在运行")
	}

	// 添加交易对
	for _, symbol := range symbols {
		bws.marketDataSymbols[strings.ToLower(symbol)] = true
	}

	if len(bws.marketDataSymbols) == 0 {
		return fmt.Errorf("没有交易对需要订阅")
	}

	// 启动市场数据连接
	if err := bws.connectMarketData(); err != nil {
		return err
	}

	bws.marketDataRunning = true
	bws.marketDataReconnectAttempts = 0

	// 启动监听协程
	go bws.listenMarketData()
	go bws.healthCheckMarketData()

	logrus.Infof("市场数据流已启动，订阅 %d 个交易对", len(bws.marketDataSymbols))
	return nil
}

// AddMarketDataSymbols 添加市场数据订阅
func (bws *BinanceWebSocketManager) AddMarketDataSymbols(symbols []string) error {
	bws.marketDataMu.Lock()
	defer bws.marketDataMu.Unlock()

	changed := false
	for _, symbol := range symbols {
		symbol = strings.ToLower(symbol)
		if !bws.marketDataSymbols[symbol] {
			bws.marketDataSymbols[symbol] = true
			changed = true
			logrus.Infof("添加市场数据订阅: %s", strings.ToUpper(symbol))
		}
	}

	// 如果有变化且正在运行，重连
	if changed && bws.marketDataRunning {
		logrus.Info("重新连接市场数据流以应用新的订阅")
		return bws.connectMarketData()
	}

	return nil
}

// RemoveMarketDataSymbol 移除市场数据订阅
func (bws *BinanceWebSocketManager) RemoveMarketDataSymbol(symbol string) error {
	bws.marketDataMu.Lock()
	defer bws.marketDataMu.Unlock()

	symbol = strings.ToLower(symbol)
	if bws.marketDataSymbols[symbol] {
		delete(bws.marketDataSymbols, symbol)
		logrus.Infof("移除市场数据订阅: %s", strings.ToUpper(symbol))

		// 如果正在运行，重连
		if bws.marketDataRunning {
			logrus.Info("重新连接市场数据流以应用订阅变化")
			return bws.connectMarketData()
		}
	}

	return nil
}

// StopMarketData 停止市场数据流
func (bws *BinanceWebSocketManager) StopMarketData() {
	bws.marketDataMu.Lock()
	defer bws.marketDataMu.Unlock()

	if !bws.marketDataRunning {
		return
	}

	bws.marketDataRunning = false

	// 发送停止信号
	close(bws.marketDataStopChan)
	bws.marketDataStopChan = make(chan bool)

	// 关闭连接
	if bws.marketDataConn != nil {
		bws.marketDataConn.Close()
		bws.marketDataConn = nil
	}

	logrus.Info("市场数据流已停止")
}

// GetMarketDataStatus 获取市场数据状态
func (bws *BinanceWebSocketManager) GetMarketDataStatus() map[string]interface{} {
	bws.marketDataMu.RLock()
	defer bws.marketDataMu.RUnlock()

	status := map[string]interface{}{
		"running":            bws.marketDataRunning,
		"connected":          bws.isMarketDataConnected(),
		"subscribed_symbols": len(bws.marketDataSymbols),
		"reconnect_attempts": bws.marketDataReconnectAttempts,
	}

	// 获取订阅的交易对列表
	symbols := make([]string, 0, len(bws.marketDataSymbols))
	for symbol := range bws.marketDataSymbols {
		symbols = append(symbols, strings.ToUpper(symbol))
	}
	status["symbols"] = symbols

	return status
}
