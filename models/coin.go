package models

import (
	"time"
)

// 操作类型常量
const (
	ActionTypeOpen  = "open"  // 开仓/加仓
	ActionTypeClose = "close" // 平仓/止盈/止损
)

// 触发类型常量
const (
	TriggerTypeImmediate = "immediate" // 立即执行
)

// 创建者类型常量（标识具体操作类型）
const (
	CreatedByTakeProfit = "take_profit" // 止盈
	CreatedByStopLoss   = "stop_loss"   // 止损
)

// 价格预估状态常量
const (
	EstimateStatusListening = "listening" // 监听状态（默认状态）
	EstimateStatusTriggered = "triggered" // 已触发成功
	EstimateStatusFailed    = "failed"    // 触发失败
)

// Coin 币种信息
type Coin struct {
	Symbol            string `json:"symbol"`             // 交易对符号，如BTCUSDT
	BaseAsset         string `json:"base_asset"`         // 基础资产，如BTC
	QuoteAsset        string `json:"quote_asset"`        // 计价资产，如USDT
	Status            string `json:"status"`             // 状态：active, inactive
	IsSelected        bool   `json:"is_selected"`        // 是否被筛选
	PricePrecision    int    `json:"price_precision"`    // 价格精度（小数位数）
	QuantityPrecision int    `json:"quantity_precision"` // 数量精度（小数位数）
	TickSize          string `json:"tick_size"`          // 价格最小变动单位
	StepSize          string `json:"step_size"`          // 数量最小变动单位
	MinPrice          string `json:"min_price"`          // 最小价格
	MaxPrice          string `json:"max_price"`          // 最大价格
	MinQty            string `json:"min_qty"`            // 最小数量
	MaxQty            string `json:"max_qty"`            // 最大数量
	// 24小时统计信息
	Price              string    `json:"price"`                // 当前价格
	PriceChange        string    `json:"price_change"`         // 24小时价格变化
	PriceChangePercent string    `json:"price_change_percent"` // 24小时价格变化百分比
	Volume             string    `json:"volume"`               // 24小时成交量
	QuoteVolume        string    `json:"quote_volume"`         // 24小时成交额
	HighPrice          string    `json:"high_price"`           // 24小时最高价
	LowPrice           string    `json:"low_price"`            // 24小时最低价
	OpenPrice          string    `json:"open_price"`           // 24小时开盘价
	ClosePrice         string    `json:"close_price"`          // 最新价格
	Count              int64     `json:"count"`                // 24小时交易次数
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// PriceEstimate 价格预估
type PriceEstimate struct {
	ID          string    `json:"id"`
	Symbol      string    `json:"symbol"`       // 交易对符号
	Side        string    `json:"side"`         // 方向：long, short
	ActionType  string    `json:"action_type"`  // 操作类型：open(开仓/加仓), close(平仓/减仓/止盈/止损)
	TargetPrice float64   `json:"target_price"` // 目标价格
	Quantity    float64   `json:"quantity"`     // 交易数量
	UsdtAmount  float64   `json:"usdt_amount"`  // USDT金额（后端计算）
	Leverage    int       `json:"leverage"`     // 杠杆倍数
	OrderType   string    `json:"order_type"`   // 订单类型：market, limit
	MarginMode  string    `json:"margin_mode"`  // 保证金模式：cross, isolated
	Status      string    `json:"status"`       // 状态：listening(监听状态), triggered(已触发成功), failed(触发失败)
	Enabled     bool      `json:"enabled"`      // 监听开关：true=实际监听, false=暂不监听
	CreatedBy   string    `json:"created_by"`   // 创建者，用于标识具体操作类型
	TriggerType string    `json:"trigger_type"` // 触发条件：immediate(立即执行), condition(条件触发)
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// OrderBook 订单薄数据
type OrderBook struct {
	Symbol    string      `json:"symbol"`
	Bids      []PriceData `json:"bids"` // 买单
	Asks      []PriceData `json:"asks"` // 卖单
	Timestamp int64       `json:"timestamp"`
}

type PriceData struct {
	Price    string `json:"price"`
	Quantity string `json:"quantity"`
}

// Order 订单信息
type Order struct {
	ID          string    `json:"id"`
	Symbol      string    `json:"symbol"`
	Side        string    `json:"side"`         // BUY, SELL
	Type        string    `json:"type"`         // MARKET, LIMIT
	Quantity    float64   `json:"quantity"`     // 原始数量
	ExecutedQty float64   `json:"executed_qty"` // 已执行数量
	Price       float64   `json:"price"`
	MarginMode  string    `json:"margin_mode"` // 保证金模式：cross, isolated
	Status      string    `json:"status"`      // NEW, FILLED, CANCELLED
	EstimateID  string    `json:"estimate_id"` // 关联的价格预估ID
	ExchangeID  string    `json:"exchange_id"` // 交易所返回的订单ID
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Position 持仓信息 (双向持仓模式) - 来自币安PositionRisk API
type Position struct {
	Symbol        string    `json:"symbol"`
	Side          string    `json:"side"`           // LONG, SHORT (币安PositionSide字段)
	Size          float64   `json:"size"`           // 持仓数量 (正数表示多头，负数表示空头)
	EntryPrice    float64   `json:"entry_price"`    // 开仓价格
	MarkPrice     float64   `json:"mark_price"`     // 标记价格 (币安API直接返回)
	UnrealizedPnl float64   `json:"unrealized_pnl"` // 未实现盈亏
	Leverage      int       `json:"leverage"`       // 杠杆倍数 (币安API直接返回)
	MarginMode    string    `json:"margin_mode"`    // 保证金模式: cross, isolated (币安API直接返回)
	Notional      float64   `json:"notional"`       // 持仓价值/名义价值 (币安API直接返回)
	UpdatedAt     time.Time `json:"updated_at"`
}

// Balance 余额信息
type Balance struct {
	Asset     string    `json:"asset"`  // 资产名称
	Free      float64   `json:"free"`   // 可用余额
	Locked    float64   `json:"locked"` // 锁定余额
	Total     float64   `json:"total"`  // 总余额
	UpdatedAt time.Time `json:"updated_at"`
}
