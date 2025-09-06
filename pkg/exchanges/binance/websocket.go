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

	"github.com/sirupsen/logrus"
)

// WebSocketConfig WebSocket配置
type WebSocketConfig struct {
	MaxConnections         int           `json:"maxConnections"`         // 最大连接数
	StreamsPerConnection   int           `json:"streamsPerConnection"`   // 每个连接的最大流数
	MaxReconnectAttempts   int           `json:"maxReconnectAttempts"`   // 最大重连次数
	BatchSize              int           `json:"batchSize"`              // 批量大小
	BatchInterval          time.Duration `json:"batchInterval"`          // 批量间隔
	HealthCheckInterval    time.Duration `json:"healthCheckInterval"`    // 健康检查间隔
	DataProcessConcurrency int           `json:"dataProcessConcurrency"` // 数据处理并发数
}

// DefaultWebSocketConfig 默认配置
func DefaultWebSocketConfig() *WebSocketConfig {
	return &WebSocketConfig{
		MaxConnections:         5,                      // 减少连接数，Mark Price 单连接即可处理
		StreamsPerConnection:   200,                    // 增加单连接流数量，减少连接管理开销
		MaxReconnectAttempts:   3,                      // 减少重连次数，快速失败
		BatchSize:              20,                     // 进一步减小批量，适合Mark Price订阅
		BatchInterval:          200 * time.Millisecond, // 适度增加间隔，符合Binance频率限制
		HealthCheckInterval:    15 * time.Second,       // 缩短健康检查间隔
		DataProcessConcurrency: 20,                     // 增加并发数，适应更多币种
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
//
// Mark Price 订阅示例:
//
//	ws.SubscribeMarkPrice()                              // 订阅全市场所有币种
//	ws.SubscribeMarkPrice("BTCUSDT", "ETHUSDT")         // 订阅特定币种
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

// SubscribeMarkPrice 订阅标记价格
func (ws *WebSocket) SubscribeMarkPrice() error {
	if atomic.LoadInt32(&ws.isRunning) == 0 {
		return fmt.Errorf("websocket not running")
	}

	// 直接订阅全市场流
	streamName := StreamMarkPriceArray1s

	ws.allStreamsMux.Lock()
	ws.allStreams[streamName] = true
	ws.allStreamsMux.Unlock()

	conn := ws.selectBestConnection()
	if conn == nil {
		return fmt.Errorf("no connection available")
	}

	subscribeMsg := map[string]interface{}{
		FieldMethod: MethodSubscribe,
		FieldParams: []string{streamName},
		FieldId:     time.Now().UnixNano(),
	}

	if err := ws.msgRateLimiter.Wait(ws.ctx); err != nil {
		return err
	}

	if err := conn.ws.SendMessage(subscribeMsg); err != nil {
		return fmt.Errorf("failed to subscribe global mark price: %w", err)
	}

	atomic.AddInt32(&conn.streamCount, 1)
	conn.lastUsed = time.Now()

	conn.streamsMux.Lock()
	conn.streams[streamName] = true
	conn.streamsMux.Unlock()

	logrus.Infof("成功订阅全市场标记价格流: %s", streamName)
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

	// 检查消息格式：数组还是对象
	if len(data) > 0 && data[0] == '[' {
		// 数组格式的消息
		return ws.handleArrayMessage(data)
	} else {
		// 对象格式的消息
		return ws.handleObjectMessage(data)
	}
}

// handleArrayMessage 处理数组格式的消息
func (ws *WebSocket) handleArrayMessage(data []byte) error {
	var arrData []interface{}
	if err := json.Unmarshal(data, &arrData); err != nil {
		atomic.AddInt64(&ws.errorCount, 1)
		logrus.Errorf("解析数组格式消息失败, 数据长度: %d, 错误: %v", len(data), err)
		return fmt.Errorf("parse array message failed: %w", err)
	}

	return ws.handleMarkPriceArray(arrData)
}

// handleObjectMessage 处理对象格式的消息
func (ws *WebSocket) handleObjectMessage(data []byte) error {
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		atomic.AddInt64(&ws.errorCount, 1)
		logrus.Errorf("解析对象格式消息失败, 数据长度: %d, 错误: %v", len(data), err)
		return fmt.Errorf("parse object message failed: %w", err)
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

// handleMarkPriceArray 处理标记价格数组数据
func (ws *WebSocket) handleMarkPriceArray(arrData []interface{}) error {
	if ws.publishFunc == nil {
		return nil
	}

	dataCount := len(arrData)
	if dataCount == 0 {
		return nil
	}

	// 动态调整并发数：数据量小时减少并发，避免开销
	maxConcurrency := ws.config.DataProcessConcurrency
	if dataCount < 100 {
		if dataCount/10+1 < maxConcurrency {
			maxConcurrency = dataCount/10 + 1
		}
	}

	semaphore := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	// 使用对象池减少内存分配
	validDataPool := make([]map[string]interface{}, 0, dataCount)

	// 预筛选有效数据，减少goroutine创建
	for i := range arrData {
		if priceObj, ok := arrData[i].(map[string]interface{}); ok {
			if symbol, exists := priceObj[FieldSymbol]; exists && symbol != "" {
				validDataPool = append(validDataPool, priceObj)
			}
		}
	}

	validCount := len(validDataPool)
	if validCount == 0 {
		return nil
	}

	// 批量处理，减少goroutine数量
	batchSize := 1
	if validCount/maxConcurrency > batchSize {
		batchSize = validCount / maxConcurrency
	}

	for i := 0; i < validCount; i += batchSize {
		end := i + batchSize
		if end > validCount {
			end = validCount
		}
		batch := validDataPool[i:end]

		wg.Add(1)
		go func(dataBatch []map[string]interface{}) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 批量处理数据
			for _, priceData := range dataBatch {
				markPrice := ws.parseMarkPriceSingle(priceData)
				if markPrice == nil {
					continue
				}

				if err := ws.publishWithMetaData(markPrice.Symbol, "markPrice", StreamMarkPriceArray1s, markPrice.TimeStamp, markPrice); err != nil {
					logrus.Errorf("发布标记价格数据失败 %s: %v", markPrice.Symbol, err)
				}
			}
		}(batch)
	}

	// 异步等待完成，避免阻塞
	go func() {
		start := time.Now()
		wg.Wait()
		duration := time.Since(start)

		// 性能监控（每500次记录一次，减少日志量）
		if atomic.AddInt64(&ws.msgCount, 1)%500 == 0 {
			logrus.Debugf("批量处理 %d 个币种数据，耗时: %v, 并发数: %d, 批量大小: %d",
				validCount, duration, maxConcurrency, batchSize)
		}
	}()

	return nil
}

// publishWithMetaData
func (ws *WebSocket) publishWithMetaData(marketID, dataType, stream string, timestamp int64, data interface{}) error {
	if ws.publishFunc == nil {
		return nil
	}

	metaData := types.MetaData{
		Exchange:  "binance",
		Market:    ws.getMarketType(),
		MarketID:  marketID,
		DataType:  dataType,
		Stream:    stream,
		Timestamp: timestamp,
	}

	return ws.publishFunc(metaData, data)
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

	// 构造并发布数据
	timestamp := ws.extractTimestamp(msg)
	dataType := ws.convertEventTypeToDataType(eventType)

	if eventType == EventTypeKline && streamName != "" {
		// K线数据需要特殊处理Timeframe
		metaData := types.MetaData{
			Exchange:  "binance",
			Market:    ws.getMarketType(),
			MarketID:  symbol,
			DataType:  dataType,
			Stream:    streamName,
			Timestamp: timestamp,
			Timeframe: ws.extractTimeframe(streamName),
		}
		return ws.publishFunc(metaData, parsedData)
	}

	return ws.publishWithMetaData(symbol, dataType, streamName, timestamp, parsedData)
}

// parseMarkPriceSingle 解析单个标记价格对象
func (ws *WebSocket) parseMarkPriceSingle(priceObj map[string]interface{}) *types.WatchMarkPrice {
	symbol := strings.ToUpper(ws.SafeString(priceObj, FieldSymbol, ""))
	if symbol == "" {
		return nil
	}

	return &types.WatchMarkPrice{
		Symbol:      symbol,
		TimeStamp:   ws.SafeInt(priceObj, FieldEventTime, time.Now().UnixMilli()),
		MarkPrice:   ws.SafeFloat(priceObj, FieldMarkPrice, 0),
		IndexPrice:  ws.SafeFloat(priceObj, FieldIndexPrice, 0),
		FundingRate: ws.SafeFloat(priceObj, FieldFundingRate, 0),
		FundingTime: ws.SafeInt(priceObj, FieldFundingTime, 0),
	}
}

// parseStreamInfo 解析流信息
func (ws *WebSocket) parseStreamInfo(streamName string) (eventType, symbol string) {
	// 处理标记价格数组流
	if streamName == StreamMarkPriceArray1s {
		return EventTypeMarkPrice, "ALL" // 使用特殊symbol标识所有币种
	}

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
			// 只保留数量大于0的价格档位
			if quantity > 0 {
				bids = append(bids, []float64{price, quantity})
			}
		}
	}

	for i := range asksData {
		askData := asksData[i]
		if askArray, ok := askData.([]interface{}); ok && len(askArray) >= 2 {
			price, _ := strconv.ParseFloat(askArray[0].(string), 64)
			quantity, _ := strconv.ParseFloat(askArray[1].(string), 64)
			// 只保留数量大于0的价格档位
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
	markPrice := ws.parseMarkPriceSingle(msg)
	if markPrice != nil {
		// 对于单个标记价格流，使用extractTimestamp而不是FieldEventTime
		markPrice.TimeStamp = ws.extractTimestamp(msg)
	}
	return markPrice
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
	default:
		return ""
	}
}

func (ws *WebSocket) getMarketType() string {
	return ws.exchange.marketType
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
		interval: 200 * time.Millisecond, // 符合Binance每秒5个消息的限制 (200ms = 5/s)
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
