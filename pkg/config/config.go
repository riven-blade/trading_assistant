package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

type Config struct {
	// Binance配置
	BinanceAPIKey    string
	BinanceSecretKey string
	BinanceTestnet   bool

	// Redis配置
	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int

	// Telegram配置
	TelegramBotToken string
	TelegramChatID   string

	// 服务配置
	LogLevel string
	BaseURL  string

	// 交易配置
	PositionMode string // both: 双向持仓, single: 单向持仓

	// 风险管理配置
	BalanceRatioThreshold float64 // 余额比例阈值，低于此比例不开仓

	// 认证配置
	AdminUsername string // 管理员用户名
	AdminPassword string // 管理员密码
	JWTSecret     string // JWT密钥
}

var GlobalConfig *Config

func LoadConfig() {
	// 加载.env文件
	if err := godotenv.Load(); err != nil {
		logrus.Warn("未找到.env文件，使用环境变量")
	}

	GlobalConfig = &Config{
		BinanceAPIKey:    getEnv("BINANCE_API_KEY", ""),
		BinanceSecretKey: getEnv("BINANCE_SECRET_KEY", ""),
		BinanceTestnet:   getEnvBool("BINANCE_TESTNET", false),

		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvInt("REDIS_DB", 0),

		TelegramBotToken: getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramChatID:   getEnv("TELEGRAM_CHAT_ID", ""),

		LogLevel: getEnv("LOG_LEVEL", "info"),
		BaseURL:  getEnv("BASE_URL", "localhost"),

		PositionMode: getEnv("POSITION_MODE", "both"),

		BalanceRatioThreshold: getEnvFloat("BALANCE_RATIO_THRESHOLD", 20.0),

		AdminUsername: getEnv("ADMIN_USERNAME", "admin"),
		AdminPassword: getEnv("ADMIN_PASSWORD", ""),
		JWTSecret:     getEnv("JWT_SECRET", "d4f8c1b2e3f4a5b6c7d8e9f0a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0"),
	}

	// 设置日志级别
	level, err := logrus.ParseLevel(GlobalConfig.LogLevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	logrus.SetLevel(level)

	logrus.Info("配置加载完成")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}
