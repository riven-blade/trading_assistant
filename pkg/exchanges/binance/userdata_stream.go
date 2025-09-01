package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
	"trading_assistant/pkg/exchanges"
	"trading_assistant/pkg/exchanges/types"

	"github.com/sirupsen/logrus"
)

// UserDataStream 独立的用户数据流管理器
type UserDataStream struct {
	exchange *Binance

	// 连接管理
	connection *exchanges.WebSocketConnection
	active     int32 // 0=未激活, 1=已激活

	// 用户数据流相关
	listenKey   string
	publishFunc func(types.MetaData, interface{}) error

	// 生命周期管理
	ctx    context.Context
	cancel context.CancelFunc
	stopCh chan struct{}
	wg     sync.WaitGroup // 等待协程结束

	// 重连处理
	reconnectHandler func(int, error)
}

// NewUserDataStream 创建用户数据流管理器
func NewUserDataStream(exchange *Binance) *UserDataStream {
	ctx, cancel := context.WithCancel(context.Background())

	return &UserDataStream{
		exchange: exchange,
		active:   0,
		ctx:      ctx,
		cancel:   cancel,
		stopCh:   make(chan struct{}),
	}
}

// Start 启动用户数据流
func (uds *UserDataStream) Start(publishFunc func(types.MetaData, interface{}) error) error {
	// 使用CAS操作确保只启动一次
	if !atomic.CompareAndSwapInt32(&uds.active, 0, 1) {
		logrus.Warn("用户数据流已经活跃，跳过重复启动")
		return fmt.Errorf("user data stream already active")
	}

	uds.publishFunc = publishFunc

	// 创建listenKey
	listenKey, err := uds.exchange.CreateListenKey()
	if err != nil {
		return fmt.Errorf("创建listenKey失败: %w", err)
	}

	uds.listenKey = listenKey

	// 启动连接循环
	uds.wg.Add(1)
	go uds.connectionLoop()

	// 启动listenKey保活
	uds.wg.Add(1)
	go uds.keepaliveLoop()

	logrus.Info("用户数据流监听已启动，实时监控账户变化")
	return nil
}

// Stop 停止用户数据流
func (uds *UserDataStream) Stop() error {
	if atomic.LoadInt32(&uds.active) == 0 {
		return nil
	}

	atomic.StoreInt32(&uds.active, 0)

	// 停止所有协程
	close(uds.stopCh)
	uds.cancel()
	uds.wg.Wait() // 等待所有协程结束

	// 关闭连接
	if uds.connection != nil {
		uds.connection.Close()
		uds.connection = nil
	}

	// 关闭listenKey
	if uds.listenKey != "" {
		uds.exchange.CloseListenKey(uds.listenKey)
		uds.listenKey = ""
	}

	logrus.Info("用户数据流已停止")
	return nil
}

// connectionLoop 连接循环
func (uds *UserDataStream) connectionLoop() {
	defer uds.wg.Done()

	for {
		select {
		case <-uds.stopCh:
			return
		default:
			// 清理旧连接
			if uds.connection != nil {
				uds.connection.Close()
				uds.connection = nil
			}

			if err := uds.connect(); err != nil {
				logrus.Errorf("用户数据流连接失败: %v", err)
				select {
				case <-uds.stopCh:
					return
				case <-time.After(5 * time.Second):
					continue
				}
			}

			// 连接成功，等待连接关闭或停止信号
			select {
			case <-uds.stopCh:
				return
			case <-uds.ctx.Done():
				logrus.Warn("用户数据流连接断开，准备重连")
			case <-time.After(24 * time.Hour):
				logrus.Info("用户数据流24小时重连")
				if uds.connection != nil {
					uds.connection.Close()
				}
			}
		}
	}
}

// connect 建立连接
func (uds *UserDataStream) connect() error {
	if uds.listenKey == "" {
		return fmt.Errorf("listenKey为空")
	}

	// 构建URL
	baseURL := uds.getWebSocketURL()
	url := fmt.Sprintf("%s/%s", baseURL, uds.listenKey)

	// 创建连接，禁用自动重连（我们自己管理重连）
	conn, err := exchanges.NewWebSocketConnection(uds.ctx, url, 0)
	if err != nil {
		return fmt.Errorf("创建用户数据流连接失败: %w", err)
	}

	// 禁用客户端ping机制
	conn.SetPingEnabled(false)

	// 设置消息处理器
	conn.SetHandler(func(data []byte) error {
		return uds.handleMessage(data)
	})

	// 设置错误处理器
	conn.SetErrorHandler(func(err error) {
		logrus.Errorf("用户数据流连接错误: %v", err)
	})

	uds.connection = conn
	logrus.Info("用户数据流连接成功")
	return nil
}

// handleMessage 处理消息
func (uds *UserDataStream) handleMessage(data []byte) error {
	if uds.publishFunc == nil {
		return nil
	}

	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		return err
	}

	// 获取事件类型
	eventType, _ := msg["e"].(string)
	if eventType == "" {
		return nil
	}

	// 解析数据
	var parsedData interface{}
	switch eventType {
	case "ACCOUNT_UPDATE":
		parsedData = uds.parseAccountUpdate(msg)
	case "ORDER_TRADE_UPDATE":
		parsedData = uds.parseOrderUpdate(msg)
	case "TRADE_LITE":
		return nil // 跳过
	default:
		return nil
	}

	if parsedData == nil {
		return nil
	}

	// 构造MetaData
	metaData := types.MetaData{
		Exchange:  "binance",
		Market:    uds.getMarketType(),
		DataType:  uds.convertEventTypeToDataType(eventType),
		Timestamp: uds.extractTimestamp(msg),
	}

	return uds.publishFunc(metaData, parsedData)
}

// parseAccountUpdate 解析账户更新
func (uds *UserDataStream) parseAccountUpdate(msg map[string]interface{}) interface{} {
	result := &types.WatchAccountUpdate{
		EventType: getString(msg, "e"),
		EventTime: getInt64(msg, "E"),
		Info:      msg,
	}

	// 解析账户数据
	if accountData, ok := msg["a"].(map[string]interface{}); ok {
		if balances, ok := accountData["B"].([]interface{}); ok {
			for _, item := range balances {
				if balance, ok := item.(map[string]interface{}); ok {
					result.Balances = append(result.Balances, types.WatchBalanceUpdate{
						Asset:              getString(balance, "a"),
						WalletBalance:      getFloat64(balance, "wb"),
						CrossWalletBalance: getFloat64(balance, "cw"),
						BalanceChange:      getFloat64(balance, "bc"),
					})
				}
			}
		}
	}
	return result
}

// parseOrderUpdate 解析订单更新
func (uds *UserDataStream) parseOrderUpdate(msg map[string]interface{}) interface{} {
	orderData := msg
	if o, ok := msg["o"].(map[string]interface{}); ok {
		orderData = o
	}

	return &types.WatchOrderUpdate{
		EventType:          getString(msg, "e"),
		EventTime:          getInt64(msg, "E"),
		Symbol:             getString(orderData, "s"),
		ClientOrderID:      getString(orderData, "c"),
		Side:               getString(orderData, "S"),
		OrderType:          getString(orderData, "o"),
		OriginalQuantity:   getFloat64(orderData, "q"),
		OriginalPrice:      getFloat64(orderData, "p"),
		AveragePrice:       getFloat64(orderData, "ap"),
		ExecutionType:      getString(orderData, "x"),
		OrderStatus:        getString(orderData, "X"),
		OrderID:            getInt64(orderData, "i"),
		LastQuantityFilled: getFloat64(orderData, "l"),
		FilledAccumulated:  getFloat64(orderData, "z"),
		LastPriceFilled:    getFloat64(orderData, "L"),
		TradeTime:          getInt64(orderData, "T"),
		Info:               orderData,
	}
}

// keepaliveLoop listenKey保活循环
func (uds *UserDataStream) keepaliveLoop() {
	defer uds.wg.Done()

	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-uds.stopCh:
			return
		case <-ticker.C:
			if uds.listenKey != "" {
				if err := uds.exchange.KeepaliveListenKey(uds.listenKey); err != nil {
					logrus.Errorf("刷新listenKey失败: %v, 将重新创建连接", err)
					// 刷新失败时关闭当前连接，触发重连
					if uds.connection != nil {
						uds.connection.Close()
					}
					// 创建新的listenKey
					if newListenKey, createErr := uds.exchange.CreateListenKey(); createErr == nil {
						uds.listenKey = newListenKey
					}
				} else {
					logrus.Info("listenKey刷新成功")
				}
			}
		}
	}
}

// getWebSocketURL 获取WebSocket URL
func (uds *UserDataStream) getWebSocketURL() string {
	if uds.exchange != nil && uds.exchange.config != nil {
		return uds.exchange.config.GetWebSocketURL()
	}
	return "wss://fstream.binance.com/ws"
}

// getMarketType 获取市场类型
func (uds *UserDataStream) getMarketType() string {
	if uds.exchange != nil {
		return uds.exchange.marketType
	}
	return "futures"
}

// convertEventTypeToDataType 转换事件类型
func (uds *UserDataStream) convertEventTypeToDataType(eventType string) string {
	switch eventType {
	case "ACCOUNT_UPDATE":
		return "account"
	case "ORDER_TRADE_UPDATE":
		return "order"
	default:
		return "unknown"
	}
}

// extractTimestamp 提取时间戳
func (uds *UserDataStream) extractTimestamp(msg map[string]interface{}) int64 {
	if eventTime, exists := msg["E"]; exists {
		if timestamp, ok := eventTime.(float64); ok {
			return int64(timestamp)
		}
	}
	return time.Now().UnixMilli()
}

// SetReconnectHandler 设置重连处理器
func (uds *UserDataStream) SetReconnectHandler(handler func(int, error)) {
	uds.reconnectHandler = handler
}

// IsActive 检查是否活跃
func (uds *UserDataStream) IsActive() bool {
	return atomic.LoadInt32(&uds.active) == 1
}

// 工具函数
func getString(obj map[string]interface{}, key string) string {
	if val, exists := obj[key]; exists {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getFloat64(obj map[string]interface{}, key string) float64 {
	if val, exists := obj[key]; exists {
		switch v := val.(type) {
		case float64:
			return v
		case string:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return f
			}
		}
	}
	return 0
}

func getInt64(obj map[string]interface{}, key string) int64 {
	if val, exists := obj[key]; exists {
		switch v := val.(type) {
		case int64:
			return v
		case float64:
			return int64(v)
		case string:
			if i, err := strconv.ParseInt(v, 10, 64); err == nil {
				return i
			}
		}
	}
	return 0
}
