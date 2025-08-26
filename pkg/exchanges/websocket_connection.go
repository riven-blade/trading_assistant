package exchanges

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ========== WebSocket 基础框架 ==========

type WebSocketConnection struct {
	conn           *websocket.Conn
	url            string
	isConnected    bool
	pingInterval   time.Duration // 心跳间隔，默认30秒
	autoReconnect  bool          // 是否自动重连
	maxReconnect   int           // 最大重连次数
	reconnectCount int           // 当前重连次数
	mutex          sync.RWMutex

	// 处理器函数
	messageHandler   func([]byte) error // 消息处理器
	errorHandler     func(error)        // 错误处理器
	reconnectHandler func(int, error)   // 重连处理器 (attempt, error)

	// 生命周期管理
	ctx    context.Context
	cancel context.CancelFunc
}

func NewWebSocketConnection(ctx context.Context, url string, maxReconnect int) (*WebSocketConnection, error) {
	wsConn := &WebSocketConnection{
		url:            url,
		isConnected:    false,
		pingInterval:   30 * time.Second,
		autoReconnect:  maxReconnect > 0, // 只有当maxReconnect > 0时才启用自动重连
		maxReconnect:   maxReconnect,
		reconnectCount: 0,
	}

	// 尝试连接
	if err := wsConn.connect(ctx); err != nil {
		return nil, err
	}

	return wsConn, nil
}

// connect 执行实际连接
func (ws *WebSocketConnection) connect(ctx context.Context) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, ws.url, nil)
	if err != nil {
		connErr := fmt.Errorf("failed to connect to %s: %w", ws.url, err)
		if ws.errorHandler != nil {
			ws.errorHandler(connErr)
		}
		return connErr
	}

	wsCtx, cancel := context.WithCancel(ctx)

	ws.mutex.Lock()
	ws.conn = conn
	ws.isConnected = true
	ws.ctx = wsCtx
	ws.cancel = cancel
	ws.mutex.Unlock()

	// 启动协程
	go ws.messageLoop()
	go ws.pingLoop()

	return nil
}

// reconnect 重连逻辑
func (ws *WebSocketConnection) reconnect() {
	if !ws.autoReconnect || ws.reconnectCount >= ws.maxReconnect {
		if ws.errorHandler != nil {
			ws.errorHandler(fmt.Errorf("max reconnect attempts reached (%d), giving up", ws.maxReconnect))
		}
		return
	}

	ws.reconnectCount++

	// 通知重连开始
	if ws.reconnectHandler != nil {
		ws.reconnectHandler(ws.reconnectCount, fmt.Errorf("WebSocket reconnecting, attempt %d/%d", ws.reconnectCount, ws.maxReconnect))
	}

	// 指数退避：2^attempt * 1秒，最大30秒
	backoff := time.Duration(1<<uint(ws.reconnectCount)) * time.Second
	if backoff > 30*time.Second {
		backoff = 30 * time.Second
	}

	time.Sleep(backoff)

	if err := ws.connect(ws.ctx); err != nil {
		if ws.errorHandler != nil {
			ws.errorHandler(fmt.Errorf("reconnect attempt %d/%d failed: %w",
				ws.reconnectCount, ws.maxReconnect, err))
		}
		// 通知重连失败
		if ws.reconnectHandler != nil {
			ws.reconnectHandler(ws.reconnectCount, fmt.Errorf("WebSocket reconnect failed, attempt %d/%d: %w", ws.reconnectCount, ws.maxReconnect, err))
		}
		go ws.reconnect() // 继续重连
	} else {
		// 通知重连成功
		if ws.reconnectHandler != nil {
			ws.reconnectHandler(ws.reconnectCount, nil) // error为nil表示重连成功
		}
		ws.reconnectCount = 0 // 重连成功，重置计数
	}
}

// messageLoop 消息处理循环
func (ws *WebSocketConnection) messageLoop() {
	defer func() {
		ws.mutex.Lock()
		ws.isConnected = false
		if ws.conn != nil {
			ws.conn.Close()
		}
		ws.mutex.Unlock()

		// 如果启用重连，则尝试重连
		if ws.autoReconnect && ws.reconnectCount < ws.maxReconnect {
			go ws.reconnect()
		}
	}()

	for {
		select {
		case <-ws.ctx.Done():
			return
		default:
			_, message, err := ws.conn.ReadMessage()
			if err != nil {
				if ws.errorHandler != nil {
					ws.errorHandler(err)
				}
				return
			}

			if ws.messageHandler != nil {
				if err := ws.messageHandler(message); err != nil && ws.errorHandler != nil {
					ws.errorHandler(err)
				}
			}
		}
	}
}

// SetHandler 设置消息处理器
func (ws *WebSocketConnection) SetHandler(handler func([]byte) error) {
	ws.messageHandler = handler
}

// SetErrorHandler 设置错误处理器
func (ws *WebSocketConnection) SetErrorHandler(handler func(error)) {
	ws.errorHandler = handler
}

// SetReconnectHandler 设置重连处理器
func (ws *WebSocketConnection) SetReconnectHandler(handler func(int, error)) {
	ws.reconnectHandler = handler
}

// ========== 便利方法 ==========

// IsConnected 检查连接状态
func (ws *WebSocketConnection) IsConnected() bool {
	ws.mutex.RLock()
	defer ws.mutex.RUnlock()
	return ws.isConnected
}

// GetURL 获取连接URL
func (ws *WebSocketConnection) GetURL() string {
	return ws.url
}

// GetReconnectCount 获取重连次数
func (ws *WebSocketConnection) GetReconnectCount() int {
	ws.mutex.RLock()
	defer ws.mutex.RUnlock()
	return ws.reconnectCount
}

// SetPingInterval 设置心跳间隔
func (ws *WebSocketConnection) SetPingInterval(interval time.Duration) {
	ws.mutex.Lock()
	defer ws.mutex.Unlock()
	ws.pingInterval = interval
}

// SendRawMessage 发送原始字节消息
func (ws *WebSocketConnection) SendRawMessage(data []byte) error {
	ws.mutex.Lock()
	defer ws.mutex.Unlock()

	if !ws.isConnected {
		err := fmt.Errorf("connection not established")
		if ws.errorHandler != nil {
			ws.errorHandler(err)
		}
		return err
	}

	if err := ws.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		if ws.errorHandler != nil {
			ws.errorHandler(fmt.Errorf("failed to send raw message: %w", err))
		}
		return err
	}

	return nil
}

// pingLoop ping保活循环
func (ws *WebSocketConnection) pingLoop() {
	ticker := time.NewTicker(ws.pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ws.ctx.Done():
			return
		case <-ticker.C:
			ws.mutex.Lock()
			if ws.isConnected {
				if err := ws.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					if ws.errorHandler != nil {
						ws.errorHandler(fmt.Errorf("ping failed: %w", err))
					}
					// Ping失败可能意味着连接有问题，标记为断开
					ws.isConnected = false
					if ws.conn != nil {
						ws.conn.Close()
					}
					ws.mutex.Unlock()
					// 触发重连
					if ws.autoReconnect && ws.reconnectCount < ws.maxReconnect {
						go ws.reconnect()
					}
					return
				}
			}
			ws.mutex.Unlock()
		}
	}
}

// SendMessage 发送消息
func (ws *WebSocketConnection) SendMessage(msg interface{}) error {
	ws.mutex.Lock()
	defer ws.mutex.Unlock()

	if !ws.isConnected {
		err := fmt.Errorf("connection not established")
		if ws.errorHandler != nil {
			ws.errorHandler(err)
		}
		return err
	}

	data, err := json.Marshal(msg)
	if err != nil {
		if ws.errorHandler != nil {
			ws.errorHandler(fmt.Errorf("failed to marshal message: %w", err))
		}
		return err
	}

	if err := ws.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		if ws.errorHandler != nil {
			ws.errorHandler(fmt.Errorf("failed to send message: %w", err))
		}
		return err
	}

	return nil
}

// Close 关闭连接
func (ws *WebSocketConnection) Close() error {
	ws.mutex.Lock()
	defer ws.mutex.Unlock()

	ws.isConnected = false
	if ws.cancel != nil {
		ws.cancel()
	}

	if ws.conn != nil {
		if err := ws.conn.Close(); err != nil {
			if ws.errorHandler != nil {
				ws.errorHandler(fmt.Errorf("failed to close connection: %w", err))
			}
			return err
		}
	}

	return nil
}
