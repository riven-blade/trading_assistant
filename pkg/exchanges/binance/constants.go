package binance

// ========== Binance API 端点常数 ==========

// Binance API 基础URL
const (
	SpotBaseURL       = "https://api.binance.com"
	FuturesBaseURL    = "https://fapi.binance.com"
	OptionsBaseURL    = "https://eapi.binance.com"
	TestNetBaseURL    = "https://testnet.binance.vision"
	TestNetFuturesURL = "https://testnet.binancefuture.com"
	TestNetOptionsURL = "https://testnet.binanceops.com"
)

// Binance WebSocket URL
const (
	SpotWebSocketURL           = "wss://stream.binance.com:9443/ws"
	FuturesWebSocketURL        = "wss://fstream.binance.com/ws"
	OptionsWebSocketURL        = "wss://eapi.binance.com/ws"
	TestNetWebSocketURL        = "wss://testnet.binance.vision/ws"
	TestNetFuturesWebSocketURL = "wss://stream.binancefuture.com/ws"
	TestNetOptionsWebSocketURL = "wss://testnet.binanceops.com/ws"
)

// ========== Binance WebSocket 事件类型常数 ==========

// Binance WebSocket 响应消息中的事件类型（"e"字段的值）
const (
	// 市场数据流事件
	EventTypeTrade          = "trade"           // 交易事件响应
	EventType24hrTicker     = "24hrTicker"      // 24小时价格统计响应
	EventType24hrMiniTicker = "24hrMiniTicker"  // 24小时迷你价格统计响应
	EventTypeBookTicker     = "bookTicker"      // 最优挂单信息响应
	EventTypeMarkPrice      = "markPriceUpdate" // 标记价格更新响应（期货）
	EventTypeKline          = "kline"           // K线事件响应
	EventTypeDepthUpdate    = "depthUpdate"     // 深度更新响应

	// 用户数据流事件
	EventTypeAccountUpdate           = "ACCOUNT_UPDATE"          // 账户更新事件（期货）
	EventTypeOrderTradeUpdate        = "ORDER_TRADE_UPDATE"      // 订单/交易更新事件（期货）
	EventTypeBalanceUpdate           = "balanceUpdate"           // 余额更新响应（现货）
	EventTypeExecutionReport         = "executionReport"         // 执行报告事件（现货）
	EventTypeOutboundAccountPosition = "outboundAccountPosition" // 账户信息推送（现货）
)

// ========== Binance WebSocket 字段名称常量 ==========

// 通用字段
const (
	FieldEventType = "e" // 事件类型
	FieldEventTime = "E" // 事件时间
	FieldSymbol    = "s" // 交易对
)

// Ticker 相关字段
const (
	FieldOpen        = "o" // 开盘价
	FieldHigh        = "h" // 最高价
	FieldLow         = "l" // 最低价
	FieldClose       = "c" // 收盘价
	FieldVolume      = "v" // 成交量
	FieldQuoteVolume = "q" // 成交额
)

// 标记价格相关字段
const (
	FieldMarkPrice   = "p" // 标记价格 (markPriceUpdate 事件中是 "p")
	FieldIndexPrice  = "i" // 指数价格
	FieldFundingRate = "r" // 资金费率
	FieldFundingTime = "T" // 资金费时间
)

// BookTicker 相关字段
const (
	FieldUpdateId = "u" // 更新ID
	FieldBidPrice = "b" // 买价
	FieldBidQty   = "B" // 买量
	FieldAskPrice = "a" // 卖价
	FieldAskQty   = "A" // 卖量
)

// 交易相关字段
const (
	FieldTradeId   = "t" // 交易ID
	FieldPrice     = "p" // 价格
	FieldQuantity  = "q" // 数量
	FieldTradeTime = "T" // 交易时间
)

// K线相关字段
const (
	FieldKlineData      = "k" // K线数据
	FieldKlineStartTime = "t" // K线开始时间
	FieldKlineInterval  = "i" // K线间隔
)

// WebSocket 协议字段
const (
	FieldStream = "stream" // 流名称
	FieldData   = "data"   // 数据内容
	FieldResult = "result" // 响应结果
	FieldError  = "error"  // 错误信息
	FieldMethod = "method" // 方法名称
	FieldParams = "params" // 参数
	FieldId     = "id"     // 请求ID
)

// WebSocket 方法常量
const (
	MethodSubscribe   = "SUBSCRIBE"          // 订阅
	MethodUnsubscribe = "UNSUBSCRIBE"        // 取消订阅
	MethodListStreams = "LIST_SUBSCRIPTIONS" // 列出订阅
	MethodSetProperty = "SET_PROPERTY"       // 设置属性
	MethodGetProperty = "GET_PROPERTY"       // 获取属性
)

// WebSocket 订阅流名称后缀（用于构建订阅请求，如 "btcusdt@trade"）
const (
	StreamSuffixTicker     = "ticker"     // 24小时ticker订阅
	StreamSuffixMiniTicker = "miniTicker" // 迷你ticker订阅
	StreamSuffixBookTicker = "bookTicker" // 最优挂单订阅
	StreamSuffixMarkPrice  = "markPrice"  // 标记价格订阅
	StreamSuffixDepth      = "depth"      // 深度订阅
	StreamSuffixTrade      = "trade"      // 交易订阅
	StreamSuffixKline      = "kline"      // K线订阅
)

// WebSocket 流名称模板常量
const (
	StreamTemplateTrade        = "%s@trade"      // 交易流模板
	StreamTemplateTicker       = "%s@ticker"     // 24小时ticker流模板
	StreamTemplateMiniTicker   = "%s@miniTicker" // 迷你ticker流模板
	StreamTemplateBookTicker   = "%s@bookTicker" // 最优挂单流模板
	StreamTemplateMarkPrice    = "%s@markPrice"  // 标记价格流模板
	StreamTemplateDepth        = "%s@depth"      // 深度流模板
	StreamTemplateKline        = "%s@kline_%s"   // K线流模板 (symbol, interval)
	StreamTemplateKlineDefault = "%s@kline_1m"   // 默认K线流模板
)

// WebSocket 全局流常量
const (
	StreamMarkPriceArray   = "!markPrice@arr"    // 全市场标记价格流（3秒更新）
	StreamMarkPriceArray1s = "!markPrice@arr@1s" // 全市场标记价格流（1秒更新）
)
