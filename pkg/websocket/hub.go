package websocket

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"
	"trading_assistant/models"
	"trading_assistant/pkg/redis"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// Hub 维护活跃的客户端集合并向客户端广播消息
type Hub struct {
	// 注册的客户端
	clients map[*Client]bool

	// 来自客户端的入站消息
	broadcast chan []byte

	// 来自客户端的注册请求
	register chan *Client

	// 来自客户端的注销请求
	unregister chan *Client

	// 客户端管理
	clientsMutex sync.RWMutex

	// 订阅管理
	subscriptions map[string]map[*Client]bool // dataType -> clients
	subsMutex     sync.RWMutex
}

// Client 表示单个WebSocket客户端
type Client struct {
	hub *Hub

	// WebSocket连接
	conn *websocket.Conn

	// 出站消息的缓冲通道
	send chan []byte

	// 客户端唯一标识
	id string

	// 客户端订阅的数据类型
	subscriptions map[string]bool
	subsMutex     sync.RWMutex

	// 连接时间
	connectedAt time.Time

	// 最后活跃时间
	lastActivity time.Time

	// 客户端状态
	closed     bool
	closeMutex sync.RWMutex
}

// Message 表示WebSocket消息格式
type Message struct {
	Type      string      `json:"type"`      // message, subscribe, unsubscribe, ping, pong, error
	DataType  string      `json:"dataType"`  // estimates, account, prices, balances
	Data      interface{} `json:"data"`      // 实际数据
	Timestamp int64       `json:"timestamp"` // 时间戳
	ClientID  string      `json:"clientId"`  // 客户端ID（仅用于调试）
}

// ErrorMessage 错误消息格式
type ErrorMessage struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	Details string `json:"details,omitempty"`
}

const (
	// 消息类型
	MessageTypeMessage     = "message"
	MessageTypeSubscribe   = "subscribe"
	MessageTypeUnsubscribe = "unsubscribe"
	MessageTypePing        = "ping"
	MessageTypePong        = "pong"
	MessageTypeError       = "error"

	// 数据类型
	DataTypeEstimates = "estimates"
	DataTypeAccount   = "account"
	DataTypePrices    = "prices"
	DataTypeBalances  = "balances"
	DataTypeEvents    = "events"

	// 时间常量
	writeWait      = 10 * time.Second    // 写入等待时间
	pongWait       = 60 * time.Second    // Pong等待时间
	pingPeriod     = (pongWait * 9) / 10 // Ping发送周期
	maxMessageSize = 512                 // 最大消息大小
)

// NewHub 创建新的Hub
func NewHub() *Hub {
	return &Hub{
		broadcast:     make(chan []byte),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		clients:       make(map[*Client]bool),
		subscriptions: make(map[string]map[*Client]bool),
	}
}

// Run 启动Hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clientsMutex.Lock()
			h.clients[client] = true
			h.clientsMutex.Unlock()
			logrus.WithField("clientId", client.id).Info("客户端已连接")

			// 发送欢迎消息
			welcome := Message{
				Type:      MessageTypeMessage,
				DataType:  "system",
				Data:      map[string]string{"status": "connected", "clientId": client.id},
				Timestamp: time.Now().UnixMilli(),
				ClientID:  client.id,
			}
			if data, err := json.Marshal(welcome); err == nil {
				select {
				case client.send <- data:
				default:
					client.safeClose()
					delete(h.clients, client)
				}
			}

		case client := <-h.unregister:
			h.clientsMutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.safeClose()

				// 从所有订阅中移除客户端
				h.subsMutex.Lock()
				for i := range h.subscriptions {
					clients := h.subscriptions[i]
					delete(clients, client)
				}
				h.subsMutex.Unlock()

				logrus.WithField("clientId", client.id).Info("客户端已断开")
			}
			h.clientsMutex.Unlock()

		case message := <-h.broadcast:
			h.clientsMutex.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					client.safeClose()
					delete(h.clients, client)
				}
			}
			h.clientsMutex.RUnlock()
		}
	}
}

// GetStats 获取Hub统计信息
func (h *Hub) GetStats() map[string]interface{} {
	h.clientsMutex.RLock()
	clientCount := len(h.clients)
	h.clientsMutex.RUnlock()

	h.subsMutex.RLock()
	subscriptionStats := make(map[string]int)
	for dataType, clients := range h.subscriptions {
		subscriptionStats[dataType] = len(clients)
	}
	h.subsMutex.RUnlock()

	return map[string]interface{}{
		"connectedClients": clientCount,
		"subscriptions":    subscriptionStats,
		"startTime":        time.Now().Format("2006-01-02 15:04:05"),
	}
}

// BroadcastToSubscribers 向订阅指定数据类型的客户端广播消息
func (h *Hub) BroadcastToSubscribers(dataType string, data interface{}) {
	message := Message{
		Type:      MessageTypeMessage,
		DataType:  dataType,
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
	}

	messageData, err := json.Marshal(message)
	if err != nil {
		logrus.Errorf("序列化广播消息失败: %v", err)
		return
	}

	h.subsMutex.RLock()
	subscribers, exists := h.subscriptions[dataType]
	if !exists || len(subscribers) == 0 {
		h.subsMutex.RUnlock()
		logrus.Debugf("没有订阅 %s 的客户端", dataType)
		return
	}

	// 创建订阅者列表的副本以避免并发问题
	clientList := make([]*Client, 0, len(subscribers))
	for client := range subscribers {
		clientList = append(clientList, client)
	}
	h.subsMutex.RUnlock()

	// 发送给所有订阅者
	successCount := 0
	failedClients := make([]*Client, 0)

	for i := range clientList {
		client := clientList[i]

		// 检查客户端是否已关闭
		if client.isClosed() {
			failedClients = append(failedClients, client)
			continue
		}

		// 使用defer + recover来捕获panic
		func() {
			defer func() {
				if r := recover(); r != nil {
					logrus.Warnf("向客户端 %s 发送数据时发生panic: %v", client.id, r)
					failedClients = append(failedClients, client)
				}
			}()

			select {
			case client.send <- messageData:
				successCount++
			default:
				// 客户端发送缓冲区已满，标记为失败
				failedClients = append(failedClients, client)
			}
		}()
	}

	// 清理失败的客户端
	for i := range failedClients {
		client := failedClients[i]
		h.unregisterClient(client)
	}

	logrus.Debugf("向 %d 个订阅 %s 的客户端发送数据，成功 %d 个，失败 %d 个",
		len(clientList), dataType, successCount, len(failedClients))
}

// Subscribe 客户端订阅数据类型
func (h *Hub) Subscribe(client *Client, dataType string) {
	h.subsMutex.Lock()
	defer h.subsMutex.Unlock()

	if h.subscriptions[dataType] == nil {
		h.subscriptions[dataType] = make(map[*Client]bool)
	}
	h.subscriptions[dataType][client] = true

	client.subsMutex.Lock()
	client.subscriptions[dataType] = true
	client.subsMutex.Unlock()

	logrus.WithFields(logrus.Fields{
		"clientId": client.id,
		"dataType": dataType,
	}).Info("客户端订阅数据类型")

	// 立即推送该数据类型的当前数据
	go h.sendInitialDataForType(client, dataType)
}

// Unsubscribe 客户端取消订阅数据类型
func (h *Hub) Unsubscribe(client *Client, dataType string) {
	h.subsMutex.Lock()
	defer h.subsMutex.Unlock()

	if clients, exists := h.subscriptions[dataType]; exists {
		delete(clients, client)
	}

	client.subsMutex.Lock()
	delete(client.subscriptions, dataType)
	client.subsMutex.Unlock()

	logrus.WithFields(logrus.Fields{
		"clientId": client.id,
		"dataType": dataType,
	}).Info("客户端取消订阅数据类型")
}

// unregisterClient 注销客户端
func (h *Hub) unregisterClient(client *Client) {
	// 检查客户端是否已经关闭
	if client.isClosed() {
		return
	}

	select {
	case h.unregister <- client:
	default:
		// 注销通道已满，直接删除
		h.clientsMutex.Lock()
		if _, ok := h.clients[client]; ok {
			delete(h.clients, client)
			client.safeClose()
		}
		h.clientsMutex.Unlock()
	}
}

// isClosed 检查客户端是否已经关闭
func (c *Client) isClosed() bool {
	c.closeMutex.RLock()
	defer c.closeMutex.RUnlock()
	return c.closed
}

// safeClose 安全关闭客户端
func (c *Client) safeClose() {
	c.closeMutex.Lock()
	defer c.closeMutex.Unlock()

	if !c.closed {
		c.closed = true
		close(c.send)
	}
}

// NewClient 创建新的客户端
func NewClient(hub *Hub, conn *websocket.Conn, id string) *Client {
	return &Client{
		hub:           hub,
		conn:          conn,
		send:          make(chan []byte, 256),
		id:            id,
		subscriptions: make(map[string]bool),
		connectedAt:   time.Now(),
		lastActivity:  time.Now(),
	}
}

// readPump 处理来自WebSocket连接的读取操作
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		err := c.conn.Close()
		if err != nil {
			return
		}
	}()

	c.conn.SetReadLimit(maxMessageSize)
	err := c.conn.SetReadDeadline(time.Now().Add(pongWait))
	if err != nil {
		return
	}
	c.conn.SetPongHandler(func(string) error {
		err = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		if err != nil {
			return err
		}
		c.lastActivity = time.Now()
		return nil
	})

	for {
		_, messageData, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logrus.Errorf("WebSocket错误: %v", err)
			}
			break
		}

		c.lastActivity = time.Now()

		// 解析消息
		var msg Message
		if err := json.Unmarshal(messageData, &msg); err != nil {
			logrus.Errorf("解析WebSocket消息失败: %v", err)
			c.sendError("INVALID_MESSAGE", "消息格式错误", fmt.Sprintf("解析失败: %v", err))
			continue
		}

		// 处理消息
		c.handleMessage(&msg)
	}
}

// writePump 处理向WebSocket连接的写入操作
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// 添加队列中的其他消息
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage 处理客户端消息
func (c *Client) handleMessage(msg *Message) {
	switch msg.Type {
	case MessageTypeSubscribe:
		if msg.DataType == "" {
			c.sendError("INVALID_DATATYPE", "订阅失败", "dataType不能为空")
			return
		}

		// 验证数据类型
		if !c.isValidDataType(msg.DataType) {
			c.sendError("INVALID_DATATYPE", "订阅失败", fmt.Sprintf("不支持的数据类型: %s", msg.DataType))
			return
		}

		c.hub.Subscribe(c, msg.DataType)

		// 发送订阅确认
		response := Message{
			Type:      MessageTypeMessage,
			DataType:  "system",
			Data:      map[string]string{"action": "subscribed", "dataType": msg.DataType},
			Timestamp: time.Now().UnixMilli(),
			ClientID:  c.id,
		}
		c.sendMessage(&response)

	case MessageTypeUnsubscribe:
		if msg.DataType == "" {
			c.sendError("INVALID_DATATYPE", "取消订阅失败", "dataType不能为空")
			return
		}

		c.hub.Unsubscribe(c, msg.DataType)

		// 发送取消订阅确认
		response := Message{
			Type:      MessageTypeMessage,
			DataType:  "system",
			Data:      map[string]string{"action": "unsubscribed", "dataType": msg.DataType},
			Timestamp: time.Now().UnixMilli(),
			ClientID:  c.id,
		}
		c.sendMessage(&response)

	case MessageTypePing:
		// 响应ping
		pong := Message{
			Type:      MessageTypePong,
			DataType:  "system",
			Data:      map[string]string{"message": "pong"},
			Timestamp: time.Now().UnixMilli(),
			ClientID:  c.id,
		}
		c.sendMessage(&pong)

	default:
		c.sendError("UNKNOWN_MESSAGE_TYPE", "未知消息类型", fmt.Sprintf("不支持的消息类型: %s", msg.Type))
	}
}

// isValidDataType 验证数据类型是否有效
func (c *Client) isValidDataType(dataType string) bool {
	validTypes := []string{
		DataTypeEstimates,
		DataTypeAccount,
		DataTypePrices,
		DataTypeBalances,
		DataTypeEvents,
	}

	for _, validType := range validTypes {
		if dataType == validType {
			return true
		}
	}
	return false
}

// sendMessage 发送消息给客户端
func (c *Client) sendMessage(msg *Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		logrus.Errorf("序列化消息失败: %v", err)
		return
	}

	// 检查客户端是否已关闭
	if c.isClosed() {
		return
	}

	select {
	case c.send <- data:
	default:
		// 发送缓冲区已满，关闭连接
		c.safeClose()
	}
}

// sendError 发送错误消息给客户端
func (c *Client) sendError(code, message, details string) {
	errorMsg := Message{
		Type:     MessageTypeError,
		DataType: "system",
		Data: ErrorMessage{
			Error:   message,
			Code:    code,
			Details: details,
		},
		Timestamp: time.Now().UnixMilli(),
		ClientID:  c.id,
	}

	c.sendMessage(&errorMsg)
}

// StartClient 启动客户端的读写协程
func (c *Client) StartClient() {
	go c.writePump()
	go c.readPump()
}

// sendInitialDataForType 为新订阅的客户端发送初始数据
func (h *Hub) sendInitialDataForType(client *Client, dataType string) {
	var data interface{}
	var err error

	switch dataType {
	case DataTypePrices:
		// 获取当前价格数据
		data, err = h.getCurrentPricesData()
	case DataTypeAccount:
		// 获取当前账户数据（持仓）
		data, err = h.getCurrentAccountData()
	case DataTypeBalances:
		// 获取当前余额数据
		data, err = h.getCurrentBalancesData()
	case DataTypeEstimates:
		// 获取当前预估数据
		data, err = h.getCurrentEstimatesData()
	case DataTypeEvents:
		// 事件数据不需要初始推送
		return
	default:
		logrus.Warnf("未知的数据类型: %s", dataType)
		return
	}

	if err != nil {
		logrus.Errorf("获取 %s 初始数据失败: %v", dataType, err)
		return
	}

	if data == nil {
		logrus.Debugf("没有可用的 %s 初始数据", dataType)
		return
	}

	// 发送初始数据
	message := Message{
		Type:      MessageTypeMessage,
		DataType:  dataType,
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
		ClientID:  client.id,
	}

	messageData, err := json.Marshal(message)
	if err != nil {
		logrus.Errorf("序列化初始数据失败: %v", err)
		return
	}

	// 检查客户端是否已关闭
	if client.isClosed() {
		logrus.Debugf("客户端 %s 已关闭，跳过发送初始 %s 数据", client.id, dataType)
		return
	}

	select {
	case client.send <- messageData:
		logrus.Debugf("向客户端 %s 发送初始 %s 数据", client.id, dataType)
	default:
		logrus.Warnf("客户端 %s 发送缓冲区已满，无法发送初始 %s 数据", client.id, dataType)
	}
}

// getCurrentPricesData 获取当前价格数据
func (h *Hub) getCurrentPricesData() (interface{}, error) {
	// 获取选中的币种
	selectedSymbols, err := redis.GlobalRedisClient.GetSelectedCoinSymbols()
	if err != nil {
		return nil, fmt.Errorf("获取选中币种失败: %v", err)
	}

	pricesData := make(map[string]interface{})
	for i := range selectedSymbols {
		symbol := selectedSymbols[i]
		if markPrice, err := redis.GlobalRedisClient.GetMarkPrice(symbol); err == nil {
			// 从Redis获取coin数据来获取价格变化信息
			priceChange := 0.0
			priceChangePercent := 0.0

			if coin, coinErr := redis.GlobalRedisClient.GetCoin(symbol); coinErr == nil {
				if change, parseErr := strconv.ParseFloat(coin.PriceChange, 64); parseErr == nil {
					priceChange = change
				}
				if changePercent, parseErr := strconv.ParseFloat(coin.PriceChangePercent, 64); parseErr == nil {
					priceChangePercent = changePercent
				}
			}

			pricesData[symbol] = map[string]interface{}{
				"symbol":             symbol,
				"markPrice":          markPrice.MarkPrice,
				"indexPrice":         markPrice.IndexPrice,
				"fundingRate":        markPrice.FundingRate,
				"fundingTime":        markPrice.FundingTime,
				"updateTime":         markPrice.TimeStamp,
				"priceChange":        priceChange,
				"priceChangePercent": priceChangePercent,
			}
		}
	}

	logrus.Debugf("获取当前价格数据成功，包含 %d 个币种", len(pricesData))
	return pricesData, nil
}

// getCurrentAccountData 获取当前账户数据
func (h *Hub) getCurrentAccountData() (interface{}, error) {
	// 从Redis获取当前持仓数据
	positions, err := redis.GlobalRedisClient.GetAllPositions()
	if err != nil {
		logrus.Errorf("获取持仓数据失败: %v", err)
		return nil, err
	}

	// 计算总盈亏
	totalPnl := 0.0
	for i := range positions {
		position := positions[i]
		totalPnl += position.UnrealizedPnl
	}

	accountData := map[string]interface{}{
		"positions":     positions,
		"totalPnl":      totalPnl,
		"positionCount": len(positions),
		"lastUpdate":    time.Now().Unix(),
	}

	logrus.Debugf("获取当前账户数据成功，包含 %d 个持仓", len(positions))
	return accountData, nil
}

// getCurrentBalancesData 获取当前余额数据
func (h *Hub) getCurrentBalancesData() (interface{}, error) {
	// 从Redis获取当前余额数据
	var balances map[string]interface{}
	err := redis.GlobalRedisClient.GetBalancesRealtime(&balances)
	if err != nil {
		logrus.Debugf("未找到缓存的余额数据: %v", err)
		return nil, nil // 返回空数据而不是错误
	}

	logrus.Debugf("获取当前余额数据成功")
	return balances, nil
}

// getCurrentEstimatesData 获取当前预估数据
func (h *Hub) getCurrentEstimatesData() (interface{}, error) {
	// 从Redis获取所有预估数据
	estimates, err := redis.GlobalRedisClient.GetAllEstimates()
	if err != nil {
		logrus.Errorf("获取预估数据失败: %v", err)
		return nil, err
	}

	// 按币种分组监听状态的预估
	symbolEstimates := make(map[string][]interface{})

	for i := range estimates {
		estimate := estimates[i]
		// 只收集正在监听的预估
		if estimate.Status == models.EstimateStatusListening {
			if symbolEstimates[estimate.Symbol] == nil {
				symbolEstimates[estimate.Symbol] = make([]interface{}, 0)
			}
			symbolEstimates[estimate.Symbol] = append(symbolEstimates[estimate.Symbol], estimate)
		}
	}

	// 简化数据结构，只推送按币种分组的预估数据
	estimatesData := map[string]interface{}{
		"symbolEstimates": symbolEstimates, // 按币种分组的预估数据
		"lastUpdate":      time.Now().Unix(),
	}

	logrus.Debugf("获取当前预估数据成功，包含 %d 个币种", len(symbolEstimates))
	return estimatesData, nil
}
