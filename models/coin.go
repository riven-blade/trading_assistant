package models

import (
	"time"
)

// 交易操作类型常量（5种明确的操作类型）
const (
	ActionTypeOpen       = "open"        // 开仓
	ActionTypeAddition   = "addition"    // 加仓
	ActionTypeClose      = "close"       // 平仓
	ActionTypeTakeProfit = "take_profit" // 止盈
	ActionTypeStopLoss   = "stop_loss"   // 止损
)

// 触发类型常量
const (
	TriggerTypeImmediate = "immediate" // 立即执行
	TriggerTypeCondition = "condition" // 条件触发
)

// 价格预估状态常量
const (
	EstimateStatusListening = "listening" // 监听状态（默认状态）
	EstimateStatusQueued    = "queued"    // 排队中
	EstimateStatusTriggered = "triggered" // 已触发成功
	EstimateStatusFailed    = "failed"    // 触发失败
)

// 币种选择状态常量
const (
	CoinSelectionActive   = "active"   // 选中且活跃监听
	CoinSelectionInactive = "inactive" // 取消选中
)

// Coin 币种信息 - 基础市场数据，不包含选中状态
type Coin struct {
	// ========== 基础信息 ==========
	Symbol     string `json:"symbol"`      // 交易对符号，如BTCUSDT
	BaseAsset  string `json:"base_asset"`  // 基础资产，如BTC
	QuoteAsset string `json:"quote_asset"` // 计价资产，如USDT
	Status     string `json:"status"`      // 状态：active, inactive

	// ========== 精度和限制（核心交易参数）==========
	// 价格相关
	TickSize       string `json:"tick_size"`       // 价格最小变动单位（如"0.10000000"）
	PricePrecision int    `json:"price_precision"` // 价格小数位数（从TickSize自动计算）
	MinPrice       string `json:"min_price"`       // 最小价格
	MaxPrice       string `json:"max_price"`       // 最大价格

	// 数量相关
	StepSize          string `json:"step_size"`          // 数量最小变动单位（如"0.00100000"）
	QuantityPrecision int    `json:"quantity_precision"` // 数量小数位数（从StepSize自动计算）
	MinQty            string `json:"min_qty"`            // 最小数量
	MaxQty            string `json:"max_qty"`            // 最大数量

	// ========== 实时价格信息 ==========
	Price              string `json:"price"`                // 当前价格
	PriceChange        string `json:"price_change"`         // 24小时价格变化金额
	PriceChangePercent string `json:"price_change_percent"` // 24小时涨跌幅（如"-2.71"）

	// ========== 交易量信息（核心指标）==========
	Volume      string `json:"volume"`       // 24小时成交量（基础资产）
	QuoteVolume string `json:"quote_volume"` // 24小时成交额（USDT）

	// ========== 时间戳 ==========
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CoinSelection 币种选择状态 - 独立管理选中状态
type CoinSelection struct {
	Symbol    string    `json:"symbol"`     // 交易对符号
	Status    string    `json:"status"`     // 选择状态：active, inactive
	CreatedAt time.Time `json:"created_at"` // 选中时间
	UpdatedAt time.Time `json:"updated_at"` // 更新时间
}

// PriceEstimate 价格预估
type PriceEstimate struct {
	ID          string  `json:"id"`
	Symbol      string  `json:"symbol"`       // 交易对符号
	Side        string  `json:"side"`         // 方向：long, short
	ActionType  string  `json:"action_type"`  // 操作类型：open(开仓/加仓), close(平仓/减仓/止盈/止损)
	TargetPrice float64 `json:"target_price"` // 目标价格
	Quantity    float64 `json:"quantity"`     // 交易数量
	Leverage    int     `json:"leverage"`     // 杠杆倍数
	OrderType   string  `json:"order_type"`   // 订单类型：market, limit
	MarginMode  string  `json:"margin_mode"`  // 保证金模式：CROSS, ISOLATED
	Status      string  `json:"status"`       // 状态：listening(监听状态), triggered(已触发成功), failed(触发失败)
	Enabled     bool    `json:"enabled"`      // 监听开关：true=实际监听, false=暂不监听
	// CreatedBy字段已移除，改用ActionType明确标识操作类型
	TriggerType string    `json:"trigger_type"` // 触发条件：immediate(立即执行), condition(条件触发)
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// OrderBook 订单薄数据
type OrderBook struct {
	Symbol    string      `json:"symbol"`
	Bids      []PriceData `json:"bids"`      // 买单
	Asks      []PriceData `json:"asks"`      // 卖单
	Timestamp int64       `json:"timestamp"` // 更新时间
}

type PriceData struct {
	Price    string `json:"price"`
	Quantity string `json:"quantity"`
}

// Order 订单信息
type Order struct {
	ID           string    `json:"id"`
	Symbol       string    `json:"symbol"`
	Side         string    `json:"side"`          // BUY, SELL (订单方向)
	PositionSide string    `json:"position_side"` // LONG, SHORT, BOTH (持仓方向)
	Type         string    `json:"type"`          // MARKET, LIMIT
	Quantity     float64   `json:"quantity"`      // 原始数量
	ExecutedQty  float64   `json:"executed_qty"`  // 已执行数量
	Price        float64   `json:"price"`
	MarginMode   string    `json:"margin_mode"` // 保证金模式：CROSS, ISOLATED
	Status       string    `json:"status"`      // NEW, FILLED, CANCELLED
	EstimateID   string    `json:"estimate_id"` // 关联的价格预估ID
	ExchangeID   string    `json:"exchange_id"` // 交易所返回的订单ID
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Position 持仓信息 (双向持仓模式)
type Position struct {
	Symbol            string    `json:"symbol"`
	Side              string    `json:"side"`               // LONG, SHORT (币安PositionSide字段)
	Size              float64   `json:"size"`               // 持仓数量 (正数)
	EntryPrice        float64   `json:"entry_price"`        // 开仓价格
	MarkPrice         float64   `json:"mark_price"`         // 标记价格
	UnrealizedPnl     float64   `json:"unrealized_pnl"`     // 未实现盈亏
	Leverage          int       `json:"leverage"`           // 杠杆倍数
	MarginMode        string    `json:"margin_mode"`        // 保证金模式: CROSS, ISOLATED
	IsolatedMargin    float64   `json:"isolated_margin"`    // 逐仓保证金
	InitialMargin     float64   `json:"initial_margin"`     // 初始保证金
	MaintenanceMargin float64   `json:"maintenance_margin"` // 维持保证金
	Notional          float64   `json:"notional"`           // 持仓价值/名义价值
	UpdatedAt         time.Time `json:"updated_at"`
}

// Balance 余额信息
type Balance struct {
	Asset     string    `json:"asset"`  // 资产名称
	Free      float64   `json:"free"`   // 可用余额
	Locked    float64   `json:"locked"` // 锁定余额
	Total     float64   `json:"total"`  // 总余额
	UpdatedAt time.Time `json:"updated_at"`
}

// CoinWithSelection 带选择状态的币种信息
type CoinWithSelection struct {
	Coin
	IsSelected bool `json:"is_selected"`
}

// CoinStatistics 24小时统计信息（可选，用于图表展示）
type CoinStatistics struct {
	Symbol      string    `json:"symbol"`       // 交易对符号
	Volume      string    `json:"volume"`       // 24小时成交量
	QuoteVolume string    `json:"quote_volume"` // 24小时成交额
	HighPrice   string    `json:"high_price"`   // 24小时最高价
	LowPrice    string    `json:"low_price"`    // 24小时最低价
	OpenPrice   string    `json:"open_price"`   // 24小时开盘价
	Count       int64     `json:"count"`        // 24小时交易次数
	UpdatedAt   time.Time `json:"updated_at"`
}

// GetPricePrecisionFromTickSize 从TickSize计算价格精度
func (c *Coin) GetPricePrecisionFromTickSize() int {
	return calculatePrecisionFromStepSize(c.TickSize)
}

// GetQuantityPrecisionFromStepSize 从StepSize计算数量精度
func (c *Coin) GetQuantityPrecisionFromStepSize() int {
	return calculatePrecisionFromStepSize(c.StepSize)
}

// calculatePrecisionFromStepSize 从步长字符串计算精度位数
func calculatePrecisionFromStepSize(stepSize string) int {
	if stepSize == "" || stepSize == "0" {
		return 0
	}

	// 查找小数点位置
	dotIndex := -1
	for i, char := range stepSize {
		if char == '.' {
			dotIndex = i
			break
		}
	}

	if dotIndex == -1 {
		return 0 // 没有小数点，精度为0
	}

	// 计算小数点后的位数，去除尾随的0
	precision := 0
	for i := len(stepSize) - 1; i > dotIndex; i-- {
		if stepSize[i] != '0' {
			precision = i - dotIndex
			break
		}
	}

	return precision
}
