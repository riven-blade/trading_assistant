package binance

import (
	"fmt"
	"trading_assistant/pkg/exchanges/types"
)

// ========== Binance 配置 ==========

// Config Binance 交易所配置
type Config struct {
	// API 认证
	APIKey string `json:"apiKey,omitempty"`
	Secret string `json:"secret,omitempty"`

	// 环境配置
	Sandbox bool `json:"sandbox"` // 是否使用沙盒环境
	TestNet bool `json:"testnet"` // 是否使用测试网

	// 网络配置
	Timeout         int    `json:"timeout"`         // 超时时间(毫秒)
	EnableRateLimit bool   `json:"enableRateLimit"` // 是否启用限流
	Proxy           string `json:"proxy,omitempty"` // 代理地址

	// 高级配置
	RecvWindow int64                  `json:"recvWindow"` // 接收窗口时间(毫秒)
	UserAgent  string                 `json:"userAgent"`  // 用户代理
	Headers    map[string]string      `json:"headers"`    // 自定义头部
	Options    map[string]interface{} `json:"options"`    // 其他选项

	// 市场类型配置
	MarketType string `json:"marketType"` // 市场类型: spot, margin, futures

	// WebSocket 配置
	EnableWebSocket bool `json:"enableWebSocket"` // 是否启用WebSocket
	WSMaxReconnect  int  `json:"wsMaxReconnect"`  // WebSocket最大重连次数
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Sandbox:         false,
		TestNet:         false,
		Timeout:         30000, // 30秒
		EnableRateLimit: true,
		RecvWindow:      5000, // 5秒
		UserAgent:       "trading_assistant/1.0",
		Headers:         make(map[string]string),
		Options:         make(map[string]interface{}),
		MarketType:      types.MarketTypeSpot,
		EnableWebSocket: true,
		WSMaxReconnect:  3,
	}
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Timeout < 0 {
		return fmt.Errorf("timeout cannot be negative")
	}

	if c.RecvWindow < 0 {
		return fmt.Errorf("recvWindow cannot be negative")
	}

	if c.RecvWindow > 60000 {
		return fmt.Errorf("recvWindow cannot exceed 60000ms")
	}

	// 验证市场类型
	validTypes := map[string]bool{
		types.MarketTypeSpot:   true,
		types.MarketTypeMargin: true,
		types.MarketTypeFuture: true,
		types.MarketTypeOption: true,
	}

	if !validTypes[c.MarketType] {
		return fmt.Errorf("invalid marketType: %s", c.MarketType)
	}

	return nil
}

// Clone 克隆配置
func (c *Config) Clone() *Config {
	clone := *c

	// 深拷贝 map
	clone.Headers = make(map[string]string)
	for k, v := range c.Headers {
		clone.Headers[k] = v
	}

	clone.Options = make(map[string]interface{})
	for k, v := range c.Options {
		clone.Options[k] = v
	}

	return &clone
}

// SetAPICredentials 设置API凭证
func (c *Config) SetAPICredentials(apiKey, secret string) {
	c.APIKey = apiKey
	c.Secret = secret
}

// SetEnvironment 设置环境
func (c *Config) SetEnvironment(testnet, sandbox bool) {
	c.TestNet = testnet
	c.Sandbox = sandbox
}

// SetNetworking 设置网络相关配置
func (c *Config) SetNetworking(timeout int, enableRateLimit bool, proxy string) {
	c.Timeout = timeout
	c.EnableRateLimit = enableRateLimit
	c.Proxy = proxy
}

// SetWebSocket 设置WebSocket配置
func (c *Config) SetWebSocket(enable bool, maxReconnect int) {
	c.EnableWebSocket = enable
	c.WSMaxReconnect = maxReconnect
}

// AddHeader 添加自定义头部
func (c *Config) AddHeader(key, value string) {
	if c.Headers == nil {
		c.Headers = make(map[string]string)
	}
	c.Headers[key] = value
}

// SetOption 设置选项
func (c *Config) SetOption(key string, value interface{}) {
	if c.Options == nil {
		c.Options = make(map[string]interface{})
	}
	c.Options[key] = value
}

// GetOption 获取选项
func (c *Config) GetOption(key string) (interface{}, bool) {
	if c.Options == nil {
		return nil, false
	}
	value, exists := c.Options[key]
	return value, exists
}

// ========== 配置验证辅助函数 ==========

// IsValidCredentials 检查是否有有效的API凭证
func (c *Config) IsValidCredentials() bool {
	return c.APIKey != "" && c.Secret != ""
}

// RequiresAuth 检查是否需要认证
func (c *Config) RequiresAuth() bool {
	return !c.Sandbox && c.IsValidCredentials()
}

// GetBaseURL 获取基础URL
func (c *Config) GetBaseURL() string {
	if c.TestNet {
		return "https://testnet.binance.vision"
	}
	if c.Sandbox {
		return "https://testnet.binance.vision"
	}
	return "https://api.binance.com"
}

// GetWebSocketURL 获取WebSocket URL
func (c *Config) GetWebSocketURL() string {
	// 测试网环境
	if c.TestNet {
		if c.MarketType == types.MarketTypeFuture {
			return TestNetFuturesWebSocketURL
		}
		return TestNetWebSocketURL
	}

	// 沙盒环境
	if c.Sandbox {
		if c.MarketType == types.MarketTypeFuture {
			return TestNetFuturesWebSocketURL
		}
		return TestNetWebSocketURL
	}

	// 生产环境 - 根据市场类型选择URL
	switch c.MarketType {
	case types.MarketTypeFuture:
		return FuturesWebSocketURL
	case types.MarketTypeOption:
		return OptionsWebSocketURL
	default: // spot或未指定
		return SpotWebSocketURL
	}
}

// GetFuturesURL 获取期货URL
func (c *Config) GetFuturesURL() string {
	if c.TestNet {
		return "https://testnet.binancefuture.com"
	}
	return "https://fapi.binance.com"
}

// GetOptionsURL 获取期权URL
func (c *Config) GetOptionsURL() string {
	if c.TestNet {
		return "https://testnet.binanceops.com"
	}
	return "https://eapi.binance.com"
}
