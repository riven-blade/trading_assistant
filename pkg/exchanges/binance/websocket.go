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

	"trading_assistant/pkg/exchanges"

	"github.com/sirupsen/logrus"
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
		StreamsPerConnection: 800,
		MaxReconnectAttempts: 5,
		BatchSize:            200,
		BatchInterval:        50 * time.Millisecond,
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
	publishFunc func(exchanges.MetaData, interface{}) error

	// 用户数据流相关
	userDataListenKey   string
	userDataPublishFunc func(exchanges.MetaData, interface{}) error
	userDataConnection  *exchanges.WebSocketConnection
	userDataActive      int32 // 0=未激活, 1=已激活
	userDataStopCh      chan struct{}
	userDataWg          sync.WaitGroup

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

	// 直接添加到批量队列
	ws.addToBatch(streamName)
	return nil
}

// UnsubscribeStream 取消订阅数据流
func (ws *WebSocket) UnsubscribeStream(streamName string) error {
	if atomic.LoadInt32(&ws.isRunning) == 0 {
		return fmt.Errorf("websocket not running")
	}

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

	// 对于用户数据流事件，symbol可能为空或从其他字段获取，这是正常的
	if eventType == "" {
		return nil
	}

	// 非用户数据流事件需要symbol
	if symbol == "" && !ws.isUserDataEvent(eventType) {
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
	case EventTypeAccountUpdate:
		parsedData = ws.parseAccountUpdate(msg)
	case EventTypeOrderTradeUpdate:
		parsedData = ws.parseOrderTradeUpdate(msg)
	default:
		return nil
	}

	if parsedData == nil {
		return nil
	}

	// 构造MetaData
	metaData := exchanges.MetaData{
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
func (ws *WebSocket) parseDepthUpdate(msg map[string]interface{}) *exchanges.WatchOrderBook {
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

	return &exchanges.WatchOrderBook{
		Symbol:    symbol,
		TimeStamp: ws.extractTimestamp(msg),
		Bids:      bids,
		Asks:      asks,
		Nonce:     ws.SafeInt(msg, FieldUpdateId, 0),
	}
}

// parseKline 解析K线数据
func (ws *WebSocket) parseKline(msg map[string]interface{}) *exchanges.Kline {
	klineData, ok := msg[FieldKlineData].(map[string]interface{})
	if !ok {
		return nil
	}

	symbol := strings.ToUpper(ws.SafeString(klineData, FieldSymbol, ""))
	if symbol == "" {
		return nil
	}

	return &exchanges.Kline{
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
func (ws *WebSocket) parseBookTicker(msg map[string]interface{}) *exchanges.WatchBookTicker {
	symbol := strings.ToUpper(ws.SafeString(msg, FieldSymbol, ""))
	if symbol == "" {
		return nil
	}

	return &exchanges.WatchBookTicker{
		Symbol:      symbol,
		TimeStamp:   ws.extractTimestamp(msg),
		BidPrice:    ws.SafeFloat(msg, FieldBidPrice, 0),
		BidQuantity: ws.SafeFloat(msg, FieldBidQty, 0),
		AskPrice:    ws.SafeFloat(msg, FieldAskPrice, 0),
		AskQuantity: ws.SafeFloat(msg, FieldAskQty, 0),
	}
}

// parseMarkPrice 解析标记价格
func (ws *WebSocket) parseMarkPrice(msg map[string]interface{}) *exchanges.WatchMarkPrice {
	symbol := strings.ToUpper(ws.SafeString(msg, FieldSymbol, ""))
	if symbol == "" {
		return nil
	}

	return &exchanges.WatchMarkPrice{
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
func (ws *WebSocket) parseAccountUpdate(msg map[string]interface{}) *exchanges.WatchAccountUpdate {
	result := &exchanges.WatchAccountUpdate{
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
					balance := exchanges.WatchBalanceUpdate{
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
					position := exchanges.WatchPositionUpdate{
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
func (ws *WebSocket) parseOrderTradeUpdate(msg map[string]interface{}) *exchanges.WatchOrderUpdate {
	// 获取订单数据
	var orderData map[string]interface{}
	if data, ok := msg["o"].(map[string]interface{}); ok {
		orderData = data
	} else {
		orderData = msg
	}

	return &exchanges.WatchOrderUpdate{
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
		interval: 200 * time.Millisecond,
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

	// 更新连接的流计数
	atomic.AddInt32(&conn.streamCount, int32(len(streams)))
	conn.lastUsed = time.Now()
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

	// 如果没有可用连接，尝试创建新连接
	if bestConn == nil || minLoad > int32(ws.config.StreamsPerConnection*3/4) {
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
	}

	wsInst.SetHandler(func(data []byte) error {
		return ws.handleMessage(data, conn)
	})

	wsInst.SetErrorHandler(func(err error) {
		atomic.StoreInt32(&conn.isHealthy, 0)
	})

	ws.connections = append(ws.connections, conn)
	return nil
}

func (ws *WebSocket) closeConnection(conn *WSConnection) {
	if conn.ws != nil {
		conn.ws.Close()
	}
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

	for i := len(ws.connections) - 1; i >= 0; i-- {
		conn := ws.connections[i]
		if atomic.LoadInt32(&conn.isHealthy) == 0 || !conn.ws.IsConnected() {
			ws.closeConnection(conn)
			ws.connections = append(ws.connections[:i], ws.connections[i+1:]...)
		}
	}

	// 如果没有健康连接，创建新连接
	if len(ws.connections) == 0 {
		ws.createConnectionUnsafe()
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

// ========== 用户数据流管理 ==========

// SubscribeUserData 订阅用户数据流
func (ws *WebSocket) SubscribeUserData(listenKey string, publishFunc func(exchanges.MetaData, interface{}) error) error {
	if atomic.LoadInt32(&ws.userDataActive) == 1 {
		return fmt.Errorf("user data stream already active")
	}

	ws.userDataListenKey = listenKey
	ws.userDataPublishFunc = publishFunc
	ws.userDataStopCh = make(chan struct{})

	// 启动用户数据流连接
	ws.userDataWg.Add(1)
	go ws.startUserDataStream()

	// 启动listenKey刷新
	ws.userDataWg.Add(1)
	go ws.keepaliveListenKey()

	atomic.StoreInt32(&ws.userDataActive, 1)
	return nil
}

// UnsubscribeUserData 取消订阅用户数据流
func (ws *WebSocket) UnsubscribeUserData() error {
	if atomic.LoadInt32(&ws.userDataActive) == 0 {
		return nil
	}

	atomic.StoreInt32(&ws.userDataActive, 0)
	close(ws.userDataStopCh)
	ws.userDataWg.Wait()

	// 关闭连接
	if ws.userDataConnection != nil {
		ws.userDataConnection.Close()
		ws.userDataConnection = nil
	}

	// 关闭listenKey
	if ws.userDataListenKey != "" && ws.exchange != nil {
		ws.exchange.CloseListenKey(ws.userDataListenKey)
		ws.userDataListenKey = ""
	}

	return nil
}

// startUserDataStream 启动用户数据流连接
func (ws *WebSocket) startUserDataStream() {
	defer ws.userDataWg.Done()

	for {
		select {
		case <-ws.userDataStopCh:
			return
		default:
			if err := ws.connectUserDataStream(); err != nil {
				logrus.Errorf("用户数据流连接失败: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}

			// 连接成功，等待连接关闭或停止信号
			select {
			case <-ws.userDataStopCh:
				return
			case <-time.After(24 * time.Hour): // 24小时后重连
				logrus.Info("用户数据流24小时重连")
				if ws.userDataConnection != nil {
					ws.userDataConnection.Close()
				}
			}
		}
	}
}

// connectUserDataStream 连接用户数据流
func (ws *WebSocket) connectUserDataStream() error {
	if ws.userDataListenKey == "" {
		return fmt.Errorf("listenKey为空")
	}

	// 构建WebSocket URL - 用户数据流URL格式: wss://fstream.binance.com/ws/{listenKey}
	baseURL := ws.getUserDataWebSocketURL()
	url := fmt.Sprintf("%s/%s", baseURL, ws.userDataListenKey)

	// 创建WebSocket连接
	conn, err := exchanges.NewWebSocketConnection(context.Background(), url, 5) // 最大重连5次
	if err != nil {
		logrus.Errorf("用户数据流连接失败，URL: %s, 错误: %v", url, err)
		return fmt.Errorf("创建用户数据流连接失败: %w", err)
	}

	// 设置消息处理器
	conn.SetHandler(func(data []byte) error {
		return ws.handleUserDataMessage(data)
	})

	// 设置错误处理器
	conn.SetErrorHandler(func(err error) {
		logrus.Errorf("用户数据流连接错误: %v", err)
	})

	// 连接已经在NewWebSocketConnection中启动

	ws.userDataConnection = conn
	logrus.Info("用户数据流连接成功")
	return nil
}

// handleUserDataMessage 处理用户数据流消息
func (ws *WebSocket) handleUserDataMessage(data []byte) error {
	if ws.userDataPublishFunc == nil {
		return nil
	}

	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		return err
	}

	// 获取事件类型
	eventType, _ := msg[FieldEventType].(string)
	if eventType == "" {
		return nil
	}

	// 根据事件类型解析数据
	var parsedData interface{}
	switch eventType {
	case EventTypeAccountUpdate:
		parsedData = ws.parseAccountUpdate(msg)
	case EventTypeOrderTradeUpdate:
		parsedData = ws.parseOrderTradeUpdate(msg)
	default:
		return nil
	}

	if parsedData == nil {
		return nil
	}

	// 构造MetaData
	metaData := exchanges.MetaData{
		Exchange:  "binance",
		Market:    ws.getMarketType(),
		DataType:  ws.convertEventTypeToDataType(eventType),
		Timestamp: ws.extractTimestamp(msg),
	}

	// 调用发布函数
	return ws.userDataPublishFunc(metaData, parsedData)
}

// keepaliveListenKey 保持listenKey活跃
func (ws *WebSocket) keepaliveListenKey() {
	defer ws.userDataWg.Done()

	ticker := time.NewTicker(30 * time.Minute) // 每30分钟刷新一次
	defer ticker.Stop()

	for {
		select {
		case <-ws.userDataStopCh:
			return
		case <-ticker.C:
			if ws.userDataListenKey != "" && ws.exchange != nil {
				if err := ws.exchange.KeepaliveListenKey(ws.userDataListenKey); err != nil {
					logrus.Errorf("刷新listenKey失败: %v", err)
				} else {
					logrus.Debug("listenKey刷新成功")
				}
			}
		}
	}
}

// getUserDataWebSocketURL 获取用户数据流WebSocket URL
func (ws *WebSocket) getUserDataWebSocketURL() string {
	// 用户数据流使用标准的WebSocket URL（包含/ws路径）
	if ws.exchange != nil && ws.exchange.config != nil {
		return ws.exchange.config.GetWebSocketURL()
	}

	return "wss://fstream.binance.com/ws"
}
