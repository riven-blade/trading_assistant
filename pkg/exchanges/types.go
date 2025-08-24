package exchanges

import (
	"fmt"
	"sync"
	"time"
	"trading_assistant/models"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/gorilla/websocket"
)

// OrderSide 交易方向枚举
type OrderSide string

const (
	OrderSideBuy  OrderSide = "BUY"
	OrderSideSell OrderSide = "SELL"
)

// PositionSide 持仓方向枚举（双向持仓模式）
type PositionSide string

const (
	PositionSideLong  PositionSide = "LONG"
	PositionSideShort PositionSide = "SHORT"
)

// ActionType 操作类型枚举
type ActionType string

const (
	ActionTypeOpen  ActionType = "open"  // 开仓
	ActionTypeClose ActionType = "close" // 平仓
)

// OrderType 订单类型枚举
type OrderType string

const (
	OrderTypeMarket OrderType = "MARKET"
	OrderTypeLimit  OrderType = "LIMIT"
)

// MarginMode 保证金模式枚举
type MarginMode string

const (
	MarginModeCross    MarginMode = "cross"
	MarginModeIsolated MarginMode = "isolated"
)

// ToBinanceSideType 转换函数：OrderSide -> Binance SideType
func (s OrderSide) ToBinanceSideType() (futures.SideType, error) {
	switch s {
	case OrderSideBuy:
		return futures.SideTypeBuy, nil
	case OrderSideSell:
		return futures.SideTypeSell, nil
	default:
		return "", fmt.Errorf("无效的OrderSide: %s", s)
	}
}

// ToBinancePositionSideType 转换函数：PositionSide -> Binance PositionSideType
func (p PositionSide) ToBinancePositionSideType() (futures.PositionSideType, error) {
	switch p {
	case PositionSideLong:
		return futures.PositionSideTypeLong, nil
	case PositionSideShort:
		return futures.PositionSideTypeShort, nil
	default:
		return "", fmt.Errorf("无效的PositionSide: %s", p)
	}
}

// ToBinanceOrderType 转换函数：OrderType -> Binance OrderType
func (o OrderType) ToBinanceOrderType() (futures.OrderType, error) {
	switch o {
	case OrderTypeMarket:
		return futures.OrderTypeMarket, nil
	case OrderTypeLimit:
		return futures.OrderTypeLimit, nil
	default:
		return "", fmt.Errorf("无效的OrderType: %s", o)
	}
}

// ToBinanceMarginType 转换函数：MarginMode -> Binance MarginType
func (m MarginMode) ToBinanceMarginType() (futures.MarginType, error) {
	switch m {
	case MarginModeCross:
		return futures.MarginTypeCrossed, nil
	case MarginModeIsolated:
		return futures.MarginTypeIsolated, nil
	default:
		return "", fmt.Errorf("无效的MarginMode: %s", m)
	}
}

// GetOrderSide 工具函数：根据持仓方向和操作类型确定订单方向
func GetOrderSide(positionSide PositionSide, actionType ActionType) (OrderSide, error) {
	switch actionType {
	case ActionTypeOpen:
		// 开仓：订单方向与持仓方向一致
		switch positionSide {
		case PositionSideLong:
			return OrderSideBuy, nil
		case PositionSideShort:
			return OrderSideSell, nil
		default:
			return "", fmt.Errorf("无效的PositionSide: %s", positionSide)
		}
	case ActionTypeClose:
		// 平仓：订单方向与持仓方向相反
		switch positionSide {
		case PositionSideLong:
			return OrderSideSell, nil // 平多仓
		case PositionSideShort:
			return OrderSideBuy, nil // 平空仓
		default:
			return "", fmt.Errorf("无效的PositionSide: %s", positionSide)
		}
	default:
		return "", fmt.Errorf("无效的ActionType: %s", actionType)
	}
}

// StringToPositionSide 工具函数：字符串转PositionSide
func StringToPositionSide(s string) (PositionSide, error) {
	switch s {
	case "long", "LONG":
		return PositionSideLong, nil
	case "short", "SHORT":
		return PositionSideShort, nil
	default:
		return "", fmt.Errorf("无效的位置方向字符串: %s", s)
	}
}

// StringToActionType 工具函数：字符串转ActionType
func StringToActionType(s string) (ActionType, error) {
	switch s {
	case "open":
		return ActionTypeOpen, nil
	case "close":
		return ActionTypeClose, nil
	default:
		return "", fmt.Errorf("无效的操作类型字符串: %s", s)
	}
}

// StringToOrderType 工具函数：字符串转OrderType
func StringToOrderType(s string) (OrderType, error) {
	switch s {
	case "MARKET", "market":
		return OrderTypeMarket, nil
	case "LIMIT", "limit":
		return OrderTypeLimit, nil
	default:
		return "", fmt.Errorf("无效的订单类型字符串: %s", s)
	}
}

// StringToMarginMode 工具函数：字符串转MarginMode
func StringToMarginMode(s string) (MarginMode, error) {
	switch s {
	case "cross":
		return MarginModeCross, nil
	case "isolated":
		return MarginModeIsolated, nil
	default:
		return "", fmt.Errorf("无效的保证金模式字符串: %s", s)
	}
}

// WebSocketConfig WebSocket配置
type WebSocketConfig struct {
	ReadTimeout          int // 读取超时时间（秒）
	PingInterval         int // ping间隔（秒）
	ReconnectInterval    int // 重连间隔（秒）
	MaxReconnectAttempts int // 最大重连次数
}

// DefaultWebSocketConfig 默认WebSocket配置
func DefaultWebSocketConfig() *WebSocketConfig {
	return &WebSocketConfig{
		ReadTimeout:          60,
		PingInterval:         30,
		ReconnectInterval:    5,
		MaxReconnectAttempts: 10,
	}
}

// UserDataHandlers 用户数据处理器
type UserDataHandlers struct {
	OnPosition func(*models.Position) // 持仓更新处理器
	OnOrder    func(*models.Order)    // 订单更新处理器
	OnError    func(error)            // 错误处理器
}

// BinanceWebSocketManager Binance WebSocket管理器
type BinanceWebSocketManager struct {
	config *WebSocketConfig

	// 市场数据相关
	marketDataConn     *websocket.Conn
	marketDataSymbols  map[string]bool
	marketDataRunning  bool
	marketDataMu       sync.RWMutex
	marketDataStopChan chan bool
	marketDataLastPong time.Time

	// 用户数据相关
	userDataRunning   bool
	userDataListenKey string
	userDataHandlers  *UserDataHandlers
	userDataMu        sync.RWMutex

	// 控制通道
	stopChan chan struct{}

	// 重连相关
	marketDataReconnectAttempts int
	userDataReconnectAttempts   int

	// 整体状态
	running bool
	mu      sync.RWMutex
}

// GlobalWebSocketManager 全局WebSocket管理器
var GlobalWebSocketManager *BinanceWebSocketManager
