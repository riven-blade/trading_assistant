package exchanges

import (
	"fmt"
	"net/http"
)

// ========== 错误类型层次结构 ==========

// Error 基础错误接口
type Error interface {
	error
	GetType() string
	GetCode() int
	GetDetails() string
}

// BaseError 基础错误结构
type BaseError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Details string `json:"details"`
	Code    int    `json:"code"`
}

func (e *BaseError) Error() string {
	return e.Message
}

func (e *BaseError) GetType() string {
	return e.Type
}

func (e *BaseError) GetCode() int {
	return e.Code
}

func (e *BaseError) GetDetails() string {
	return e.Details
}

// ========== 网络和连接错误 ==========

// NetworkError 网络错误
type NetworkError struct {
	*BaseError
}

func NewNetworkError(message string) *NetworkError {
	return &NetworkError{
		BaseError: &BaseError{
			Type:    "NetworkError",
			Message: message,
		},
	}
}

// RequestTimeout 请求超时错误
type RequestTimeout struct {
	*BaseError
}

func NewRequestTimeout(message string) *RequestTimeout {
	return &RequestTimeout{
		BaseError: &BaseError{
			Type:    "RequestTimeout",
			Message: message,
		},
	}
}

// DDoSProtection DDoS保护错误
type DDoSProtection struct {
	*BaseError
}

func NewDDoSProtection(message string) *DDoSProtection {
	return &DDoSProtection{
		BaseError: &BaseError{
			Type:    "DDoSProtection",
			Message: message,
		},
	}
}

// ExchangeNotAvailable 交易所不可用错误
type ExchangeNotAvailable struct {
	*BaseError
}

func NewExchangeNotAvailable(message string) *ExchangeNotAvailable {
	return &ExchangeNotAvailable{
		BaseError: &BaseError{
			Type:    "ExchangeNotAvailable",
			Message: message,
		},
	}
}

// ========== 认证和权限错误 ==========

// AuthenticationError 认证错误
type AuthenticationError struct {
	*BaseError
}

func NewAuthenticationError(message string) *AuthenticationError {
	return &AuthenticationError{
		BaseError: &BaseError{
			Type:    "AuthenticationError",
			Message: message,
		},
	}
}

// PermissionDenied 权限拒绝错误
type PermissionDenied struct {
	*BaseError
}

func NewPermissionDenied(message string) *PermissionDenied {
	return &PermissionDenied{
		BaseError: &BaseError{
			Type:    "PermissionDenied",
			Message: message,
			Code:    403,
		},
	}
}

// InvalidNonce 无效随机数错误
type InvalidNonce struct {
	*BaseError
}

func NewInvalidNonce(message string) *InvalidNonce {
	return &InvalidNonce{
		BaseError: &BaseError{
			Type:    "InvalidNonce",
			Message: message,
		},
	}
}

// ========== 限流错误 ==========

// RateLimitExceeded 限流错误
type RateLimitExceeded struct {
	*BaseError
	RetryAfter int // 重试等待时间（秒）
}

func NewRateLimitExceeded(message string, retryAfter int) *RateLimitExceeded {
	return &RateLimitExceeded{
		BaseError: &BaseError{
			Type:    "RateLimitExceeded",
			Message: message,
			Code:    429,
		},
		RetryAfter: retryAfter,
	}
}

// ========== 交易所业务错误 ==========

// ExchangeError 交易所一般错误
type ExchangeError struct {
	*BaseError
}

func NewExchangeError(message string) *ExchangeError {
	return &ExchangeError{
		BaseError: &BaseError{
			Type:    "ExchangeError",
			Message: message,
		},
	}
}

// MarketNotFound 市场未找到错误
type MarketNotFound struct {
	*BaseError
	Symbol string `json:"symbol"`
}

func NewMarketNotFound(symbol string) *MarketNotFound {
	return &MarketNotFound{
		BaseError: &BaseError{
			Type:    "MarketNotFound",
			Message: fmt.Sprintf("market %s not found", symbol),
		},
		Symbol: symbol,
	}
}

// InvalidSymbol 无效交易对错误
type InvalidSymbol struct {
	*BaseError
	Symbol string `json:"symbol"`
}

func NewInvalidSymbol(symbol string) *InvalidSymbol {
	return &InvalidSymbol{
		BaseError: &BaseError{
			Type:    "InvalidSymbol",
			Message: fmt.Sprintf("invalid symbol: %s", symbol),
		},
		Symbol: symbol,
	}
}

// MarketClosed 市场关闭错误
type MarketClosed struct {
	*BaseError
	Symbol string `json:"symbol"`
}

func NewMarketClosed(symbol string) *MarketClosed {
	return &MarketClosed{
		BaseError: &BaseError{
			Type:    "MarketClosed",
			Message: fmt.Sprintf("market %s is closed", symbol),
		},
		Symbol: symbol,
	}
}

// ========== 订单相关错误 ==========

// InvalidOrder 无效订单错误
type InvalidOrder struct {
	*BaseError
}

func NewInvalidOrder(message string, details string) *InvalidOrder {
	return &InvalidOrder{
		BaseError: &BaseError{
			Type:    "InvalidOrder",
			Message: message,
			Details: details,
			Code:    400,
		},
	}
}

// OrderNotFound 订单未找到错误
type OrderNotFound struct {
	*BaseError
	OrderID string `json:"orderId"`
}

func NewOrderNotFound(orderID string) *OrderNotFound {
	return &OrderNotFound{
		BaseError: &BaseError{
			Type:    "OrderNotFound",
			Message: fmt.Sprintf("order %s not found", orderID),
		},
		OrderID: orderID,
	}
}

// InsufficientFunds 余额不足错误
type InsufficientFunds struct {
	*BaseError
	Currency  string  `json:"currency"`
	Required  float64 `json:"required"`
	Available float64 `json:"available"`
}

func NewInsufficientFunds(currency string, required, available float64) *InsufficientFunds {
	return &InsufficientFunds{
		BaseError: &BaseError{
			Type:    "InsufficientFunds",
			Message: fmt.Sprintf("insufficient %s balance: required %.8f, available %.8f", currency, required, available),
		},
		Currency:  currency,
		Required:  required,
		Available: available,
	}
}

// InvalidAmount 无效数量错误
type InvalidAmount struct {
	*BaseError
	Amount float64 `json:"amount"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
}

func NewInvalidAmount(amount, min, max float64) *InvalidAmount {
	return &InvalidAmount{
		BaseError: &BaseError{
			Type:    "InvalidAmount",
			Message: fmt.Sprintf("invalid amount %.8f (min: %.8f, max: %.8f)", amount, min, max),
		},
		Amount: amount,
		Min:    min,
		Max:    max,
	}
}

// InvalidPrice 无效价格错误
type InvalidPrice struct {
	*BaseError
	Price float64 `json:"price"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
}

func NewInvalidPrice(price, min, max float64) *InvalidPrice {
	return &InvalidPrice{
		BaseError: &BaseError{
			Type:    "InvalidPrice",
			Message: fmt.Sprintf("invalid price %.8f (min: %.8f, max: %.8f)", price, min, max),
		},
		Price: price,
		Min:   min,
		Max:   max,
	}
}

// ========== 功能不支持错误 ==========

// NotSupported 功能不支持错误
type NotSupported struct {
	*BaseError
	Feature string `json:"feature"`
}

func NewNotSupported(feature string) *NotSupported {
	return &NotSupported{
		BaseError: &BaseError{
			Type:    "NotSupported",
			Message: fmt.Sprintf("feature not supported: %s", feature),
		},
		Feature: feature,
	}
}

// ========== 参数和请求错误 ==========

// BadRequest 错误请求
type BadRequest struct {
	*BaseError
}

func NewBadRequest(message string) *BadRequest {
	return &BadRequest{
		BaseError: &BaseError{
			Type:    "BadRequest",
			Message: message,
			Code:    400,
		},
	}
}

// ========== 错误工厂和处理函数 ==========

// HTTPError HTTP错误信息
type HTTPError struct {
	StatusCode int    `json:"statusCode"`
	StatusText string `json:"statusText"`
	URL        string `json:"url"`
	Method     string `json:"method"`
	Headers    string `json:"headers"`
	Body       string `json:"body"`
}

// CreateErrorFromHTTP 从HTTP响应创建错误
func createErrorFromHTTP(httpErr HTTPError, exchangeSpecificHandler func(HTTPError) Error) Error {
	// 首先尝试交易所特定的错误处理
	if exchangeSpecificHandler != nil {
		if err := exchangeSpecificHandler(httpErr); err != nil {
			return err
		}
	}

	// 通用HTTP错误处理
	switch httpErr.StatusCode {
	case http.StatusBadRequest:
		return NewBadRequest(fmt.Sprintf("HTTP %d: %s", httpErr.StatusCode, httpErr.StatusText))
	case http.StatusUnauthorized:
		return NewAuthenticationError("unauthorized access")
	case http.StatusForbidden:
		return NewPermissionDenied("forbidden access")
	case http.StatusNotFound:
		return NewExchangeError("endpoint not found")
	case http.StatusTooManyRequests:
		return NewRateLimitExceeded("too many requests", 60)
	case http.StatusInternalServerError:
		return NewExchangeError("internal server error")
	case http.StatusBadGateway:
		return NewExchangeNotAvailable("bad gateway")
	case http.StatusServiceUnavailable:
		return NewExchangeNotAvailable("service unavailable")
	case http.StatusGatewayTimeout:
		return NewRequestTimeout("gateway timeout")
	default:
		if httpErr.StatusCode >= 400 && httpErr.StatusCode < 500 {
			return NewBadRequest(fmt.Sprintf("HTTP %d: %s", httpErr.StatusCode, httpErr.StatusText))
		}
		if httpErr.StatusCode >= 500 {
			return NewExchangeError(fmt.Sprintf("HTTP %d: %s", httpErr.StatusCode, httpErr.StatusText))
		}
		return NewNetworkError(fmt.Sprintf("HTTP %d: %s", httpErr.StatusCode, httpErr.StatusText))
	}
}

// IsRetryable 检查错误是否可重试
func IsRetryable(err error) bool {
	switch err.(type) {
	case *NetworkError, *RequestTimeout, *DDoSProtection, *ExchangeNotAvailable:
		return true
	case *RateLimitExceeded:
		return true // 可以等待后重试
	default:
		return false
	}
}

// GetRetryDelay 获取重试延迟时间(秒)
func GetRetryDelay(err error) int {
	if rateLimitErr, ok := err.(*RateLimitExceeded); ok {
		return rateLimitErr.RetryAfter
	}
	return 1 // 默认1秒
}

// NewInvalidRequest 创建无效请求错误
func NewInvalidRequest(message string) *BaseError {
	return &BaseError{
		Type:    "InvalidRequest",
		Message: message,
		Code:    400,
	}
}
