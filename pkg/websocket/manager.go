package websocket

import (
	"sync"
)

// GlobalWebSocketManager 全局WebSocket管理器实例
var GlobalWebSocketManager *WebSocketManager
var once sync.Once

// InitializeGlobalWebSocketManager 初始化全局WebSocket管理器
func InitializeGlobalWebSocketManager() {
	once.Do(func() {
		GlobalWebSocketManager = NewWebSocketManager()
		GlobalWebSocketManager.Start()
	})
}

// GetGlobalWebSocketManager 获取全局WebSocket管理器实例
func GetGlobalWebSocketManager() *WebSocketManager {
	if GlobalWebSocketManager == nil {
		InitializeGlobalWebSocketManager()
	}
	return GlobalWebSocketManager
}
