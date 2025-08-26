package models

// CoinPriceData 币种价格数据结构
type CoinPriceData struct {
	Symbol       string  `json:"symbol"`
	MarkPrice    float64 `json:"mark_price"`
	IndexPrice   float64 `json:"index_price"`
	FundingRate  float64 `json:"funding_rate"`
	FundingTime  int64   `json:"funding_time"`
	UpdateTime   int64   `json:"update_time"`
	PriceChange  string  `json:"price_change,omitempty"`
	PricePercent string  `json:"price_change_percent,omitempty"`
}
