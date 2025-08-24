package models

import "time"

// KLineData K线数据结构
type KLineData struct {
	Timestamp   int64   `json:"timestamp"`    // 开盘时间戳（毫秒）
	Open        float64 `json:"open"`         // 开盘价
	High        float64 `json:"high"`         // 最高价
	Low         float64 `json:"low"`          // 最低价
	Close       float64 `json:"close"`        // 收盘价
	Volume      float64 `json:"volume"`       // 成交量
	QuoteVolume float64 `json:"quote_volume"` // 成交额
	TradeCount  int64   `json:"trade_count"`  // 成交笔数
}

// KLineRequest K线查询请求
type KLineRequest struct {
	Symbol    string `json:"symbol" form:"symbol" binding:"required"`
	Interval  string `json:"interval" form:"interval"`
	StartTime int64  `json:"start_time" form:"start_time"`
	EndTime   int64  `json:"end_time" form:"end_time"`
	Limit     int    `json:"limit" form:"limit"`
}

// KLineResponse K线响应数据
type KLineResponse struct {
	Data []*KLineData   `json:"data"`
	Meta *KLineMetadata `json:"meta"`
}

// KLineMetadata K线元数据
type KLineMetadata struct {
	Symbol     string    `json:"symbol"`
	Interval   string    `json:"interval"`
	Count      int       `json:"count"`
	StartTime  int64     `json:"start_time"`
	EndTime    int64     `json:"end_time"`
	UpdatedAt  time.Time `json:"updated_at"`
	IsRealtime bool      `json:"is_realtime"`
}

// KLineIndicator 技术指标数据
type KLineIndicator struct {
	Name     string                 `json:"name"`     // 指标名称
	Type     string                 `json:"type"`     // overlay, oscillator
	Category string                 `json:"category"` // trend, momentum, volatility, volume
	Data     map[string]interface{} `json:"data"`     // 指标数据
	Params   map[string]interface{} `json:"params"`   // 计算参数
	Color    string                 `json:"color"`    // 显示颜色
	Enabled  bool                   `json:"enabled"`  // 是否启用
}
