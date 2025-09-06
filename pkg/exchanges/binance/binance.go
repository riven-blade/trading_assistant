package binance

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"trading_assistant/pkg/exchanges/types"

	"trading_assistant/pkg/exchanges"

	"github.com/sirupsen/logrus"
)

// ========== Binance 交易所实现 ==========

// Binance 实现交易所接口
type Binance struct {
	*exchanges.BaseExchange
	config     *Config
	marketType string // 市场类型：spot, futures

	// API端点缓存
	endpoints map[string]string

	// WebSocket连接池
	wsClient       *WebSocket
	userDataStream *UserDataStream // 独立的期货用户数据流管理器

	// 缓存字段
	lastServerTimeRequest int64
	serverTimeOffset      int64
}

// ========== 构造函数 ==========

// New 创建新的Binance实例
func New(config *Config) (*Binance, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	base := exchanges.NewBaseExchange("binance", "Binance", "v3", []string{"JP", "MT"})
	binance := &Binance{
		BaseExchange: base,
		config:       config.Clone(),
		marketType:   config.MarketType,
		endpoints:    make(map[string]string),
	}

	// 设置基础信息
	binance.setBasicInfo()

	// 设置支持的功能
	binance.setCapabilities()

	// 设置API端点
	binance.setEndpoints()

	// 设置凭证
	binance.SetCredentials(config.APIKey, config.Secret, "", "")

	// 初始同步服务器时间
	go binance.updateServerTimeOffset()

	// 初始化WebSocket (如果启用)
	if config.EnableWebSocket {
		wsConfig := DefaultWebSocketConfig()
		binance.wsClient = NewWebSocket(binance, wsConfig)

		// 初始化独立的期货用户数据流管理器
		binance.userDataStream = NewUserDataStream(binance)
	}

	return binance, nil
}

// setBasicInfo 设置基础信息
func (b *Binance) setBasicInfo() {
	b.BaseExchange.SetRetryConfig(3, 100*time.Millisecond, 10*time.Second, true)
	b.BaseExchange.EnableRetry()
}

// setCapabilities 设置支持的功能
func (b *Binance) setCapabilities() {
	capabilities := map[string]bool{
		"fetchMarkets":    true,
		"fetchTicker":     true,
		"fetchKline":      true,
		"fetchTrades":     true,
		"fetchOrderBook":  true,
		"fetchBalance":    true,
		"createOrder":     true,
		"cancelOrder":     true,
		"fetchOrder":      true,
		"fetchOrders":     true,
		"fetchOpenOrders": true,
		"fetchPositions":  true,
		"setLeverage":     true,
		"setMarginMode":   true,
	}

	// 根据市场类型调整功能
	if b.marketType != types.MarketTypeFuture {
		capabilities["fetchPositions"] = false
		capabilities["setLeverage"] = false
		capabilities["setMarginMode"] = false
	}

	// 设置时间周期
	timeframes := map[string]string{
		"1m":  "1m",
		"3m":  "3m",
		"5m":  "5m",
		"15m": "15m",
		"30m": "30m",
		"1h":  "1h",
		"2h":  "2h",
		"4h":  "4h",
		"6h":  "6h",
		"8h":  "8h",
		"12h": "12h",
		"1d":  "1d",
		"3d":  "3d",
		"1w":  "1w",
		"1M":  "1M",
	}

	// 直接设置功能和时间周期
	for k, v := range capabilities {
		b.BaseExchange.Has()[k] = v
	}
	for k, v := range timeframes {
		b.BaseExchange.GetTimeframes()[k] = v
	}
}

// setEndpoints 设置API端点
func (b *Binance) setEndpoints() {
	baseURL := b.config.GetBaseURL()
	futuresURL := b.config.GetFuturesURL()

	b.endpoints["base"] = baseURL
	b.endpoints["futures"] = futuresURL
	b.endpoints["websocket"] = b.config.GetWebSocketURL()

	// 现货端点
	b.endpoints["exchangeInfo"] = baseURL + "/api/v3/exchangeInfo"
	b.endpoints["ticker24hr"] = baseURL + "/api/v3/ticker/24hr"
	b.endpoints["klines"] = baseURL + "/api/v3/klines"
	b.endpoints["trades"] = baseURL + "/api/v3/trades"
	b.endpoints["depth"] = baseURL + "/api/v3/depth"
	b.endpoints["account"] = baseURL + "/api/v3/account"
	b.endpoints["order"] = baseURL + "/api/v3/order"
	b.endpoints["allOrders"] = baseURL + "/api/v3/allOrders"
	b.endpoints["openOrders"] = baseURL + "/api/v3/openOrders"

	// 期货端点
	if b.marketType == types.MarketTypeFuture {
		b.endpoints["futuresExchangeInfo"] = futuresURL + "/fapi/v1/exchangeInfo"
		b.endpoints["futuresTicker24hr"] = futuresURL + "/fapi/v1/ticker/24hr"
		b.endpoints["futuresKlines"] = futuresURL + "/fapi/v1/klines"
		b.endpoints["futuresDepth"] = futuresURL + "/fapi/v1/depth"
		b.endpoints["futuresAccount"] = futuresURL + "/fapi/v2/account"
		b.endpoints["futuresOrder"] = futuresURL + "/fapi/v1/order"
		b.endpoints["futuresAllOrders"] = futuresURL + "/fapi/v1/allOrders"
		b.endpoints["futuresOpenOrders"] = futuresURL + "/fapi/v1/openOrders"
		b.endpoints["futuresPositionRisk"] = futuresURL + "/fapi/v2/positionRisk"
		b.endpoints["futuresLeverage"] = futuresURL + "/fapi/v1/leverage"
		b.endpoints["futuresMarginType"] = futuresURL + "/fapi/v1/marginType"
		b.endpoints["futuresListenKey"] = futuresURL + "/fapi/v1/listenKey"
	}
}

// ========== 签名和认证 ==========

// Sign 签名请求
func (b *Binance) Sign(path, api, method string, params map[string]interface{}, headers map[string]string, body interface{}) (string, map[string]string, interface{}, error) {
	if headers == nil {
		headers = make(map[string]string)
	}

	// 公开API不需要签名
	if api == "public" {
		query := b.buildQuery(params)
		if query != "" {
			if strings.Contains(path, "?") {
				path += "&" + query
			} else {
				path += "?" + query
			}
		}
		return path, headers, body, nil
	}

	// 私有API需要签名
	if b.GetApiKey() == "" || b.GetSecret() == "" {
		return "", nil, nil, exchanges.NewAuthenticationError("API key and secret required")
	}

	// 添加时间戳
	if params == nil {
		params = make(map[string]interface{})
	}
	params["timestamp"] = b.GetServerTime()

	// 添加接收窗口
	if b.config.RecvWindow > 0 {
		params["recvWindow"] = b.config.RecvWindow
	}

	// 构建查询字符串
	query := b.buildQuery(params)

	// 生成签名
	signature := b.generateSignature(query)
	if query != "" {
		query += "&signature=" + signature
	} else {
		query = "signature=" + signature
	}

	// 添加签名到路径
	if strings.Contains(path, "?") {
		path += "&" + query
	} else {
		path += "?" + query
	}

	// 添加API Key到头部
	headers["X-MBX-APIKEY"] = b.GetApiKey()

	return path, headers, body, nil
}

// buildQuery 构建查询字符串
func (b *Binance) buildQuery(params map[string]interface{}) string {
	if len(params) == 0 {
		return ""
	}

	var keys []string
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		v := params[k]
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}

	return strings.Join(parts, "&")
}

// generateSignature 生成HMAC SHA256签名
func (b *Binance) generateSignature(query string) string {
	mac := hmac.New(sha256.New, []byte(b.GetSecret()))
	mac.Write([]byte(query))
	return hex.EncodeToString(mac.Sum(nil))
}

// signRequest 签名请求
func (b *Binance) signRequest(method, endpoint string, params map[string]interface{}) (string, map[string]string, interface{}, error) {
	headers := make(map[string]string)
	headers["Content-Type"] = "application/x-www-form-urlencoded"

	// 检查是否需要API密钥
	if b.GetApiKey() == "" || b.GetSecret() == "" {
		return "", nil, nil, exchanges.NewAuthenticationError("API key and secret required for signed requests")
	}

	// 确保参数映射存在
	if params == nil {
		params = make(map[string]interface{})
	}

	// 添加时间戳
	params["timestamp"] = b.GetServerTime()

	// 添加接收窗口
	if b.config.RecvWindow > 0 {
		params["recvWindow"] = b.config.RecvWindow
	}

	// 构建查询字符串
	query := b.buildQuery(params)

	// 生成签名
	signature := b.generateSignature(query)
	if query != "" {
		query += "&signature=" + signature
	} else {
		query = "signature=" + signature
	}

	// 构建完整路径
	path := endpoint
	if query != "" {
		if strings.Contains(path, "?") {
			path += "&" + query
		} else {
			path += "?" + query
		}
	}

	// 添加API Key到头部
	headers["X-MBX-APIKEY"] = b.GetApiKey()

	return path, headers, nil, nil
}

// GetServerTime 获取服务器时间
func (b *Binance) GetServerTime() int64 {
	now := time.Now().UnixMilli()

	// 如果有时间偏移，应用偏移
	if b.serverTimeOffset != 0 {
		return now + b.serverTimeOffset
	}

	// 如果距离上次请求服务器时间超过5分钟，更新时间偏移
	if now-b.lastServerTimeRequest > 5*60*1000 {
		go b.updateServerTimeOffset()
	}

	return now
}

// updateServerTimeOffset 更新服务器时间偏移
func (b *Binance) updateServerTimeOffset() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := b.endpoints["base"] + "/api/v3/time"
	resp, err := b.Fetch(ctx, url, "GET", nil, "")
	if err != nil {
		return
	}

	var timeResp struct {
		ServerTime int64 `json:"serverTime"`
	}
	if err := json.Unmarshal([]byte(resp), &timeResp); err != nil {
		return
	}

	localTime := time.Now().UnixMilli()
	b.serverTimeOffset = timeResp.ServerTime - localTime
	b.lastServerTimeRequest = localTime
}

// ========== HTTP通用方法 ==========

// post 发送POST请求
func (b *Binance) post(endpoint string, params map[string]interface{}, signed bool) (map[string]interface{}, error) {
	var url string
	var headers map[string]string
	var body interface{}
	var err error

	if signed {
		// 签名请求
		params["timestamp"] = b.GetServerTime()
		var path string
		path, headers, body, err = b.signRequest("POST", endpoint, params)
		if err != nil {
			return nil, fmt.Errorf("签名请求失败: %w", err)
		}
		url = b.getAPIURL() + path
	} else {
		headers = map[string]string{
			"X-MBX-APIKEY": b.GetApiKey(),
			"Content-Type": "application/x-www-form-urlencoded",
		}
		url = b.getAPIURL() + endpoint

		// 构建body
		if len(params) > 0 {
			values := make([]string, 0, len(params))
			for k, v := range params {
				values = append(values, fmt.Sprintf("%s=%v", k, v))
			}
			body = strings.Join(values, "&")
		}
	}

	response, err := b.Request(context.Background(), url, "POST", headers, body, nil)
	if err != nil {
		return nil, fmt.Errorf("发送POST请求失败: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(response.Body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return result, nil
}

// put 发送PUT请求
func (b *Binance) put(endpoint string, params map[string]interface{}, signed bool) (map[string]interface{}, error) {
	var url string
	var headers map[string]string
	var body interface{}
	var err error

	if signed {
		// 签名请求
		params["timestamp"] = b.GetServerTime()
		var path string
		path, headers, body, err = b.signRequest("PUT", endpoint, params)
		if err != nil {
			return nil, fmt.Errorf("签名请求失败: %w", err)
		}
		url = b.getAPIURL() + path
	} else {
		// 未签名请求，只需要API Key
		headers = map[string]string{
			"X-MBX-APIKEY": b.GetApiKey(),
			"Content-Type": "application/x-www-form-urlencoded",
		}
		url = b.getAPIURL() + endpoint

		// 构建body
		if len(params) > 0 {
			values := make([]string, 0, len(params))
			for k, v := range params {
				values = append(values, fmt.Sprintf("%s=%v", k, v))
			}
			body = strings.Join(values, "&")
		}
	}

	response, err := b.Request(context.Background(), url, "PUT", headers, body, nil)
	if err != nil {
		return nil, fmt.Errorf("发送PUT请求失败: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(response.Body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return result, nil
}

// delete 发送DELETE请求
func (b *Binance) delete(endpoint string, params map[string]interface{}, signed bool) (map[string]interface{}, error) {
	var url string
	var headers map[string]string
	var body interface{}
	var err error

	if signed {
		// 签名请求
		params["timestamp"] = b.GetServerTime()
		var path string
		path, headers, body, err = b.signRequest("DELETE", endpoint, params)
		if err != nil {
			return nil, fmt.Errorf("签名请求失败: %w", err)
		}
		url = b.getAPIURL() + path
	} else {
		// 未签名请求，只需要API Key
		headers = map[string]string{
			"X-MBX-APIKEY": b.GetApiKey(),
			"Content-Type": "application/x-www-form-urlencoded",
		}
		url = b.getAPIURL() + endpoint

		// 构建body
		if len(params) > 0 {
			values := make([]string, 0, len(params))
			for k, v := range params {
				values = append(values, fmt.Sprintf("%s=%v", k, v))
			}
			body = strings.Join(values, "&")
		}
	}

	response, err := b.Request(context.Background(), url, "DELETE", headers, body, nil)
	if err != nil {
		return nil, fmt.Errorf("发送DELETE请求失败: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(response.Body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return result, nil
}

// ========== 市场数据API ==========

// FetchMarkets 获取市场信息
func (b *Binance) FetchMarkets(ctx context.Context, params map[string]interface{}) ([]*types.Market, error) {
	var endpoint string
	if b.marketType == types.MarketTypeFuture {
		endpoint = b.endpoints["futuresExchangeInfo"]
	} else {
		endpoint = b.endpoints["exchangeInfo"]
	}

	respStr, err := b.FetchWithRetry(ctx, endpoint, "GET", nil, "")
	if err != nil {
		return nil, err
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(respStr), &resp); err != nil {
		return nil, err
	}

	symbols, ok := resp["symbols"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	var markets []*types.Market
	for _, symbolData := range symbols {
		symbolMap, ok := symbolData.(map[string]interface{})
		if !ok {
			continue
		}

		market := b.parseMarket(symbolMap)
		if market != nil {
			markets = append(markets, market)
		}
	}

	return markets, nil
}

// parseMarket 解析市场信息
func (b *Binance) parseMarket(data map[string]interface{}) *types.Market {
	symbol := b.SafeString(data, "symbol", "")
	if symbol == "" {
		return nil
	}

	status := b.SafeString(data, "status", "")
	if status != "TRADING" {
		return nil
	}

	baseAsset := b.SafeString(data, "baseAsset", "")
	quoteAsset := b.SafeString(data, "quoteAsset", "")

	// 从API数据中获取合约类型字段
	contractType := b.SafeString(data, "contractType", "")

	// 根据API提供的contractType字段判断是否为永续合约
	isSwap := false
	if b.marketType == types.MarketTypeFuture {
		isSwap = contractType == "PERPETUAL"
	}

	market := &types.Market{
		ID:     symbol,
		Symbol: fmt.Sprintf("%s/%s", baseAsset, quoteAsset),
		Base:   baseAsset,
		Quote:  quoteAsset,
		Type:   b.marketType,
		Active: status == "TRADING",
		Spot:   b.marketType == types.MarketTypeSpot,
		Future: b.marketType == types.MarketTypeFuture,
		Swap:   isSwap, // 根据API的contractType字段正确设置
		Info:   data,
	}

	// 解析精度信息
	if filters, ok := data["filters"].([]interface{}); ok {
		market.Precision = b.parseMarketPrecision(filters)
		market.Limits = b.parseMarketLimits(filters)
	}

	return market
}

// parseMarketPrecision 解析市场精度
func (b *Binance) parseMarketPrecision(filters []interface{}) types.MarketPrecision {
	precision := types.MarketPrecision{}

	for _, filterData := range filters {
		filter, ok := filterData.(map[string]interface{})
		if !ok {
			continue
		}

		filterType := b.SafeString(filter, "filterType", "")
		switch filterType {
		case "LOT_SIZE":
			stepSize := b.SafeString(filter, "stepSize", "")
			precision.Amount = b.PrecisionFromString(stepSize)
		case "PRICE_FILTER":
			tickSize := b.SafeString(filter, "tickSize", "")
			precision.Price = b.PrecisionFromString(tickSize)
		}
	}

	return precision
}

// parseMarketLimits 解析市场限制
func (b *Binance) parseMarketLimits(filters []interface{}) types.MarketLimits {
	limits := types.MarketLimits{}

	for _, filterData := range filters {
		filter, ok := filterData.(map[string]interface{})
		if !ok {
			continue
		}

		filterType := b.SafeString(filter, "filterType", "")
		switch filterType {
		case "LOT_SIZE":
			limits.Amount.Min = b.SafeFloat(filter, "minQty", 0)
			limits.Amount.Max = b.SafeFloat(filter, "maxQty", 0)
			limits.Amount.Step = b.SafeFloat(filter, "stepSize", 0)
		case "PRICE_FILTER":
			limits.Price.Min = b.SafeFloat(filter, "minPrice", 0)
			limits.Price.Max = b.SafeFloat(filter, "maxPrice", 0)
			limits.Price.Step = b.SafeFloat(filter, "tickSize", 0)
		case "MIN_NOTIONAL":
			limits.Cost.Min = b.SafeFloat(filter, "minNotional", 0)
		}
	}

	return limits
}

// FetchTickers 批量获取24小时价格统计
func (b *Binance) FetchTickers(ctx context.Context, symbols []string, params map[string]interface{}) (map[string]*types.Ticker, error) {
	// 如果没有symbols，获取所有ticker
	var endpoint string
	if b.marketType == types.MarketTypeFuture {
		endpoint = b.endpoints["futuresTicker24hr"]
	} else {
		endpoint = b.endpoints["ticker24hr"]
	}

	// 不传symbol参数，获取所有ticker数据
	respStr, err := b.FetchWithRetry(ctx, endpoint, "GET", nil, "")
	if err != nil {
		return nil, err
	}

	// 尝试解析为数组（所有ticker）
	var dataArray []interface{}
	if err := json.Unmarshal([]byte(respStr), &dataArray); err != nil {
		return nil, fmt.Errorf("解析ticker数组失败: %v", err)
	}

	// 转换为map，便于查找
	tickers := make(map[string]*types.Ticker)
	symbolsMap := make(map[string]bool)

	// 如果指定了symbols，创建查找map
	if len(symbols) > 0 {
		for _, symbol := range symbols {
			symbolsMap[symbol] = true
		}
	}

	for _, tickerData := range dataArray {
		tickerMap, ok := tickerData.(map[string]interface{})
		if !ok {
			continue
		}

		// 获取symbol
		symbol := b.SafeString(tickerMap, "symbol", "")
		if symbol == "" {
			continue
		}

		// 如果指定了symbols，只处理指定的symbols
		if len(symbols) > 0 && !symbolsMap[symbol] {
			continue
		}

		ticker := b.parseTicker(tickerMap, symbol)
		tickers[symbol] = ticker
	}

	return tickers, nil
}

// FetchTickersBatch 分批获取ticker数据 - 避免超时
func (b *Binance) FetchTickersBatch(ctx context.Context, symbols []string, batchSize int) (map[string]*types.Ticker, error) {
	if batchSize <= 0 {
		batchSize = 100 // 默认批次大小
	}

	// 如果symbols为空或很小，直接获取全部
	if len(symbols) == 0 || len(symbols) <= batchSize {
		return b.FetchTickers(ctx, symbols, nil)
	}

	allTickers := make(map[string]*types.Ticker)

	// 分批处理
	for i := 0; i < len(symbols); i += batchSize {
		end := i + batchSize
		if end > len(symbols) {
			end = len(symbols)
		}

		batch := symbols[i:end]

		// 获取这一批的ticker数据
		batchTickers, err := b.FetchTickers(ctx, batch, nil)
		if err != nil {
			return nil, fmt.Errorf("批次 %d-%d 获取失败: %v", i, end-1, err)
		}

		// 合并结果
		for symbol, ticker := range batchTickers {
			allTickers[symbol] = ticker
		}

		// 批次间延迟，避免rate limit
		if i+batchSize < len(symbols) {
			time.Sleep(100 * time.Millisecond)
		}
	}

	return allTickers, nil
}

// parseTicker 解析ticker数据
func (b *Binance) parseTicker(data map[string]interface{}, symbol string) *types.Ticker {
	timestamp := b.SafeInteger(data, "closeTime", time.Now().UnixMilli())

	return &types.Ticker{
		Symbol:      symbol,
		TimeStamp:   timestamp,
		Datetime:    b.ISO8601(timestamp),
		High:        b.SafeFloat(data, "highPrice", 0),
		Low:         b.SafeFloat(data, "lowPrice", 0),
		Bid:         b.SafeFloat(data, "bidPrice", 0),
		BidVolume:   b.SafeFloat(data, "bidQty", 0),
		Ask:         b.SafeFloat(data, "askPrice", 0),
		AskVolume:   b.SafeFloat(data, "askQty", 0),
		Open:        b.SafeFloat(data, "openPrice", 0),
		Close:       b.SafeFloat(data, "lastPrice", 0),
		Last:        b.SafeFloat(data, "lastPrice", 0),
		Change:      b.SafeFloat(data, "priceChange", 0),
		Percentage:  b.SafeFloat(data, "priceChangePercent", 0),
		BaseVolume:  b.SafeFloat(data, "volume", 0),
		QuoteVolume: b.SafeFloat(data, "quoteVolume", 0),
		Info:        data,
	}
}

// FetchKlines 获取K线数据
func (b *Binance) FetchKlines(ctx context.Context, symbol, interval string, since int64, limit int, params map[string]interface{}) ([]*types.Kline, error) {
	if symbol == "" {
		return nil, fmt.Errorf("symbol不能为空")
	}

	// 构建请求参数
	requestParams := map[string]interface{}{
		"symbol":   symbol,
		"interval": interval,
	}

	if limit > 0 {
		if limit > 1500 {
			limit = 1500 // Binance最大限制
		}
		requestParams["limit"] = limit
	} else {
		requestParams["limit"] = 500 // 默认值
	}

	// 如果指定了起始时间
	if since > 0 {
		requestParams["startTime"] = since
	}

	// 合并用户参数
	for k, v := range params {
		requestParams[k] = v
	}

	// 选择正确的端点
	var endpoint string
	if b.marketType == types.MarketTypeFuture {
		endpoint = b.endpoints["futuresKlines"]
	} else {
		endpoint = b.endpoints["klines"]
	}

	// 构建查询字符串
	queryParams := make([]string, 0, len(requestParams))
	for k, v := range requestParams {
		queryParams = append(queryParams, fmt.Sprintf("%s=%v", k, v))
	}

	if len(queryParams) > 0 {
		endpoint += "?" + strings.Join(queryParams, "&")
	}

	// 发送请求
	respStr, err := b.FetchWithRetry(ctx, endpoint, "GET", nil, "")
	if err != nil {
		return nil, fmt.Errorf("获取K线数据失败: %w", err)
	}

	// 解析响应
	var rawKlines [][]interface{}
	if err := json.Unmarshal([]byte(respStr), &rawKlines); err != nil {
		return nil, fmt.Errorf("解析K线数据失败: %w", err)
	}

	// 转换为标准格式
	klines := make([]*types.Kline, 0, len(rawKlines))
	for i := range rawKlines {
		rawKline := rawKlines[i]
		kline := b.parseKline(rawKline, symbol, interval)
		if kline != nil {
			klines = append(klines, kline)
		}
	}

	return klines, nil
}

// parseKline 解析K线数据
func (b *Binance) parseKline(data []interface{}, symbol, interval string) *types.Kline {
	if len(data) < 11 {
		return nil
	}

	// Binance K线数据格式:
	// [
	//   1499040000000,      // 开盘时间
	//   "0.01634790",       // 开盘价
	//   "0.80000000",       // 最高价
	//   "0.01575800",       // 最低价
	//   "0.01577100",       // 收盘价(当前K线未结束的即为最新价)
	//   "148976.11427815",  // 成交量
	//   1499644799999,      // 收盘时间
	//   "2434.19055334",    // 成交额
	//   308,                // 成交笔数
	//   "1756.87402397",    // 主动买入成交量
	//   "28.46694368",      // 主动买入成交额
	//   "17928899.62484339" // 请忽略该参数
	// ]

	// 安全的类型转换函数
	toInt64 := func(val interface{}) int64 {
		switch v := val.(type) {
		case float64:
			return int64(v)
		case int64:
			return v
		case int:
			return int64(v)
		case string:
			if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
				return parsed
			}
		}
		return time.Now().UnixMilli()
	}

	toFloat64 := func(val interface{}) float64 {
		switch v := val.(type) {
		case float64:
			return v
		case int64:
			return float64(v)
		case int:
			return float64(v)
		case string:
			if parsed, err := strconv.ParseFloat(v, 64); err == nil {
				return parsed
			}
		}
		return 0
	}

	timestamp := toInt64(data[0])
	closeTime := toInt64(data[6])

	return &types.Kline{
		Symbol:    symbol,
		Timeframe: interval,
		Timestamp: timestamp,
		Open:      toFloat64(data[1]),
		High:      toFloat64(data[2]),
		Low:       toFloat64(data[3]),
		Close:     toFloat64(data[4]),
		Volume:    toFloat64(data[5]),
		IsClosed:  closeTime <= time.Now().UnixMilli(), // 收盘时间小于等于当前时间表示已收盘
	}
}

// FetchOrders 获取订单信息
func (b *Binance) FetchOrders(ctx context.Context, symbol string, since int64, limit int, params map[string]interface{}) ([]*types.Order, error) {
	// 构建请求参数
	requestParams := map[string]interface{}{}

	// 如果指定了交易对
	if symbol != "" {
		requestParams["symbol"] = symbol
	}

	// 如果指定了起始时间
	if since > 0 {
		requestParams["startTime"] = since
	}

	// 如果指定了限制数量
	if limit > 0 {
		if limit > 1000 {
			limit = 1000 // Binance最大限制
		}
		requestParams["limit"] = limit
	}

	// 合并用户参数
	for k, v := range params {
		requestParams[k] = v
	}

	// 添加时间戳（API要求）
	requestParams["timestamp"] = b.GetServerTime()

	// 选择正确的端点
	var endpoint string
	if b.marketType == types.MarketTypeFuture {
		endpoint = "/fapi/v1/allOrders"
	} else {
		endpoint = "/api/v3/allOrders"
	}

	// 签名请求
	path, headers, body, err := b.signRequest("GET", endpoint, requestParams)
	if err != nil {
		return nil, fmt.Errorf("签名请求失败: %w", err)
	}

	// 发送请求
	response, err := b.Request(ctx, b.getAPIURL()+path, "GET", headers, body, nil)
	if err != nil {
		return nil, fmt.Errorf("获取订单数据失败: %w", err)
	}

	// 解析响应
	var rawOrders []map[string]interface{}
	if err := json.Unmarshal(response.Body, &rawOrders); err != nil {
		return nil, fmt.Errorf("解析订单数据失败: %w", err)
	}

	// 转换为标准格式
	orders := make([]*types.Order, 0, len(rawOrders))
	for _, rawOrder := range rawOrders {
		order := b.parseOrder(rawOrder)
		if order != nil {
			orders = append(orders, order)
		}
	}

	return orders, nil
}

// CancelOrder 取消订单
func (b *Binance) CancelOrder(ctx context.Context, symbol string, orderID string, clientOrderID string) error {
	if orderID == "" && clientOrderID == "" {
		return fmt.Errorf("订单ID或客户端订单ID不能为空")
	}

	// 构建请求参数
	params := map[string]interface{}{
		"timestamp": b.GetServerTime(),
	}

	if symbol != "" {
		params["symbol"] = symbol
	}

	if orderID != "" {
		params["orderId"] = orderID
	}

	if clientOrderID != "" {
		params["origClientOrderId"] = clientOrderID
	}

	// 选择正确的端点
	var endpoint string
	if b.marketType == types.MarketTypeFuture {
		endpoint = "/fapi/v1/order"
	} else {
		endpoint = "/api/v3/order"
	}

	// 使用DELETE方法取消订单
	result, err := b.delete(endpoint, params, true)
	if err != nil {
		return fmt.Errorf("取消订单请求失败: %w", err)
	}

	// 检查响应中是否有错误
	if code, ok := result["code"]; ok {
		if msg, ok := result["msg"]; ok {
			return fmt.Errorf("取消订单失败 (code: %v): %v", code, msg)
		}
	}

	logrus.Infof("订单取消成功: orderID=%s, clientOrderID=%s", orderID, clientOrderID)
	return nil
}

// parseOrder 解析订单数据
func (b *Binance) parseOrder(data map[string]interface{}) *types.Order {
	orderID := b.SafeString(data, "orderId", "")
	if orderID == "" {
		return nil
	}

	// 解析时间戳
	timestamp := b.SafeInteger(data, "time", 0)
	updateTime := b.SafeInteger(data, "updateTime", timestamp)

	// 期货和现货的字段可能不同
	var executedQty, cummulativeQuoteQty float64
	if b.marketType == types.MarketTypeFuture {
		executedQty = b.SafeFloat(data, "executedQty", 0)
		cummulativeQuoteQty = b.SafeFloat(data, "cumQuote", 0)
	} else {
		executedQty = b.SafeFloat(data, "executedQty", 0)
		cummulativeQuoteQty = b.SafeFloat(data, "cummulativeQuoteQty", 0)
	}

	return &types.Order{
		ID:                 orderID,
		ClientOrderId:      b.SafeString(data, "clientOrderId", ""),
		Timestamp:          timestamp,
		Datetime:           b.ISO8601(timestamp),
		LastTradeTimestamp: updateTime,
		Symbol:             b.SafeString(data, "symbol", ""),
		Type:               strings.ToLower(b.SafeString(data, "type", "")),
		TimeInForce:        b.SafeString(data, "timeInForce", ""),
		Side:               strings.ToLower(b.SafeString(data, "side", "")),
		PositionSide:       b.SafeString(data, "positionSide", ""),
		Amount:             b.SafeFloat(data, "origQty", 0),
		Price:              b.SafeFloat(data, "price", 0),
		Average:            0, // 需要根据已成交金额和数量计算
		Filled:             executedQty,
		Remaining:          b.SafeFloat(data, "origQty", 0) - executedQty,
		Cost:               cummulativeQuoteQty,
		Status:             strings.ToLower(b.SafeString(data, "status", "")),
		Fee: types.Fee{
			Currency: "", // Binance返回的订单信息中没有手续费信息
			Cost:     0,
		},
		Trades: []types.Trade{}, // 单独获取交易记录
		Info:   data,
	}
}

// ========== 账户数据API ==========

// FetchBalance 获取账户余额信息
func (b *Binance) FetchBalance(ctx context.Context, params map[string]interface{}) (*types.Account, error) {
	var endpoint string
	if b.marketType == types.MarketTypeFuture {
		endpoint = "/fapi/v2/account"
	} else {
		endpoint = "/api/v3/account"
	}

	// 签名请求
	path, headers, body, err := b.signRequest("GET", endpoint, params)
	if err != nil {
		return nil, fmt.Errorf("签名请求失败: %w", err)
	}

	// 发送请求
	response, err := b.Request(ctx, b.getAPIURL()+path, "GET", headers, body, nil)
	if err != nil {
		return nil, fmt.Errorf("获取账户余额失败: %w", err)
	}

	// 解析响应
	var accountResp map[string]interface{}
	if err := json.Unmarshal(response.Body, &accountResp); err != nil {
		return nil, fmt.Errorf("解析账户余额响应失败: %w", err)
	}

	return b.parseBalance(accountResp), nil
}

// parseBalance 解析余额数据
func (b *Binance) parseBalance(data map[string]interface{}) *types.Account {
	account := &types.Account{
		Free:      make(map[string]float64),
		Used:      make(map[string]float64),
		Total:     make(map[string]float64),
		Balances:  make(map[string]types.Balance),
		Info:      data,
		Timestamp: time.Now().UnixMilli(),
	}

	// 解析余额数组
	var balancesKey string
	if b.marketType == types.MarketTypeFuture {
		balancesKey = "assets"
	} else {
		balancesKey = "balances"
	}

	if balancesData, ok := data[balancesKey].([]interface{}); ok {
		for _, balanceItem := range balancesData {
			if balanceMap, ok := balanceItem.(map[string]interface{}); ok {
				asset := b.SafeString(balanceMap, "asset", "")
				if asset == "" {
					continue
				}

				var free, locked, total float64
				if b.marketType == types.MarketTypeFuture {
					total = b.SafeFloat(balanceMap, "walletBalance", 0)
					free = b.SafeFloat(balanceMap, "availableBalance", 0)

					// 如果没有availableBalance字段，回退到marginBalance
					if free == 0 {
						free = b.SafeFloat(balanceMap, "marginBalance", 0)
					}

					locked = total - free
					if locked < 0 {
						locked = 0
					}
				} else {
					free = b.SafeFloat(balanceMap, "free", 0)
					locked = b.SafeFloat(balanceMap, "locked", 0)
					total = free + locked
				}

				// 只保存有余额的资产
				if total > 0 {
					account.Free[asset] = free
					account.Used[asset] = locked
					account.Total[asset] = total
					account.Balances[asset] = types.Balance{
						Free:  free,
						Used:  locked,
						Total: total,
					}
				}
			}
		}
	}

	return account
}

// FetchPositions 获取持仓信息
func (b *Binance) FetchPositions(ctx context.Context, symbols []string, params map[string]interface{}) ([]*types.Position, error) {
	if b.marketType != types.MarketTypeFuture {
		return nil, fmt.Errorf("仓位信息仅在期货模式下可用")
	}

	endpoint := "/fapi/v2/positionRisk"
	requestParams := make(map[string]interface{})

	// 如果指定了交易对
	if len(symbols) == 1 {
		requestParams["symbol"] = symbols[0]
	}

	// 合并用户参数
	for k, v := range params {
		requestParams[k] = v
	}

	// 签名请求
	path, headers, body, err := b.signRequest("GET", endpoint, requestParams)
	if err != nil {
		return nil, fmt.Errorf("签名请求失败: %w", err)
	}

	// 发送请求
	response, err := b.Request(ctx, b.getAPIURL()+path, "GET", headers, body, nil)
	if err != nil {
		return nil, fmt.Errorf("获取持仓信息失败: %w", err)
	}

	// 解析响应
	var positionsResp []map[string]interface{}
	if err := json.Unmarshal(response.Body, &positionsResp); err != nil {
		return nil, fmt.Errorf("解析持仓响应失败: %w", err)
	}

	// 转换为标准格式
	positions := make([]*types.Position, 0)
	for i := range positionsResp {
		positionData := positionsResp[i]
		position := b.parsePosition(positionData)
		if position != nil && position.Size != 0 { // 只返回有持仓的记录
			positions = append(positions, position)
		}
	}

	return positions, nil
}

// parsePosition 解析持仓数据
func (b *Binance) parsePosition(data map[string]interface{}) *types.Position {
	symbol := b.SafeString(data, "symbol", "")
	if symbol == "" {
		return nil
	}

	positionAmt := b.SafeFloat(data, "positionAmt", 0)
	if positionAmt == 0 {
		return nil // 忽略无持仓的记录
	}

	// 确定持仓方向和数量
	var side string
	var size float64
	positionSide := b.SafeString(data, "positionSide", "")

	if positionSide == "BOTH" {
		// 单向持仓模式：通过数量正负判断方向
		if positionAmt > 0 {
			side = types.PositionSideLong
			size = positionAmt
		} else {
			side = types.PositionSideShort
			size = -positionAmt // 转为正数
		}
	} else {
		// 双向持仓模式：方向已确定，数量取绝对值
		side = strings.ToLower(positionSide)
		if positionAmt < 0 {
			size = -positionAmt
		} else {
			size = positionAmt
		}
	}

	// 计算名义价值 (notional)
	markPrice := b.SafeFloat(data, "markPrice", 0)
	notional := b.SafeFloat(data, "notional", 0)
	if notional < 0 {
		notional = -notional // 确保notional为正数
	}
	if notional == 0 && markPrice > 0 {
		notional = size * markPrice
	}

	return &types.Position{
		Symbol:            symbol,
		Size:              size,
		Side:              side,
		EntryPrice:        b.SafeFloat(data, "entryPrice", 0),
		MarkPrice:         markPrice,
		NotionalValue:     notional,
		UnrealizedPnl:     b.SafeFloat(data, "unRealizedProfit", 0),
		Timestamp:         b.SafeInt(data, "updateTime", time.Now().UnixMilli()),
		Leverage:          b.SafeFloat(data, "leverage", 1),
		MarginType:        b.SafeString(data, "marginType", ""),
		IsolatedMargin:    b.SafeFloat(data, "isolatedMargin", 0),
		InitialMargin:     b.SafeFloat(data, "initialMargin", 0),
		MaintenanceMargin: b.SafeFloat(data, "maintMargin", 0),
		Info:              data,
	}
}

// ========== WebSocket相关方法 ==========

// StartWebSocket 启动WebSocket连接
func (b *Binance) StartWebSocket() error {
	if b.wsClient == nil {
		return fmt.Errorf("websocket not initialized")
	}
	return b.wsClient.Start()
}

// ========== 用户数据流相关方法 ==========

// CreateListenKey 创建用户数据流监听密钥
func (b *Binance) CreateListenKey() (string, error) {
	endpoint := "/fapi/v1/listenKey"

	params := make(map[string]interface{})
	result, err := b.post(endpoint, params, false) // 不需要签名，但需要API key
	if err != nil {
		logrus.Errorf("创建listenKey失败: %v", err)
		return "", fmt.Errorf("创建listenKey失败: %w", err)
	}

	if listenKey, ok := result["listenKey"].(string); ok {
		logrus.Infof("成功创建listenKey: %s", listenKey[:16]+"...")
		return listenKey, nil
	}

	logrus.Errorf("listenKey响应格式错误，响应内容: %+v", result)
	return "", fmt.Errorf("listenKey响应格式错误")
}

// KeepaliveListenKey 延长listenKey有效期
func (b *Binance) KeepaliveListenKey(listenKey string) error {
	endpoint := "/fapi/v1/listenKey"

	params := map[string]interface{}{
		"listenKey": listenKey,
	}

	_, err := b.put(endpoint, params, false) // 不需要签名，但需要API key
	if err != nil {
		return fmt.Errorf("延长listenKey失败: %w", err)
	}

	return nil
}

// CloseListenKey 关闭用户数据流
func (b *Binance) CloseListenKey(listenKey string) error {
	endpoint := "/fapi/v1/listenKey"

	params := map[string]interface{}{
		"listenKey": listenKey,
	}

	_, err := b.delete(endpoint, params, false) // 不需要签名，但需要API key
	if err != nil {
		return fmt.Errorf("关闭listenKey失败: %w", err)
	}

	return nil
}

// StopWebSocket 停止WebSocket连接
func (b *Binance) StopWebSocket() {
	if b.wsClient != nil {
		b.wsClient.Stop()
	}
}

// SubscribeToOrderbook 订阅订单簿
func (b *Binance) SubscribeToOrderbook(symbol string, publishFunc func(types.MetaData, interface{}) error) error {
	if b.wsClient == nil {
		return fmt.Errorf("websocket not initialized")
	}

	b.wsClient.publishFunc = publishFunc
	streamName := fmt.Sprintf("%s@depth", strings.ToLower(strings.Replace(symbol, "/", "", -1)))
	return b.wsClient.UnsubscribeStream(streamName) // 注意：当前WebSocket实现主要专注于MarkPrice，此功能可能需要扩展
}

// UnsubscribeFromOrderbook 取消订阅订单簿
func (b *Binance) UnsubscribeFromOrderbook(symbol string) error {
	if b.wsClient == nil {
		return fmt.Errorf("websocket not initialized")
	}

	streamName := fmt.Sprintf("%s@depth", strings.ToLower(strings.Replace(symbol, "/", "", -1)))
	return b.wsClient.UnsubscribeStream(streamName)
}

// SubscribeToMarkPrice 订阅所有币种的标记价格数组流
func (b *Binance) SubscribeToMarkPrice(publishFunc func(types.MetaData, interface{}) error) error {
	if b.wsClient == nil {
		return fmt.Errorf("websocket not initialized")
	}

	// 设置发布函数
	if publishFunc != nil {
		b.wsClient.SetPublishFunc(publishFunc)
	}

	// 订阅所有币种的标记价格流（使用新的简化API）
	return b.wsClient.SubscribeMarkPrice()
}

// UnsubscribeFromMarkPrice 取消订阅标记价格数组流
func (b *Binance) UnsubscribeFromMarkPrice() error {
	if b.wsClient == nil {
		return fmt.Errorf("websocket not initialized")
	}

	return b.wsClient.UnsubscribeStream(StreamMarkPriceArray1s)
}

// ========== 用户数据流订阅方法 ==========

// SubscribeToUserData 订阅用户数据流
func (b *Binance) SubscribeToUserData(publishFunc func(types.MetaData, interface{}) error) error {
	if b.userDataStream == nil {
		return fmt.Errorf("user data stream not initialized")
	}

	return b.userDataStream.Start(publishFunc)
}

// UnsubscribeFromUserData 取消订阅用户数据流
func (b *Binance) UnsubscribeFromUserData() error {
	if b.userDataStream == nil {
		return fmt.Errorf("user data stream not initialized")
	}

	return b.userDataStream.Stop()
}

// GetWebSocketClient 获取WebSocket客户端
func (b *Binance) GetWebSocketClient() *WebSocket {
	return b.wsClient
}

// SetWebSocketReconnectHandler 设置WebSocket重连处理器
func (b *Binance) SetWebSocketReconnectHandler(handler func(int, error)) {
	// 为普通连接池设置重连处理器
	if b.wsClient != nil {
		b.wsClient.SetReconnectHandler(handler)
	}
}

// SetUserDataReconnectHandler 设置用户数据流重连处理器
func (b *Binance) SetUserDataReconnectHandler(handler func(int, error)) {
	if b.userDataStream != nil {
		b.userDataStream.SetReconnectHandler(handler)
	}
}

// SetUserDataErrorHandler 设置用户数据流错误处理器
func (b *Binance) SetUserDataErrorHandler(handler func(error)) {
	if b.userDataStream != nil {
		b.userDataStream.SetErrorHandler(handler)
	}
}

// GetUserDataStats 获取用户数据流统计信息
func (b *Binance) GetUserDataStats() map[string]interface{} {
	if b.userDataStream != nil {
		return b.userDataStream.GetStats()
	}
	return nil
}

// ========== 期货交易API ==========

// FuturesNewOrder 期货下单 - 支持双向持仓
func (b *Binance) FuturesNewOrder(params map[string]interface{}) (*FuturesOrderResponse, error) {
	endpoint := "/fapi/v1/order"

	// 添加时间戳
	params["timestamp"] = b.GetServerTime()

	// 签名请求
	path, headers, body, err := b.signRequest("POST", endpoint, params)
	if err != nil {
		return nil, fmt.Errorf("签名请求失败: %v", err)
	}

	// 发送请求
	response, err := b.Request(context.Background(), b.getAPIURL()+path, "POST", headers, body, nil)
	if err != nil {
		return nil, fmt.Errorf("发送下单请求失败: %v", err)
	}

	// 检查HTTP状态码
	if response.StatusCode != 200 && response.StatusCode != 201 {
		// 尝试解析错误响应
		var errorResp struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		if err := json.Unmarshal(response.Body, &errorResp); err == nil {
			return nil, fmt.Errorf("binance下单失败 [%d]: %s (HTTP %d)",
				errorResp.Code, errorResp.Msg, response.StatusCode)
		}
		return nil, fmt.Errorf("binance下单失败: HTTP %d, 响应: %s",
			response.StatusCode, string(response.Body))
	}

	var orderResp FuturesOrderResponse
	if err := json.Unmarshal(response.Body, &orderResp); err != nil {
		return nil, fmt.Errorf("解析下单响应失败: %v, 响应内容: %s", err, string(response.Body))
	}

	// 验证订单ID
	if orderResp.OrderID == 0 {
		return nil, fmt.Errorf("下单失败: 返回的OrderID为0, 响应: %+v", orderResp)
	}

	return &orderResp, nil
}

// SetLeverage 设置杠杆
func (b *Binance) SetLeverage(symbol string, leverage int) error {
	params := map[string]interface{}{
		"symbol":    symbol,
		"leverage":  leverage,
		"timestamp": b.GetServerTime(),
	}

	endpoint := "/fapi/v1/leverage"
	path, headers, body, err := b.signRequest("POST", endpoint, params)
	if err != nil {
		return fmt.Errorf("签名设置杠杆请求失败: %v", err)
	}

	_, err = b.Request(context.Background(), b.getAPIURL()+path, "POST", headers, body, nil)
	if err != nil {
		return fmt.Errorf("设置杠杆失败: %v", err)
	}

	return nil
}

// SetMarginType 设置保证金模式
func (b *Binance) SetMarginType(symbol string, marginType string) error {
	params := map[string]interface{}{
		"symbol":     symbol,
		"marginType": marginType, // ISOLATED 或 CROSSED
		"timestamp":  time.Now().UnixMilli(),
	}

	endpoint := "/fapi/v1/marginType"
	path, headers, body, err := b.signRequest("POST", endpoint, params)
	if err != nil {
		return fmt.Errorf("签名设置保证金模式请求失败: %v", err)
	}

	_, err = b.Request(context.Background(), b.getAPIURL()+path, "POST", headers, body, nil)
	if err != nil {
		return fmt.Errorf("设置保证金模式失败: %v", err)
	}

	return nil
}

// getAPIURL 获取API基础URL
func (b *Binance) getAPIURL() string {
	// 使用config中的URL配置，保证与其他方法一致
	return b.config.GetFuturesURL()
}

// ========== 响应结构体 ==========

// FuturesOrderResponse 期货订单响应
type FuturesOrderResponse struct {
	ClientOrderID string `json:"clientOrderId"`
	CumQty        string `json:"cumQty"`
	CumQuote      string `json:"cumQuote"`
	ExecutedQty   string `json:"executedQty"`
	OrderID       int64  `json:"orderId"`
	AvgPrice      string `json:"avgPrice"`
	OrigQty       string `json:"origQty"`
	Price         string `json:"price"`
	ReduceOnly    bool   `json:"reduceOnly"`
	Side          string `json:"side"`
	PositionSide  string `json:"positionSide"`
	Status        string `json:"status"`
	StopPrice     string `json:"stopPrice"`
	ClosePosition bool   `json:"closePosition"`
	Symbol        string `json:"symbol"`
	TimeInForce   string `json:"timeInForce"`
	Type          string `json:"type"`
	OrigType      string `json:"origType"`
	ActivatePrice string `json:"activatePrice"`
	PriceRate     string `json:"priceRate"`
	UpdateTime    int64  `json:"updateTime"`
	WorkingType   string `json:"workingType"`
	PriceProtect  bool   `json:"priceProtect"`
}

// ========== 实用方法 ==========

// GetMarketType 获取市场类型
func (b *Binance) GetMarketType() string {
	return b.marketType
}

// IsTestnet 是否测试网
func (b *Binance) IsTestnet() bool {
	return b.config.TestNet
}

// GetConfig 获取配置
func (b *Binance) GetConfig() *Config {
	return b.config
}
