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

// UserDataStream Binance 期货用户数据流 - 2025 最佳实现
type UserDataStream struct {
	// 核心组件
	exchange *Binance
	apiKey   string
	secret   string

	// 连接管理 - 使用原子指针避免锁竞争
	conn      atomic.Pointer[exchanges.WebSocketConnection]
	listenKey atomic.Pointer[string]

	// 状态管理 - 原子操作确保线程安全
	state     atomic.Int32 // 0=stopped, 1=connecting, 2=connected, 3=stopping
	connected atomic.Bool

	// 生命周期控制
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	shutdownOnce sync.Once

	// 消息处理 - 性能优化
	messageHandler func(types.MetaData, interface{}) error
	msgPool        sync.Pool   // 消息对象复用池
	msgChan        chan []byte // 消息通道，避免goroutine泄漏

	// 保活机制 - 双重保活策略
	listenKeyTicker   *time.Ticker
	connectionChecker *time.Ticker
	lastHeartbeat     atomic.Int64
	lastMessage       atomic.Int64

	// 重连机制 - 指数退避
	reconnectEnabled  atomic.Bool
	maxReconnectCount int32
	baseDelay         time.Duration
	maxDelay          time.Duration

	// 错误处理
	errorHandler     func(error)
	reconnectHandler func(int, error)

	// 性能统计
	stats StreamStats
}

// StreamStats 性能统计
type StreamStats struct {
	StartTime       int64
	ConnectedTime   atomic.Int64
	MessageCount    atomic.Int64
	ReconnectCount  atomic.Int32
	LastMessageTime atomic.Int64
	LastErrorTime   atomic.Int64
}

// UserDataEvent 用户数据事件
type UserDataEvent struct {
	EventType string                 `json:"e"`
	EventTime int64                  `json:"E"`
	Data      map[string]interface{} `json:"-"`
}

// 状态常量
const (
	StateStopped    int32 = 0
	StateConnecting int32 = 1
	StateConnected  int32 = 2
	StateStopping   int32 = 3
)

// 配置常量 - 根据官方文档优化
const (
	// Binance 官方推荐的保活间隔
	ListenKeyKeepaliveInterval = 30 * time.Minute // listenKey 保活
	ConnectionCheckInterval    = 10 * time.Second // 连接检查
	MessageTimeout             = 65 * time.Minute // 消息超时

	// 重连配置
	MaxReconnectCount  = 100
	BaseReconnectDelay = 1 * time.Second
	MaxReconnectDelay  = 30 * time.Second

	// 性能配置
	MessageBufferSize = 1000
	MaxMessageSize    = 1024 * 1024 // 1MB
)

// NewUserDataStream 创建期货用户数据流
func NewUserDataStream(exchange *Binance) *UserDataStream {
	stream := &UserDataStream{
		exchange:          exchange,
		apiKey:            exchange.config.APIKey,
		secret:            exchange.config.Secret,
		maxReconnectCount: MaxReconnectCount,
		baseDelay:         BaseReconnectDelay,
		maxDelay:          MaxReconnectDelay,
		stats: StreamStats{
			StartTime: time.Now().Unix(),
		},
	}

	// 初始化对象池
	stream.msgPool = sync.Pool{
		New: func() interface{} {
			return &UserDataEvent{
				Data: make(map[string]interface{}),
			}
		},
	}

	// 初始化消息通道，带缓冲避免阻塞
	stream.msgChan = make(chan []byte, MessageBufferSize)

	stream.reconnectEnabled.Store(true)
	return stream
}

// Start 启动用户数据流
func (s *UserDataStream) Start(messageHandler func(types.MetaData, interface{}) error) error {
	if !s.state.CompareAndSwap(StateStopped, StateConnecting) {
		return fmt.Errorf("stream already running, state: %d", s.state.Load())
	}

	if messageHandler == nil {
		s.state.Store(StateStopped)
		return fmt.Errorf("message handler cannot be nil")
	}

	if s.apiKey == "" || s.secret == "" {
		s.state.Store(StateStopped)
		return fmt.Errorf("API credentials required")
	}

	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.messageHandler = messageHandler

	// 重置统计
	s.stats.ReconnectCount.Store(0)
	s.stats.MessageCount.Store(0)

	// 启动主循环
	s.wg.Add(1)
	go s.mainLoop()

	// 启动消息处理器，避免goroutine泄漏
	s.wg.Add(1)
	go s.messageProcessor()

	logrus.Info("Binance futures user data stream starting...")
	return nil
}

// Stop 停止用户数据流
func (s *UserDataStream) Stop() error {
	s.shutdownOnce.Do(func() {
		currentState := s.state.Load()
		if currentState == StateStopped || currentState == StateStopping {
			return
		}

		s.state.Store(StateStopping)
		logrus.Info("Stopping Binance futures user data stream...")

		// 禁用重连
		s.reconnectEnabled.Store(false)

		// 取消上下文
		if s.cancel != nil {
			s.cancel()
		}

		// 停止定时器
		s.stopTimers()

		// 等待退出或超时
		done := make(chan struct{})
		go func() {
			s.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			logrus.Info("Stream stopped gracefully")
		case <-time.After(10 * time.Second):
			logrus.Warn("Stop timeout, forcing close")
		}

		// 清理连接
		s.closeConnection()

		// 清理消息通道
		s.drainMessageChannel()

		s.state.Store(StateStopped)
	})

	return nil
}

// messageProcessor 专用消息处理器，避免goroutine泄漏
func (s *UserDataStream) messageProcessor() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		case data := <-s.msgChan:
			s.handleMessage(data)
		}
	}
}

// mainLoop 主事件循环
func (s *UserDataStream) mainLoop() {
	defer s.wg.Done()
	defer s.closeConnection()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			if err := s.connect(); err != nil {
				if !s.reconnectEnabled.Load() {
					logrus.Info("Reconnect disabled, exiting")
					return
				}
				s.handleReconnect(err)
				continue
			}

			// 连接成功，开始监听
			s.state.Store(StateConnected)
			s.connected.Store(true)
			s.stats.ConnectedTime.Store(time.Now().Unix())
			s.stats.ReconnectCount.Store(0) // 重置重连计数

			// 启动保活机制
			s.startKeepalive()

			// 监听消息直到连接断开
			s.listenMessages()

			// 连接断开
			s.connected.Store(false)
			s.stopTimers()

			if !s.reconnectEnabled.Load() {
				return
			}
		}
	}
}

// connect 建立连接
func (s *UserDataStream) connect() error {
	listenKey, err := s.getValidListenKey()
	if err != nil {
		return fmt.Errorf("failed to get listen key: %w", err)
	}

	streamURL := fmt.Sprintf("wss://fstream.binance.com/ws/%s", listenKey)
	conn, err := exchanges.NewWebSocketConnection(s.ctx, streamURL, MessageBufferSize)
	if err != nil {
		return fmt.Errorf("failed to create connection: %w", err)
	}

	conn.SetPingEnabled(true) // 启用底层 ping
	conn.SetErrorHandler(s.handleConnectionError)

	s.conn.Store(conn)
	s.listenKey.Store(&listenKey)

	logrus.Infof("Connected to Binance futures user data stream with listenKey: %s...", listenKey[:8])
	return nil
}

// getValidListenKey 获取有效的 listenKey
func (s *UserDataStream) getValidListenKey() (string, error) {
	if s.exchange == nil {
		return "", fmt.Errorf("exchange client is nil")
	}

	// 尝试创建新的 listenKey
	listenKey, err := s.exchange.CreateListenKey()
	if err != nil {
		return "", fmt.Errorf("create listen key failed: %w", err)
	}

	if len(listenKey) < 8 {
		return "", fmt.Errorf("invalid listen key received")
	}

	logrus.Infof("成功创建listenKey: %s...", listenKey[:8])
	return listenKey, nil
}

// startKeepalive 启动保活机制
func (s *UserDataStream) startKeepalive() {
	// ListenKey 保活
	s.listenKeyTicker = time.NewTicker(ListenKeyKeepaliveInterval)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.listenKeyTicker.Stop()

		for {
			select {
			case <-s.ctx.Done():
				return
			case <-s.listenKeyTicker.C:
				s.keepaliveListenKey()
			}
		}
	}()

	// 连接健康检查 - 每10秒
	s.connectionChecker = time.NewTicker(ConnectionCheckInterval)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.connectionChecker.Stop()

		for {
			select {
			case <-s.ctx.Done():
				return
			case <-s.connectionChecker.C:
				if !s.isConnectionHealthy() {
					logrus.Warn("Connection unhealthy, triggering reconnect")
					s.handleConnectionError(fmt.Errorf("connection health check failed"))
					return
				}
			}
		}
	}()
}

// keepaliveListenKey 保持 listenKey 活跃
func (s *UserDataStream) keepaliveListenKey() {
	listenKeyPtr := s.listenKey.Load()
	if listenKeyPtr == nil {
		logrus.Warn("No listen key available for keepalive")
		return
	}

	listenKey := *listenKeyPtr
	if listenKey == "" {
		logrus.Warn("Empty listen key, skipping keepalive")
		return
	}

	if s.exchange == nil {
		logrus.Error("Exchange client is nil, cannot keepalive")
		return
	}

	if err := s.exchange.KeepaliveListenKey(listenKey); err != nil {
		logrus.Errorf("ListenKey keepalive failed: %v", err)
		s.handleConnectionError(err)
	} else {
		s.lastHeartbeat.Store(time.Now().Unix())
		logrus.Debug("ListenKey keepalive successful")
	}
}

// listenMessages 监听消息
func (s *UserDataStream) listenMessages() {
	conn := s.conn.Load()
	if conn == nil {
		return
	}

	// 设置消息处理器
	(*conn).SetHandler(func(data []byte) error {
		// 使用通道避免创建大量goroutine
		select {
		case s.msgChan <- data:
			// 消息发送成功
		default:
			// 通道满了，丢弃消息并记录警告
			logrus.Warn("Message channel full, dropping message")
		}
		return nil
	})

	// 连接会自动处理消息循环，我们只需要等待连接断开
	for s.connected.Load() {
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(time.Second):
			// 检查连接状态
			if !(*conn).IsConnected() {
				logrus.Warn("Connection lost")
				s.handleConnectionError(fmt.Errorf("connection lost"))
				return
			}
		}
	}
}

// handleMessage 处理消息 - 优化性能和安全
func (s *UserDataStream) handleMessage(data []byte) {
	// 消息有效性检查
	if len(data) == 0 {
		return
	}

	// 消息大小检查
	if len(data) > MaxMessageSize {
		logrus.Errorf("Message too large: %d bytes", len(data))
		return
	}

	s.stats.MessageCount.Add(1)
	s.stats.LastMessageTime.Store(time.Now().Unix())
	s.lastMessage.Store(time.Now().Unix())

	// 从对象池获取事件对象
	event := s.msgPool.Get().(*UserDataEvent)
	defer func() {
		// 清理并返回对象池
		event.EventType = ""
		event.EventTime = 0
		for k := range event.Data {
			delete(event.Data, k)
		}
		s.msgPool.Put(event)
	}()

	// 解析消息 - 增加错误恢复
	if err := json.Unmarshal(data, event); err != nil {
		logrus.Errorf("Parse message failed: %v, data: %s", err, string(data[:min(len(data), 100)]))
		return
	}

	// 解析完整数据
	if err := json.Unmarshal(data, &event.Data); err != nil {
		logrus.Errorf("Parse message data failed: %v", err)
		return
	}

	// 验证事件类型
	if event.EventType == "" {
		logrus.Warn("Received message with empty event type")
		return
	}

	// 处理特殊事件
	if event.EventType == "listenKeyExpired" {
		logrus.Warn("ListenKey expired, reconnecting...")
		s.handleConnectionError(fmt.Errorf("listen key expired"))
		return
	}

	// 构建并发送事件
	s.processUserDataEvent(event)
}

// processUserDataEvent 处理用户数据事件 - 增加panic保护
func (s *UserDataStream) processUserDataEvent(event *UserDataEvent) {
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("Panic in processUserDataEvent: %v", r)
		}
	}()

	if s.messageHandler == nil {
		return
	}

	var parsedEvent interface{}
	var dataType string

	switch event.EventType {
	case "ACCOUNT_UPDATE":
		parsedEvent = s.parseAccountUpdate(event.Data)
		dataType = "account"
	case "ORDER_TRADE_UPDATE":
		parsedEvent = s.parseOrderUpdate(event.Data)
		dataType = "order"
	case "MARGIN_CALL":
		parsedEvent = event.Data
		dataType = "margin_call"
	default:
		logrus.Debugf("Unhandled event type: %s", event.EventType)
		return
	}

	if parsedEvent == nil {
		return
	}

	metaData := types.MetaData{
		Exchange:  "binance",
		Market:    "futures",
		DataType:  dataType,
		Timestamp: event.EventTime,
	}

	if err := s.messageHandler(metaData, parsedEvent); err != nil {
		logrus.Errorf("Message handler error: %v", err)
	}
}

// parseAccountUpdate 解析账户更新 - 性能优化版本
func (s *UserDataStream) parseAccountUpdate(data map[string]interface{}) *types.WatchAccountUpdate {
	result := &types.WatchAccountUpdate{
		EventType: "ACCOUNT_UPDATE",
		EventTime: s.getInt64(data, "E"),
		Info:      data,
	}

	if accountData, ok := data["a"].(map[string]interface{}); ok {
		// 预分配切片容量
		if balances, ok := accountData["B"].([]interface{}); ok {
			result.Balances = make([]types.WatchBalanceUpdate, 0, len(balances))
			for _, item := range balances {
				if balance, ok := item.(map[string]interface{}); ok {
					result.Balances = append(result.Balances, types.WatchBalanceUpdate{
						Asset:              s.getString(balance, "a"),
						WalletBalance:      s.getFloat64(balance, "wb"),
						CrossWalletBalance: s.getFloat64(balance, "cw"),
						BalanceChange:      s.getFloat64(balance, "bc"),
					})
				}
			}
		}

		if positions, ok := accountData["P"].([]interface{}); ok {
			result.Positions = make([]types.WatchPositionUpdate, 0, len(positions))
			for _, item := range positions {
				if position, ok := item.(map[string]interface{}); ok {
					result.Positions = append(result.Positions, types.WatchPositionUpdate{
						Symbol:                 s.getString(position, "s"),
						PositionAmount:         s.getFloat64(position, "pa"),
						EntryPrice:             s.getFloat64(position, "ep"),
						PreAccumulatedRealized: s.getFloat64(position, "cr"),
						UnrealizedPnl:          s.getFloat64(position, "up"),
						MarginType:             s.getString(position, "mt"),
						IsolatedWallet:         s.getFloat64(position, "iw"),
						PositionSide:           s.getString(position, "ps"),
					})
				}
			}
		}
	}

	return result
}

// parseOrderUpdate 解析订单更新 - 性能优化版本
func (s *UserDataStream) parseOrderUpdate(data map[string]interface{}) *types.WatchOrderUpdate {
	var orderData map[string]interface{}
	if o, ok := data["o"].(map[string]interface{}); ok {
		orderData = o
	} else {
		orderData = data
	}

	return &types.WatchOrderUpdate{
		EventType:          "ORDER_TRADE_UPDATE",
		EventTime:          s.getInt64(data, "E"),
		Symbol:             s.getString(orderData, "s"),
		ClientOrderID:      s.getString(orderData, "c"),
		Side:               s.getString(orderData, "S"),
		OrderType:          s.getString(orderData, "o"),
		OriginalQuantity:   s.getFloat64(orderData, "q"),
		OriginalPrice:      s.getFloat64(orderData, "p"),
		AveragePrice:       s.getFloat64(orderData, "ap"),
		ExecutionType:      s.getString(orderData, "x"),
		OrderStatus:        s.getString(orderData, "X"),
		OrderID:            s.getInt64(orderData, "i"),
		LastQuantityFilled: s.getFloat64(orderData, "l"),
		FilledAccumulated:  s.getFloat64(orderData, "z"),
		LastPriceFilled:    s.getFloat64(orderData, "L"),
		TradeTime:          s.getInt64(orderData, "T"),
		RealizedProfit:     s.getFloat64(orderData, "rp"),
		Info:               data,
	}
}

// 连接健康检查
func (s *UserDataStream) isConnectionHealthy() bool {
	if !s.connected.Load() {
		return false
	}

	conn := s.conn.Load()
	if conn == nil || !(*conn).IsConnected() {
		return false
	}

	lastMsg := s.lastMessage.Load()
	lastHeartbeat := s.lastHeartbeat.Load()

	now := time.Now().Unix()

	// 如果从未收到消息，但连接时间不超过消息超时，则认为正常
	if lastMsg == 0 {
		connectedTime := s.stats.ConnectedTime.Load()
		if connectedTime > 0 && now-connectedTime < int64(MessageTimeout.Seconds()) {
			return true
		}
	}

	// 检查消息超时，但要考虑listenKey保活
	if lastMsg > 0 {
		timeSinceLastMsg := now - lastMsg
		timeSinceLastHeartbeat := now - lastHeartbeat

		// 如果消息超时但listenKey保活正常，则连接仍然健康
		if timeSinceLastMsg > int64(MessageTimeout.Seconds()) {
			if timeSinceLastHeartbeat < int64(ListenKeyKeepaliveInterval.Seconds()*2) {
				// listenKey保活正常，用户数据流可能只是没有交易活动
				logrus.Debug("No user data messages, but listenKey keepalive is healthy")
				return true
			}
			logrus.Warn("Message timeout detected - no messages and no successful keepalive")
			return false
		}
	}

	return true
}

// 错误处理和重连 - 指数退避
func (s *UserDataStream) handleConnectionError(err error) {
	s.stats.LastErrorTime.Store(time.Now().Unix())

	if s.errorHandler != nil {
		s.errorHandler(err)
	}

	// 关闭当前连接
	s.connected.Store(false)
	s.closeConnection()
}

func (s *UserDataStream) handleReconnect(err error) {
	if !s.reconnectEnabled.Load() {
		return
	}

	count := s.stats.ReconnectCount.Add(1)
	if count > s.maxReconnectCount {
		logrus.Errorf("Max reconnect attempts reached (%d), stopping", s.maxReconnectCount)
		s.reconnectEnabled.Store(false)
		return
	}

	if s.reconnectHandler != nil {
		s.reconnectHandler(int(count), err)
	}

	// 指数退避，带抖动
	delay := time.Duration(1<<uint(count-1)) * s.baseDelay
	if delay > s.maxDelay {
		delay = s.maxDelay
	}

	// 添加抖动减少雷群效应
	jitter := time.Duration(float64(delay) * 0.1 * (0.5 - float64(time.Now().UnixNano()%1000)/1000))
	delay += jitter

	logrus.Warnf("Reconnecting in %v (attempt %d)", delay, count)

	select {
	case <-time.After(delay):
	case <-s.ctx.Done():
		return
	}
}

// 资源清理
func (s *UserDataStream) closeConnection() {
	if conn := s.conn.Swap(nil); conn != nil {
		(*conn).Close()
	}
}

func (s *UserDataStream) stopTimers() {
	if s.listenKeyTicker != nil {
		s.listenKeyTicker.Stop()
	}
	if s.connectionChecker != nil {
		s.connectionChecker.Stop()
	}
}

// drainMessageChannel 清空消息通道
func (s *UserDataStream) drainMessageChannel() {
	if s.msgChan == nil {
		return
	}

	// 非阻塞清空通道
	for {
		select {
		case <-s.msgChan:
			// 丢弃未处理的消息
		default:
			return
		}
	}
}

// 工具方法 - 内联优化
func (s *UserDataStream) getString(obj map[string]interface{}, key string) string {
	if val, exists := obj[key]; exists {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func (s *UserDataStream) getFloat64(obj map[string]interface{}, key string) float64 {
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

func (s *UserDataStream) getInt64(obj map[string]interface{}, key string) int64 {
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

// 公共接口
func (s *UserDataStream) IsRunning() bool {
	state := s.state.Load()
	return state == StateConnecting || state == StateConnected
}

func (s *UserDataStream) IsConnected() bool {
	return s.connected.Load()
}

func (s *UserDataStream) GetStats() map[string]interface{} {
	uptime := time.Now().Unix() - s.stats.StartTime
	return map[string]interface{}{
		"state":             s.state.Load(),
		"connected":         s.connected.Load(),
		"uptime_seconds":    uptime,
		"message_count":     s.stats.MessageCount.Load(),
		"reconnect_count":   s.stats.ReconnectCount.Load(),
		"last_message_time": s.stats.LastMessageTime.Load(),
		"last_error_time":   s.stats.LastErrorTime.Load(),
		"connected_time":    s.stats.ConnectedTime.Load(),
		"is_healthy":        s.isConnectionHealthy(),
	}
}

// 配置方法
func (s *UserDataStream) SetErrorHandler(handler func(error)) {
	s.errorHandler = handler
}

func (s *UserDataStream) SetReconnectHandler(handler func(int, error)) {
	s.reconnectHandler = handler
}

func (s *UserDataStream) SetMaxReconnect(max int32) {
	s.maxReconnectCount = max
}

func (s *UserDataStream) EnableReconnect(enabled bool) {
	s.reconnectEnabled.Store(enabled)
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
