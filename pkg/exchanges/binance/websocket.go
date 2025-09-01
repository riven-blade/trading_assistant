package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"trading_assistant/pkg/exchanges/types"

	"trading_assistant/pkg/exchanges"
)

// WebSocketConfig WebSocket配置
type WebSocketConfig struct {
	MaxConnections       int           `json:"maxConnections"`       // 最大连接数
	StreamsPerConnection int           `json:"streamsPerConnection"` // 每个连接的最大流数
	MaxReconnectAttempts int           `json:"maxReconnectAttempts"` // 最大重连次数
	BatchSize            int           `json:"batchSize"`            // 批量大小
	BatchInterval        time.Duration `json:"batchInterval"`        // 批量间隔
	HealthCheckInterval  time.Duration `json:"healthCheckInterval"`  // 健康检查间隔
}

// DefaultWebSocketConfig 默认配置
func DefaultWebSocketConfig() *WebSocketConfig {
	return &WebSocketConfig{
		MaxConnections:       10,
		StreamsPerConnection: 100, // 降低单连接负载，强制分散
		MaxReconnectAttempts: 5,
		BatchSize:            50,                     // 减小批量大小，更频繁处理
		BatchInterval:        100 * time.Millisecond, // 增加间隔，减少压力
		HealthCheckInterval:  30 * time.Second,
	}
}

// WebSocket Binance WebSocket客户端
type WebSocket struct {
	config   *WebSocketConfig
	exchange *Binance

	// 连接池
	connections []*WSConnection
	connMutex   sync.RWMutex

	// 批量处理
	batchChan chan string
	batchMap  sync.Map

	// 消息频率限制器
	msgRateLimiter *MessageRateLimiter

	// 发布函数
	publishFunc func(types.MetaData, interface{}) error

	// 重连事件处理函数
	reconnectHandler func(int, error)

	// 全局订阅状态跟踪
	allStreams    map[string]bool // 所有活跃的订阅流
	allStreamsMux sync.RWMutex    // 保护allStreams

	// 状态
	isRunning   int32
	msgCount    int64
	errorCount  int64
	lastMsgTime int64

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// WSConnection WebSocket连接
type WSConnection struct {
	ID          string
	ws          *exchanges.WebSocketConnection
	streamCount int32
	isHealthy   int32
	lastUsed    time.Time
	streams     map[string]bool // 跟踪此连接上的订阅流
	streamsMux  sync.RWMutex    // 保护streams map
}

// NewWebSocket 创建WebSocket客户端
func NewWebSocket(exchange *Binance, config *WebSocketConfig) *WebSocket {
	if config == nil {
		config = DefaultWebSocketConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	ws := &WebSocket{
		config:         config,
		exchange:       exchange,
		batchChan:      make(chan string, config.BatchSize*2),
		msgRateLimiter: NewMessageRateLimiter(),
		ctx:            ctx,
		cancel:         cancel,
		lastMsgTime:    time.Now().UnixMilli(),
		allStreams:     make(map[string]bool),
	}

	return ws
}

// Start 启动WebSocket客户端
func (ws *WebSocket) Start() error {
	if !atomic.CompareAndSwapInt32(&ws.isRunning, 0, 1) {
		return fmt.Errorf("websocket already running")
	}

	// 创建初始连接
	if err := ws.createConnection(); err != nil {
		atomic.StoreInt32(&ws.isRunning, 0)
		return fmt.Errorf("failed to create connection: %w", err)
	}

	// 启动批量处理器
	ws.wg.Add(1)
	go ws.batchProcessor()

	// 启动健康检查
	ws.wg.Add(1)
	go ws.healthChecker()

	return nil
}

// Stop 停止WebSocket客户端
func (ws *WebSocket) Stop() {
	if !atomic.CompareAndSwapInt32(&ws.isRunning, 1, 0) {
		return
	}

	ws.cancel()
	ws.wg.Wait()

	// 关闭连接
	ws.connMutex.Lock()
	for _, conn := range ws.connections {
		ws.closeConnection(conn)
	}
	ws.connections = nil
	ws.connMutex.Unlock()
}

// SubscribeStream 订阅数据流
func (ws *WebSocket) SubscribeStream(streamName string) error {
	if atomic.LoadInt32(&ws.isRunning) == 0 {
		return fmt.Errorf("websocket not running")
	}

	// 记录到全局订阅状态
	ws.allStreamsMux.Lock()
	ws.allStreams[streamName] = true
	ws.allStreamsMux.Unlock()

	// 添加到批量队列
	ws.addToBatch(streamName)
	return nil
}

// UnsubscribeStream 取消订阅数据流
func (ws *WebSocket) UnsubscribeStream(streamName string) error {
	if atomic.LoadInt32(&ws.isRunning) == 0 {
		return fmt.Errorf("websocket not running")
	}

	// 从全局订阅状态中删除
	ws.allStreamsMux.Lock()
	delete(ws.allStreams, streamName)
	ws.allStreamsMux.Unlock()

	conn := ws.selectBestConnection()
	if conn == nil {
		return fmt.Errorf("no connection available")
	}

	// 发送取消订阅
	unsubscribeMsg := map[string]interface{}{
		FieldMethod: MethodUnsubscribe,
		FieldParams: []string{streamName},
		FieldId:     time.Now().UnixNano(),
	}

	if err := ws.msgRateLimiter.Wait(ws.ctx); err != nil {
		return err
	}

	return conn.ws.SendMessage(unsubscribeMsg)
}

// createConnection 创建连接
func (ws *WebSocket) createConnection() error {
	ws.connMutex.Lock()
	defer ws.connMutex.Unlock()

	if len(ws.connections) >= ws.config.MaxConnections {
		return fmt.Errorf("max connections reached")
	}

	connID := fmt.Sprintf("conn_%d_%d", len(ws.connections), time.Now().UnixNano())
	wsURL := ws.getWebSocketURL()
	if wsURL == "" {
		return fmt.Errorf("websocket URL not configured")
	}

	wsInst, err := exchanges.NewWebSocketConnection(ws.ctx, wsURL, ws.config.MaxReconnectAttempts)
	if err != nil {
		return err
	}

	conn := &WSConnection{
		ID:        connID,
		ws:        wsInst,
		isHealthy: 1,
		lastUsed:  time.Now(),
		streams:   make(map[string]bool),
	}

	// 设置消息处理器
	wsInst.SetHandler(func(data []byte) error {
		return ws.handleMessage(data, conn)
	})

	// 设置错误处理器
	wsInst.SetErrorHandler(func(err error) {
		// 标记连接为不健康
		atomic.StoreInt32(&conn.isHealthy, 0)
	})

	// 设置重连处理器
	wsInst.SetReconnectHandler(func(attempt int, err error) {
		ws.handleReconnectEvent(attempt, err)
	})

	ws.connections = append(ws.connections, conn)
	return nil
}

// handleMessage 处理消息
func (ws *WebSocket) handleMessage(data []byte, conn *WSConnection) error {
	atomic.AddInt64(&ws.msgCount, 1)
	atomic.StoreInt64(&ws.lastMsgTime, time.Now().UnixMilli())

	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		atomic.AddInt64(&ws.errorCount, 1)
		return err
	}

	// 处理订阅确认
	if _, hasResult := msg[FieldResult]; hasResult {
		return nil
	}

	// 处理错误
	if errorMsg, ok := msg[FieldError]; ok {
		atomic.AddInt64(&ws.errorCount, 1)
		return fmt.Errorf("websocket error: %v", errorMsg)
	}

	// 解析并发布数据
	return ws.parseAndPublish(msg)
}

// parseAndPublish 解析消息并发布数据
func (ws *WebSocket) parseAndPublish(msg map[string]interface{}) error {
	if ws.publishFunc == nil {
		return nil
	}

	// 处理多路复用数据
	if dataField, ok := msg[FieldData].(map[string]interface{}); ok {
		msg = dataField
	}

	// 获取事件类型和symbol
	eventType, _ := msg[FieldEventType].(string)
	symbol, _ := msg[FieldSymbol].(string)
	streamName, _ := msg[FieldStream].(string)

	if eventType == "" || symbol == "" {
		// 尝试从stream字段解析
		if streamName != "" {
			eventType, symbol = ws.parseStreamInfo(streamName)
		}
	}

	if eventType == "" {
		return nil
	}

	// 市场数据事件需要symbol
	if symbol == "" {
		return nil
	}

	// 根据事件类型解析数据
	var parsedData interface{}
	switch eventType {
	case EventTypeDepthUpdate:
		parsedData = ws.parseDepthUpdate(msg)
	case EventTypeKline:
		parsedData = ws.parseKline(msg)
	case EventTypeBookTicker:
		parsedData = ws.parseBookTicker(msg)
	case EventTypeMarkPrice:
		parsedData = ws.parseMarkPrice(msg)
	default:
		return nil
	}

	if parsedData == nil {
		return nil
	}

	// 构造MetaData
	metaData := types.MetaData{
		Exchange:  "binance",
		Market:    ws.getMarketType(),
		MarketID:  symbol,
		DataType:  ws.convertEventTypeToDataType(eventType),
		Stream:    streamName,
		Timestamp: ws.extractTimestamp(msg),
	}

	if eventType == EventTypeKline && streamName != "" {
		metaData.Timeframe = ws.extractTimeframe(streamName)
	}

	// 调用发布函数
	return ws.publishFunc(metaData, parsedData)
}

// parseStreamInfo 解析流信息
func (ws *WebSocket) parseStreamInfo(streamName string) (eventType, symbol string) {
	parts := strings.Split(streamName, "@")
	if len(parts) >= 2 {
		symbol = strings.ToUpper(parts[0])
		eventTypePart := parts[1]

		if strings.HasPrefix(eventTypePart, StreamSuffixDepth) {
			return EventTypeDepthUpdate, symbol
		} else if strings.HasPrefix(eventTypePart, StreamSuffixKline) {
			return EventTypeKline, symbol
		} else if eventTypePart == StreamSuffixBookTicker {
			return EventTypeBookTicker, symbol
		} else if strings.HasPrefix(eventTypePart, StreamSuffixMarkPrice) {
			return EventTypeMarkPrice, symbol
		}
	}
	return "", ""
}

// parseDepthUpdate 解析深度更新
func (ws *WebSocket) parseDepthUpdate(msg map[string]interface{}) *types.WatchOrderBook {
	symbol := strings.ToUpper(ws.SafeString(msg, FieldSymbol, ""))
	if symbol == "" {
		return nil
	}

	bidsData, _ := msg[FieldBidPrice].([]interface{})
	asksData, _ := msg[FieldAskPrice].([]interface{})

	var bids, asks [][]float64
	for _, bidData := range bidsData {
		if bidArray, ok := bidData.([]interface{}); ok && len(bidArray) >= 2 {
			price, _ := strconv.ParseFloat(bidArray[0].(string), 64)
			quantity, _ := strconv.ParseFloat(bidArray[1].(string), 64)
			// 只保留数量大于0的价格档位（数量为0表示移除该档位）
			if quantity > 0 {
				bids = append(bids, []float64{price, quantity})
			}
		}
	}

	for _, askData := range asksData {
		if askArray, ok := askData.([]interface{}); ok && len(askArray) >= 2 {
			price, _ := strconv.ParseFloat(askArray[0].(string), 64)
			quantity, _ := strconv.ParseFloat(askArray[1].(string), 64)
			// 只保留数量大于0的价格档位（数量为0表示移除该档位）
			if quantity > 0 {
				asks = append(asks, []float64{price, quantity})
			}
		}
	}

	return &types.WatchOrderBook{
		Symbol:    symbol,
		TimeStamp: ws.extractTimestamp(msg),
		Bids:      bids,
		Asks:      asks,
		Nonce:     ws.SafeInt(msg, FieldUpdateId, 0),
	}
}

// parseKline 解析K线数据
func (ws *WebSocket) parseKline(msg map[string]interface{}) *types.Kline {
	klineData, ok := msg[FieldKlineData].(map[string]interface{})
	if !ok {
		return nil
	}

	symbol := strings.ToUpper(ws.SafeString(klineData, FieldSymbol, ""))
	if symbol == "" {
		return nil
	}

	return &types.Kline{
		Symbol:    symbol,
		Timeframe: ws.SafeString(klineData, FieldKlineInterval, ""),
		Timestamp: ws.SafeInt(klineData, FieldKlineStartTime, 0),
		Open:      ws.SafeFloat(klineData, FieldOpen, 0),
		High:      ws.SafeFloat(klineData, FieldHigh, 0),
		Low:       ws.SafeFloat(klineData, FieldLow, 0),
		Close:     ws.SafeFloat(klineData, FieldClose, 0),
		Volume:    ws.SafeFloat(klineData, FieldVolume, 0),
		IsClosed:  ws.SafeBool(klineData, "x", false),
	}
}

// parseBookTicker 解析最优订单簿价格
func (ws *WebSocket) parseBookTicker(msg map[string]interface{}) *types.WatchBookTicker {
	symbol := strings.ToUpper(ws.SafeString(msg, FieldSymbol, ""))
	if symbol == "" {
		return nil
	}

	return &types.WatchBookTicker{
		Symbol:      symbol,
		TimeStamp:   ws.extractTimestamp(msg),
		BidPrice:    ws.SafeFloat(msg, FieldBidPrice, 0),
		BidQuantity: ws.SafeFloat(msg, FieldBidQty, 0),
		AskPrice:    ws.SafeFloat(msg, FieldAskPrice, 0),
		AskQuantity: ws.SafeFloat(msg, FieldAskQty, 0),
	}
}

// parseMarkPrice 解析标记价格
func (ws *WebSocket) parseMarkPrice(msg map[string]interface{}) *types.WatchMarkPrice {
	symbol := strings.ToUpper(ws.SafeString(msg, FieldSymbol, ""))
	if symbol == "" {
		return nil
	}

	return &types.WatchMarkPrice{
		Symbol:      symbol,
		TimeStamp:   ws.extractTimestamp(msg),
		MarkPrice:   ws.SafeFloat(msg, FieldMarkPrice, 0),
		IndexPrice:  ws.SafeFloat(msg, FieldIndexPrice, 0),
		FundingRate: ws.SafeFloat(msg, FieldFundingRate, 0),
		FundingTime: ws.SafeInt(msg, FieldFundingTime, 0),
	}
}

// ========== 用户数据流解析方法 ==========

// parseAccountUpdate 解析账户更新事件
func (ws *WebSocket) parseAccountUpdate(msg map[string]interface{}) *types.WatchAccountUpdate {
	result := &types.WatchAccountUpdate{
		EventType:       ws.SafeString(msg, FieldEventType, ""),
		EventTime:       ws.SafeInt(msg, FieldEventTime, 0),
		TransactionTime: ws.SafeInt(msg, "T", 0), // 交易时间
		Info:            msg,
	}

	// 解析账户信息
	if accountData, ok := msg["a"].(map[string]interface{}); ok {
		// 解析余额信息
		if balancesData, ok := accountData["B"].([]interface{}); ok {
			for _, balanceItem := range balancesData {
				if balanceData, ok := balanceItem.(map[string]interface{}); ok {
					balance := types.WatchBalanceUpdate{
						EventType:          result.EventType,
						EventTime:          result.EventTime,
						Asset:              ws.SafeString(balanceData, "a", ""), // 资产名称
						WalletBalance:      ws.SafeFloat(balanceData, "wb", 0),  // 钱包余额
						CrossWalletBalance: ws.SafeFloat(balanceData, "cw", 0),  // 全仓钱包余额
						BalanceChange:      ws.SafeFloat(balanceData, "bc", 0),  // 余额变化
						Info:               balanceData,
					}
					result.Balances = append(result.Balances, balance)
				}
			}
		}

		// 解析持仓信息
		if positionsData, ok := accountData["P"].([]interface{}); ok {
			for _, positionItem := range positionsData {
				if positionData, ok := positionItem.(map[string]interface{}); ok {
					position := types.WatchPositionUpdate{
						EventType:              result.EventType,
						EventTime:              result.EventTime,
						Symbol:                 ws.SafeString(positionData, "s", ""),  // 交易对
						PositionAmount:         ws.SafeFloat(positionData, "pa", 0),   // 持仓数量
						EntryPrice:             ws.SafeFloat(positionData, "ep", 0),   // 持仓成本
						PreAccumulatedRealized: ws.SafeFloat(positionData, "cr", 0),   // 历史累计实现盈亏
						UnrealizedPnl:          ws.SafeFloat(positionData, "up", 0),   // 持仓未实现盈亏
						MarginType:             ws.SafeString(positionData, "mt", ""), // 保证金模式
						IsolatedWallet:         ws.SafeFloat(positionData, "iw", 0),   // 逐仓钱包余额
						PositionSide:           ws.SafeString(positionData, "ps", ""), // 持仓方向
						Info:                   positionData,
					}
					result.Positions = append(result.Positions, position)
				}
			}
		}
	}

	return result
}

// parseOrderTradeUpdate 解析订单交易更新事件
func (ws *WebSocket) parseOrderTradeUpdate(msg map[string]interface{}) *types.WatchOrderUpdate {
	// 获取订单数据
	var orderData map[string]interface{}
	if data, ok := msg["o"].(map[string]interface{}); ok {
		orderData = data
	} else {
		orderData = msg
	}

	return &types.WatchOrderUpdate{
		EventType:          ws.SafeString(msg, FieldEventType, ""),
		EventTime:          ws.SafeInt(msg, FieldEventTime, 0),
		Symbol:             ws.SafeString(orderData, "s", ""),            // 交易对
		ClientOrderID:      ws.SafeString(orderData, "c", ""),            // 客户端订单ID
		Side:               ws.SafeString(orderData, "S", ""),            // 买卖方向
		OrderType:          ws.SafeString(orderData, "o", ""),            // 订单类型
		TimeInForce:        ws.SafeString(orderData, "f", ""),            // 有效时间类型
		OriginalQuantity:   ws.SafeFloat(orderData, "q", 0),              // 原始数量
		OriginalPrice:      ws.SafeFloat(orderData, "p", 0),              // 原始价格
		AveragePrice:       ws.SafeFloat(orderData, "ap", 0),             // 平均成交价格
		StopPrice:          ws.SafeString(orderData, "sp", ""),           // 止损价格
		ExecutionType:      ws.SafeString(orderData, "x", ""),            // 执行类型
		OrderStatus:        ws.SafeString(orderData, "X", ""),            // 订单状态
		OrderID:            ws.SafeInt(orderData, "i", 0),                // 订单ID
		LastQuantityFilled: ws.SafeFloat(orderData, "l", 0),              // 成交数量
		FilledAccumulated:  ws.SafeFloat(orderData, "z", 0),              // 累计成交数量
		LastPriceFilled:    ws.SafeFloat(orderData, "L", 0),              // 成交价格
		CommissionAmount:   ws.SafeString(orderData, "n", ""),            // 手续费数量
		CommissionAsset:    ws.SafeString(orderData, "N", ""),            // 手续费资产类型
		TradeTime:          ws.SafeInt(orderData, "T", 0),                // 成交时间
		TradeID:            ws.SafeInt(orderData, "t", 0),                // 成交ID
		BidsNotional:       ws.SafeString(orderData, "b", ""),            // 买单净值
		AsksNotional:       ws.SafeString(orderData, "a", ""),            // 卖单净值
		IsMakerSide:        ws.SafeString(orderData, "m", "") == "true",  // 是否为挂单成交
		IsReduceOnly:       ws.SafeString(orderData, "R", "") == "true",  // 是否为只减仓单
		WorkingType:        ws.SafeString(orderData, "wt", ""),           // 条件价格触发类型
		OriginalOrderType:  ws.SafeString(orderData, "ot", ""),           // 原始订单类型
		PositionSide:       ws.SafeString(orderData, "ps", ""),           // 持仓方向
		IsClosePosition:    ws.SafeString(orderData, "cp", "") == "true", // 是否条件全平仓
		ActivationPrice:    ws.SafeString(orderData, "AP", ""),           // 跟踪止损激活价格
		CallbackRate:       ws.SafeString(orderData, "cr", ""),           // 跟踪止损回调比例
		RealizedProfit:     ws.SafeFloat(orderData, "rp", 0),             // 该交易实现盈亏
		Info:               msg,
	}
}

// 辅助方法
func (ws *WebSocket) convertEventTypeToDataType(eventType string) string {
	switch eventType {
	case EventTypeKline:
		return "kline"
	case EventTypeBookTicker:
		return "bookTicker"
	case EventTypeMarkPrice:
		return "markPrice"
	case EventTypeDepthUpdate:
		return "orderbook"
	case EventTypeAccountUpdate:
		return "account"
	case EventTypeOrderTradeUpdate:
		return "order"
	case EventTypeBalanceUpdate:
		return "balance"
	case EventTypeExecutionReport:
		return "execution"
	case EventTypeOutboundAccountPosition:
		return "position"
	default:
		return ""
	}
}

func (ws *WebSocket) getMarketType() string {
	return ws.exchange.marketType
}

// isUserDataEvent 判断是否为用户数据流事件
func (ws *WebSocket) isUserDataEvent(eventType string) bool {
	switch eventType {
	case EventTypeAccountUpdate, EventTypeOrderTradeUpdate, EventTypeBalanceUpdate,
		EventTypeExecutionReport, EventTypeOutboundAccountPosition:
		return true
	default:
		return false
	}
}

func (ws *WebSocket) extractTimeframe(streamName string) string {
	parts := strings.Split(streamName, "@")
	if len(parts) >= 2 && strings.HasPrefix(parts[1], "kline_") {
		return strings.TrimPrefix(parts[1], "kline_")
	}
	return ""
}

func (ws *WebSocket) extractTimestamp(msg map[string]interface{}) int64 {
	if eventTime, exists := msg[FieldEventTime]; exists {
		if timestamp, ok := eventTime.(float64); ok {
			return int64(timestamp)
		}
		if timestampStr, ok := eventTime.(string); ok {
			if timestamp, err := strconv.ParseInt(timestampStr, 10, 64); err == nil {
				return timestamp
			}
		}
	}
	return time.Now().UnixMilli()
}

func (ws *WebSocket) SafeString(obj map[string]interface{}, key string, defaultValue string) string {
	if val, exists := obj[key]; exists {
		if str, ok := val.(string); ok {
			return str
		}
		return fmt.Sprintf("%v", val)
	}
	return defaultValue
}

func (ws *WebSocket) SafeFloat(obj map[string]interface{}, key string, defaultValue float64) float64 {
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
	return defaultValue
}

func (ws *WebSocket) SafeInt(obj map[string]interface{}, key string, defaultValue int64) int64 {
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
	return defaultValue
}

func (ws *WebSocket) SafeBool(obj map[string]interface{}, key string, defaultValue bool) bool {
	if val, exists := obj[key]; exists {
		if b, ok := val.(bool); ok {
			return b
		}
		if str, ok := val.(string); ok {
			return strings.ToLower(str) == "true" || str == "1"
		}
	}
	return defaultValue
}

// 消息频率限制器实现
type MessageRateLimiter struct {
	interval time.Duration
	lastSent time.Time
	mutex    sync.Mutex
}

func NewMessageRateLimiter() *MessageRateLimiter {
	return &MessageRateLimiter{
		interval: 150 * time.Millisecond, // 稍微降低间隔，配合小批量
	}
}

func (mrl *MessageRateLimiter) Wait(ctx context.Context) error {
	mrl.mutex.Lock()
	defer mrl.mutex.Unlock()

	now := time.Now()
	elapsed := now.Sub(mrl.lastSent)

	if elapsed < mrl.interval {
		waitTime := mrl.interval - elapsed
		timer := time.NewTimer(waitTime)
		defer timer.Stop()

		select {
		case <-timer.C:
			mrl.lastSent = time.Now()
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	mrl.lastSent = now
	return nil
}

// 批量处理相关方法
func (ws *WebSocket) addToBatch(streamName string) {
	if _, exists := ws.batchMap.LoadOrStore(streamName, true); exists {
		return
	}

	select {
	case ws.batchChan <- streamName:
	default:
		// 批量队列已满，忽略
	}
}

func (ws *WebSocket) batchProcessor() {
	defer ws.wg.Done()

	batch := make([]string, 0, ws.config.BatchSize)
	ticker := time.NewTicker(ws.config.BatchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ws.ctx.Done():
			if len(batch) > 0 {
				ws.processBatch(batch)
			}
			return

		case stream := <-ws.batchChan:
			batch = append(batch, stream)
			if len(batch) >= ws.config.BatchSize {
				ws.processBatch(batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			if len(batch) > 0 {
				ws.processBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

func (ws *WebSocket) processBatch(streams []string) {
	if len(streams) == 0 {
		return
	}

	conn := ws.selectBestConnection()
	if conn == nil {
		return
	}

	// 清除批量映射
	for _, stream := range streams {
		ws.batchMap.Delete(stream)
	}

	// 发送订阅
	subscribeMsg := map[string]interface{}{
		FieldMethod: MethodSubscribe,
		FieldParams: streams,
		FieldId:     time.Now().UnixNano(),
	}

	if err := ws.msgRateLimiter.Wait(ws.ctx); err != nil {
		// 重新添加到队列
		for _, stream := range streams {
			ws.addToBatch(stream)
		}
		return
	}

	if err := conn.ws.SendMessage(subscribeMsg); err != nil {
		// 重新添加到队列
		for _, stream := range streams {
			ws.addToBatch(stream)
		}
		return
	}

	// 更新连接的流计数和订阅跟踪
	atomic.AddInt32(&conn.streamCount, int32(len(streams)))
	conn.lastUsed = time.Now()

	// 跟踪此连接上的订阅流
	conn.streamsMux.Lock()
	for i := range streams {
		stream := streams[i]
		conn.streams[stream] = true
	}
	conn.streamsMux.Unlock()
}

func (ws *WebSocket) selectBestConnection() *WSConnection {
	ws.connMutex.Lock()
	defer ws.connMutex.Unlock()

	var bestConn *WSConnection
	var minLoad int32 = int32(ws.config.StreamsPerConnection)

	for _, conn := range ws.connections {
		if atomic.LoadInt32(&conn.isHealthy) == 0 {
			continue
		}

		load := atomic.LoadInt32(&conn.streamCount)
		if load < minLoad {
			minLoad = load
			bestConn = conn
		}
	}

	// 积极创建新连接分散负载
	if bestConn == nil || minLoad > int32(ws.config.StreamsPerConnection/2) { // 降低阈值，更早分散
		if len(ws.connections) < ws.config.MaxConnections {
			if err := ws.createConnectionUnsafe(); err == nil {
				if len(ws.connections) > 0 {
					newConn := ws.connections[len(ws.connections)-1]
					if atomic.LoadInt32(&newConn.isHealthy) == 1 {
						return newConn
					}
				}
			}
		}
	}

	return bestConn
}

func (ws *WebSocket) createConnectionUnsafe() error {
	connID := fmt.Sprintf("conn_%d_%d", len(ws.connections), time.Now().UnixNano())
	wsURL := ws.getWebSocketURL()

	wsInst, err := exchanges.NewWebSocketConnection(ws.ctx, wsURL, ws.config.MaxReconnectAttempts)
	if err != nil {
		return err
	}

	conn := &WSConnection{
		ID:        connID,
		ws:        wsInst,
		isHealthy: 1,
		lastUsed:  time.Now(),
		streams:   make(map[string]bool),
	}

	wsInst.SetHandler(func(data []byte) error {
		return ws.handleMessage(data, conn)
	})

	wsInst.SetErrorHandler(func(err error) {
		atomic.StoreInt32(&conn.isHealthy, 0)
	})

	// 设置重连处理器
	wsInst.SetReconnectHandler(func(attempt int, err error) {
		ws.handleReconnectEvent(attempt, err)
	})

	ws.connections = append(ws.connections, conn)
	return nil
}

func (ws *WebSocket) closeConnection(conn *WSConnection) {
	if conn.ws != nil {
		conn.ws.Close()
	}

	// 清理连接的订阅跟踪
	conn.streamsMux.Lock()
	conn.streams = make(map[string]bool)
	conn.streamsMux.Unlock()
}

func (ws *WebSocket) healthChecker() {
	defer ws.wg.Done()

	ticker := time.NewTicker(ws.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ws.ctx.Done():
			return
		case <-ticker.C:
			ws.checkHealth()
		}
	}
}

func (ws *WebSocket) checkHealth() {
	ws.connMutex.Lock()
	defer ws.connMutex.Unlock()

	var lostStreams []string

	for i := len(ws.connections) - 1; i >= 0; i-- {
		conn := ws.connections[i]
		if atomic.LoadInt32(&conn.isHealthy) == 0 || !conn.ws.IsConnected() {
			// 收集丢失的订阅流
			conn.streamsMux.RLock()
			for stream := range conn.streams {
				lostStreams = append(lostStreams, stream)
			}
			conn.streamsMux.RUnlock()

			ws.closeConnection(conn)
			ws.connections = append(ws.connections[:i], ws.connections[i+1:]...)
		}
	}

	// 如果没有健康连接，创建新连接
	if len(ws.connections) == 0 {
		ws.createConnectionUnsafe()
	}

	// 恢复丢失的订阅
	for i := range lostStreams {
		stream := lostStreams[i]
		ws.allStreamsMux.RLock()
		if ws.allStreams[stream] {
			// 只恢复仍然活跃的订阅
			ws.addToBatch(stream)
		}
		ws.allStreamsMux.RUnlock()
	}
}

func (ws *WebSocket) getWebSocketURL() string {
	if ws.exchange != nil && ws.exchange.endpoints != nil {
		if wsURL, ok := ws.exchange.endpoints["websocket"]; ok {
			return wsURL
		}
	}

	if ws.exchange != nil && ws.exchange.config != nil {
		return ws.exchange.config.GetWebSocketURL()
	}

	return "wss://stream.binance.com:9443/ws"
}

// SetReconnectHandler 设置重连事件处理器
func (ws *WebSocket) SetReconnectHandler(handler func(int, error)) {
	ws.reconnectHandler = handler
}

// SetPublishFunc 设置数据发布函数
func (ws *WebSocket) SetPublishFunc(publishFunc func(types.MetaData, interface{}) error) {
	ws.publishFunc = publishFunc
}

// handleReconnectEvent 处理重连事件
func (ws *WebSocket) handleReconnectEvent(attempt int, err error) {
	if ws.reconnectHandler != nil {
		ws.reconnectHandler(attempt, err)
	}
}
