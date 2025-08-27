package websocket

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

var upgrades = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// 在生产环境中应该检查Origin
		return true
	},
}

// WebSocketManager WebSocket管理器
type WebSocketManager struct {
	hub *Hub
}

// NewWebSocketManager 创建WebSocket管理器
func NewWebSocketManager() *WebSocketManager {
	return &WebSocketManager{
		hub: NewHub(),
	}
}

// Start 启动WebSocket管理器
func (wsm *WebSocketManager) Start() {
	go wsm.hub.Run()
	logrus.Info("WebSocket管理器已启动")
}

// HandleWebSocket 处理WebSocket连接
func (wsm *WebSocketManager) HandleWebSocket(c *gin.Context) {
	// 升级HTTP连接为WebSocket
	conn, err := upgrades.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logrus.Errorf("WebSocket升级失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "WebSocket升级失败",
			"details": err.Error(),
		})
		return
	}

	// 生成客户端ID
	clientID := fmt.Sprintf("client_%d_%s", time.Now().UnixNano(), c.ClientIP())

	// 创建客户端
	client := NewClient(wsm.hub, conn, clientID)

	// 注册客户端
	wsm.hub.register <- client

	// 启动客户端
	client.StartClient()

	logrus.WithFields(logrus.Fields{
		"clientId":   clientID,
		"remoteAddr": c.Request.RemoteAddr,
		"userAgent":  c.Request.UserAgent(),
	}).Info("WebSocket连接已建立")
}

// GetStats 获取WebSocket统计信息
func (wsm *WebSocketManager) GetStats(c *gin.Context) {
	stats := wsm.hub.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   stats,
	})
}

// GetHub 获取Hub实例
func (wsm *WebSocketManager) GetHub() *Hub {
	return wsm.hub
}

// BroadcastEstimates 广播价格预估数据
func (wsm *WebSocketManager) BroadcastEstimates(data interface{}) {
	wsm.hub.BroadcastToSubscribers(DataTypeEstimates, data)
}

// BroadcastAccount 广播账户数据 持仓
func (wsm *WebSocketManager) BroadcastAccount(data interface{}) {
	wsm.hub.BroadcastToSubscribers(DataTypeAccount, data)
}

// BroadcastPrices 广播价格数据
func (wsm *WebSocketManager) BroadcastPrices(data interface{}) {
	wsm.hub.BroadcastToSubscribers(DataTypePrices, data)
}

// BroadcastBalances 广播余额数据
func (wsm *WebSocketManager) BroadcastBalances(data interface{}) {
	wsm.hub.BroadcastToSubscribers(DataTypeBalances, data)
}

// BroadcastEvent 广播事件消息
func (wsm *WebSocketManager) BroadcastEvent(data interface{}) {
	wsm.hub.BroadcastToSubscribers(DataTypeEvents, data)
}
