package exchanges

import (
	"context"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"trading_assistant/pkg/exchanges/types"
)

// ========== 配置和常量 ==========

// BaseExchange 基础交易所实现
type BaseExchange struct {
	// ========== 基础配置 ==========
	id        string
	name      string
	countries []string
	version   string

	// ========== API 配置 ==========
	apiKey   string
	secret   string
	password string
	uid      string

	// ========== 网络配置 ==========
	sandbox         bool
	testnet         bool
	timeout         time.Duration
	rateLimit       int
	enableRateLimit bool
	httpProxy       string
	userAgent       string
	headers         map[string]string

	// ========== 精度配置 ==========
	precisionMode int
	paddingMode   int

	// ========== 功能支持 ==========
	has        map[string]bool
	timeframes map[string]string

	// ========== URL 配置 ==========
	urls map[string]interface{}
	api  map[string]interface{}

	// ========== 费率配置 ==========
	fees        map[string]map[string]interface{}
	tradingFees map[string]*types.TradingFee
	fundingFees map[string]*types.Currency

	// ========== 运行时状态 ==========
	httpClient      *http.Client
	lastRequestTime int64
	requestCount    int64

	// ========== 简化重试配置 ==========
	maxRetries    int
	retryDelay    time.Duration
	maxRetryDelay time.Duration
	enableJitter  bool

	// ========== 选项配置 ==========
	options map[string]interface{}

	// ========== 市场数据缓存 ==========
	markets       map[string]*types.Market
	marketsLoaded bool
	marketsMutex  sync.RWMutex

	// ========== 同步锁 ==========
	mutex sync.RWMutex
}

// ========== 重试配置方法 ==========

// SetRetryConfig 设置重试配置
func (b *BaseExchange) SetRetryConfig(maxRetries int, retryDelay, maxRetryDelay time.Duration, enableJitter bool) {
	b.maxRetries = maxRetries
	b.retryDelay = retryDelay
	b.maxRetryDelay = maxRetryDelay
	b.enableJitter = enableJitter
}

// EnableRetry 启用重试机制
func (b *BaseExchange) EnableRetry() {
	if b.maxRetries == 0 {
		b.maxRetries = 3 // 默认重试3次
	}
	if b.retryDelay == 0 {
		b.retryDelay = 100 * time.Millisecond // 默认100ms
	}
	if b.maxRetryDelay == 0 {
		b.maxRetryDelay = 10 * time.Second // 默认最大10秒
	}
}

// DisableRetry 禁用重试机制
func (b *BaseExchange) DisableRetry() {
	b.maxRetries = 0
}

// ========== 重试逻辑实现 ==========

// shouldRetry 判断错误是否应该重试
func (b *BaseExchange) shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	// 网络错误，应该重试
	if _, ok := err.(*NetworkError); ok {
		return true
	}

	// 交易所不可用，应该重试
	if _, ok := err.(*ExchangeNotAvailable); ok {
		return true
	}

	// 限流错误，应该重试
	if _, ok := err.(*RateLimitExceeded); ok {
		return true
	}

	// 请求超时，应该重试
	if _, ok := err.(*RequestTimeout); ok {
		return true
	}

	// 检查具体的HTTP相关错误类型
	switch err.(type) {
	case *RateLimitExceeded:
		return true // 429 Too Many Requests
	case *ExchangeNotAvailable:
		return true // 502, 503, 504等
	}

	// 检查错误消息中的关键词
	errMsg := strings.ToLower(err.Error())
	retryableKeywords := []string{
		"connection", "timeout", "network", "temporary",
		"unavailable", "overloaded", "rate limit",
		"too many requests", "service unavailable",
		"bad gateway", "gateway timeout",
	}

	for _, keyword := range retryableKeywords {
		if strings.Contains(errMsg, keyword) {
			return true
		}
	}

	return false
}

// calculateBackoffDelay 计算退避延迟
func (b *BaseExchange) calculateBackoffDelay(attempt int) time.Duration {
	// 指数退避：baseDelay * 2^attempt
	delay := time.Duration(float64(b.retryDelay) * math.Pow(2, float64(attempt)))

	// 限制最大延迟
	if delay > b.maxRetryDelay {
		delay = b.maxRetryDelay
	}

	// 添加随机抖动以避免惊群效应
	if b.enableJitter && attempt > 0 {
		jitterRange := float64(delay) * 0.1                // 10%的抖动范围
		jitter := (rand.Float64() - 0.5) * 2 * jitterRange // -10% 到 +10%
		delay = time.Duration(float64(delay) + jitter)

		// 确保延迟不会为负数
		if delay < 0 {
			delay = b.retryDelay
		}
	}

	return delay
}

// RetryWithBackoff 执行带指数退避的重试
func (b *BaseExchange) RetryWithBackoff(ctx context.Context, operation func() error) error {
	if b.maxRetries == 0 {
		return operation()
	}

	var lastErr error
	for attempt := 0; attempt <= b.maxRetries; attempt++ {
		// 执行操作
		lastErr = operation()
		if lastErr == nil {
			return nil // 成功
		}

		// 检查是否应该重试
		if !b.shouldRetry(lastErr) {
			return lastErr // 不应重试的错误，直接返回
		}

		// 最后一次尝试失败，不再重试
		if attempt >= b.maxRetries {
			break
		}

		// 计算退避延迟
		backoffDelay := b.calculateBackoffDelay(attempt)

		// 等待退避时间
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoffDelay):
			// 继续下一次重试
		}
	}

	return fmt.Errorf("operation failed after %d retries: %w", b.maxRetries, lastErr)
}

// RetryWithBackoffAndResult 执行带指数退避的重试，并返回结果
func (b *BaseExchange) RetryWithBackoffAndResult(ctx context.Context, operation func() (interface{}, error)) (interface{}, error) {
	if b.maxRetries == 0 {
		return operation()
	}

	var lastErr error
	var result interface{}

	for attempt := 0; attempt <= b.maxRetries; attempt++ {
		// 执行操作
		result, lastErr = operation()
		if lastErr == nil {
			return result, nil // 成功
		}

		// 检查是否应该重试
		if !b.shouldRetry(lastErr) {
			return nil, lastErr // 不应重试的错误，直接返回
		}

		// 最后一次尝试失败，不再重试
		if attempt >= b.maxRetries {
			break
		}

		// 计算退避延迟
		backoffDelay := b.calculateBackoffDelay(attempt)

		// 等待退避时间
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoffDelay):
			// 继续下一次重试
		}
	}

	return nil, fmt.Errorf("operation failed after %d retries: %w", b.maxRetries, lastErr)
}

// ========== 构造函数 ==========

// NewBaseExchange 创建基础交易所实例
func NewBaseExchange(id, name, version string, countries []string) *BaseExchange {
	base := &BaseExchange{
		id:              id,
		name:            name,
		version:         version,
		countries:       countries,
		timeout:         30 * time.Second,
		rateLimit:       1000,
		enableRateLimit: true,
		userAgent:       "trading_assistant/1.0.0",
		headers:         make(map[string]string),
		has:             make(map[string]bool),
		timeframes:      make(map[string]string),
		fees:            make(map[string]map[string]interface{}),
		tradingFees:     make(map[string]*types.TradingFee),
		fundingFees:     make(map[string]*types.Currency),
		options:         make(map[string]interface{}),
		httpClient:      &http.Client{Timeout: 30 * time.Second},
		markets:         make(map[string]*types.Market),
		marketsLoaded:   false,
		maxRetries:      3,
		retryDelay:      100 * time.Millisecond,
		maxRetryDelay:   10 * time.Second,
		enableJitter:    true,
	}

	// 设置默认功能支持
	base.setDefaultCapabilities()
	base.setDefaultTimeframes()

	return base
}

// setDefaultCapabilities 设置默认功能支持
func (b *BaseExchange) setDefaultCapabilities() {
	b.has["fetchMarkets"] = true
	b.has["fetchTicker"] = true
	b.has["fetchKline"] = true
	b.has["fetchTrades"] = true
	b.has["fetchOrderBook"] = true
	b.has["fetchBalance"] = false
	b.has["createOrder"] = false
	b.has["cancelOrder"] = false
	b.has["fetchOrder"] = false
	b.has["fetchOrders"] = false
	b.has["fetchOpenOrders"] = false
	b.has["fetchClosedOrders"] = false
	b.has["fetchMyTrades"] = false
	b.has["fetchPositions"] = false
	b.has["fetchFundingRate"] = false
	b.has["setLeverage"] = false
	b.has["setMarginMode"] = false
}

// setDefaultTimeframes 设置默认时间周期
func (b *BaseExchange) setDefaultTimeframes() {
	b.timeframes["1m"] = "1m"
	b.timeframes["3m"] = "3m"
	b.timeframes["5m"] = "5m"
	b.timeframes["15m"] = "15m"
	b.timeframes["30m"] = "30m"
	b.timeframes["1h"] = "1h"
	b.timeframes["2h"] = "2h"
	b.timeframes["4h"] = "4h"
	b.timeframes["6h"] = "6h"
	b.timeframes["8h"] = "8h"
	b.timeframes["12h"] = "12h"
	b.timeframes["1d"] = "1d"
	b.timeframes["3d"] = "3d"
	b.timeframes["1w"] = "1w"
	b.timeframes["1M"] = "1M"
}

// ========== 基础信息方法 ==========

func (b *BaseExchange) GetID() string          { return b.id }
func (b *BaseExchange) GetName() string        { return b.name }
func (b *BaseExchange) GetCountries() []string { return b.countries }
func (b *BaseExchange) GetVersion() string     { return b.version }
func (b *BaseExchange) GetRateLimit() int      { return b.rateLimit }

func (b *BaseExchange) GetTimeout() int      { return int(b.timeout / time.Second) }
func (b *BaseExchange) GetSandbox() bool     { return b.sandbox }
func (b *BaseExchange) GetUserAgent() string { return b.userAgent }
func (b *BaseExchange) GetProxy() string     { return b.httpProxy }
func (b *BaseExchange) GetApiKey() string    { return b.apiKey }
func (b *BaseExchange) GetSecret() string    { return b.secret }
func (b *BaseExchange) GetPassword() string  { return b.password }
func (b *BaseExchange) GetUID() string       { return b.uid }

// 功能支持检查
func (b *BaseExchange) Has() map[string]bool {
	return b.has
}

func (b *BaseExchange) HasAPI(method string) bool {
	if val, exists := b.has[method]; exists {
		return val
	}
	return false
}

// 时间周期
func (b *BaseExchange) GetTimeframes() map[string]string {
	return b.timeframes
}

// ========== 时间处理方法 ==========

func (b *BaseExchange) Milliseconds() int64 {
	return time.Now().UnixMilli()
}

func (b *BaseExchange) Seconds() int64 {
	return time.Now().Unix()
}

func (b *BaseExchange) Microseconds() int64 {
	return time.Now().UnixMicro()
}

func (b *BaseExchange) ISO8601(timestamp int64) string {
	return time.Unix(timestamp/1000, (timestamp%1000)*1000000).UTC().Format("2006-01-02T15:04:05.000Z")
}

func (b *BaseExchange) ParseDate(dateString string) int64 {
	// 支持多种时间格式
	formats := []string{
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateString); err == nil {
			return t.UnixMilli()
		}
	}
	return 0
}

func (b *BaseExchange) YMD(timestamp int64, infix string) string {
	t := time.Unix(timestamp/1000, 0).UTC()
	return fmt.Sprintf("%04d%s%02d%s%02d", t.Year(), infix, int(t.Month()), infix, t.Day())
}

// ========== 安全数据提取方法 ==========

func (b *BaseExchange) SafeString(obj map[string]interface{}, key string, defaultValue string) string {
	if val, exists := obj[key]; exists {
		if str, ok := val.(string); ok {
			return str
		}
		// 特殊处理数值类型，避免科学计数法
		switch v := val.(type) {
		case float64:
			// 对于大整数（订单ID等），使用特殊格式避免科学计数法
			if v == float64(int64(v)) {
				return fmt.Sprintf("%.0f", v)
			}
			return fmt.Sprintf("%v", v)
		case int64:
			return fmt.Sprintf("%d", v)
		case int:
			return fmt.Sprintf("%d", v)
		default:
			// 其他类型使用默认转换
			return fmt.Sprintf("%v", val)
		}
	}
	return defaultValue
}

func (b *BaseExchange) SafeStringLower(obj map[string]interface{}, key string, defaultValue string) string {
	return strings.ToLower(b.SafeString(obj, key, defaultValue))
}

func (b *BaseExchange) SafeStringUpper(obj map[string]interface{}, key string, defaultValue string) string {
	return strings.ToUpper(b.SafeString(obj, key, defaultValue))
}

func (b *BaseExchange) SafeFloat(obj map[string]interface{}, key string, defaultValue float64) float64 {
	if val, exists := obj[key]; exists {
		switch v := val.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int:
			return float64(v)
		case int64:
			return float64(v)
		case string:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return f
			}
		}
	}
	return defaultValue
}

func (b *BaseExchange) SafeInteger(obj map[string]interface{}, key string, defaultValue int64) int64 {
	if val, exists := obj[key]; exists {
		switch v := val.(type) {
		case int64:
			return v
		case int:
			return int64(v)
		case float64:
			return int64(v)
		case string:
			if i, err := strconv.ParseInt(v, 10, 64); err == nil {
				return i
			}
		}
	}
	return defaultValue
}

func (b *BaseExchange) SafeBool(obj map[string]interface{}, key string, defaultValue bool) bool {
	if val, exists := obj[key]; exists {
		if b, ok := val.(bool); ok {
			return b
		}
		// 尝试从字符串转换
		if str, ok := val.(string); ok {
			return strings.ToLower(str) == "true" || str == "1"
		}
	}
	return defaultValue
}

func (b *BaseExchange) SafeValue(obj map[string]interface{}, key string, defaultValue interface{}) interface{} {
	if val, exists := obj[key]; exists {
		return val
	}
	return defaultValue
}

// SafeInt 安全获取整数值
func (b *BaseExchange) SafeInt(data map[string]interface{}, key string, defaultValue int64) int64 {
	if value, exists := data[key]; exists {
		switch v := value.(type) {
		case int:
			return int64(v)
		case int64:
			return v
		case float64:
			return int64(v)
		case string:
			if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
				return parsed
			}
		}
	}
	return defaultValue
}

// FloatToPrecision 浮点数精度转换
func (b *BaseExchange) FloatToPrecision(value float64, precision int) string {
	format := fmt.Sprintf("%%.%df", precision)
	return fmt.Sprintf(format, value)
}

// ========== 精度处理方法 ==========

func (b *BaseExchange) PrecisionFromString(precision string) float64 {
	if f, err := strconv.ParseFloat(precision, 64); err == nil {
		return f
	}
	return 0
}

func (b *BaseExchange) DecimalToPrecision(x float64, precision int, precisionMode, paddingMode int) string {
	switch precisionMode {
	case types.PrecisionModeDecimalPlaces:
		format := fmt.Sprintf("%%.%df", precision)
		result := fmt.Sprintf(format, x)
		if paddingMode == types.PaddingModeNone {
			// 移除尾随零
			result = strings.TrimRight(result, "0")
			result = strings.TrimRight(result, ".")
		}
		return result

	case types.PrecisionModeSignificantDigits:
		format := fmt.Sprintf("%%.%dg", precision)
		return fmt.Sprintf(format, x)

	case types.PrecisionModeTickSize:
		if precision > 0 {
			tickSize := math.Pow(10, -float64(precision))
			rounded := math.Round(x/tickSize) * tickSize
			return strconv.FormatFloat(rounded, 'f', -1, 64)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)

	default:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
}

// ========== URL和参数处理 ==========

func (b *BaseExchange) ImplodeParams(path string, params map[string]interface{}) string {
	result := path
	for key, value := range params {
		placeholder := "{" + key + "}"
		if strings.Contains(result, placeholder) {
			result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
			delete(params, key)
		}
	}
	return result
}

func (b *BaseExchange) ExtractParams(path string) (string, map[string]interface{}) {
	params := make(map[string]interface{})
	re := regexp.MustCompile(`\{([^}]+)\}`)
	matches := re.FindAllStringSubmatch(path, -1)

	for _, match := range matches {
		if len(match) > 1 {
			params[match[1]] = nil
		}
	}

	return path, params
}

// ========== HTTP 请求方法 ==========

// Request 发送HTTP请求
func (b *BaseExchange) Request(ctx context.Context, url string, method string, headers map[string]string, body interface{}, params map[string]interface{}) (*types.Response, error) {
	// 转换body为字符串
	var bodyStr string
	if body != nil {
		if str, ok := body.(string); ok {
			bodyStr = str
		} else if bytes, ok := body.([]byte); ok {
			bodyStr = string(bytes)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(bodyStr))
	if err != nil {
		return nil, err
	}

	// 设置默认头部
	req.Header.Set("User-Agent", b.userAgent)
	if bodyStr != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// 设置自定义头部
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// 使用HTTP客户端
	httpResp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, NewNetworkError("HTTP request failed")
	}

	// 转换为我们的Response类型
	response := &types.Response{
		StatusCode: httpResp.StatusCode,
		Body:       make([]byte, 0),
		Headers:    make(map[string]string),
	}

	// 复制headers
	for k, v := range httpResp.Header {
		if len(v) > 0 {
			response.Headers[k] = v[0]
		}
	}

	// 读取body
	if httpResp.Body != nil {
		defer httpResp.Body.Close()
		bodyBytes, err := io.ReadAll(httpResp.Body)
		if err != nil {
			return nil, NewNetworkError("failed to read response body")
		}
		response.Body = bodyBytes
	}

	return response, nil
}

// Fetch 发送HTTP请求并处理响应
func (b *BaseExchange) Fetch(ctx context.Context, url, method string, headers map[string]string, body string) (string, error) {
	resp, err := b.Request(ctx, url, method, headers, body, nil)
	if err != nil {
		return "", err
	}

	// 读取响应体
	return string(resp.Body), nil
}

// FetchWithRetry 发送带重试的HTTP请求并处理响应
func (b *BaseExchange) FetchWithRetry(ctx context.Context, url, method string, headers map[string]string, body string) (string, error) {
	var resp *types.Response

	err := b.RetryWithBackoff(ctx, func() error {
		var reqErr error
		resp, reqErr = b.Request(ctx, url, method, headers, body, nil)
		if reqErr != nil {
			return reqErr
		}

		// 检查HTTP状态码，某些状态码需要重试
		if resp != nil {
			switch resp.StatusCode {
			case 429: // Too Many Requests
				return NewRateLimitExceeded("rate limit exceeded", 60)
			case 502, 503, 504: // Bad Gateway, Service Unavailable, Gateway Timeout
				return NewExchangeNotAvailable("exchange temporarily unavailable")
			case 500: // Internal Server Error (某些情况下可重试)
				return NewExchangeNotAvailable("internal server error")
			}
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	if resp == nil {
		return "", NewNetworkError("no response received")
	}

	return string(resp.Body), nil
}

// ========== 配置更新方法 ==========

// SetCredentials 设置API凭证
func (b *BaseExchange) SetCredentials(apiKey, secret, password, uid string) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.apiKey = apiKey
	b.secret = secret
	b.password = password
	b.uid = uid
}

// ========== 签名方法的默认实现 ==========
func (b *BaseExchange) Sign(path, api, method string, params map[string]interface{}, headers map[string]string, body interface{}) (string, map[string]string, interface{}, error) {
	return path, headers, body, nil
}
