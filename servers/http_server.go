package servers

import (
	"fmt"
	"trading_assistant/apis"
	"trading_assistant/core"
	"trading_assistant/pkg/config"
	"trading_assistant/pkg/exchanges/binance"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type HTTPServer struct {
	engine        *gin.Engine
	port          string
	binanceClient *binance.Binance
	marketManager *core.MarketManager
}

// NewHTTPServer 创建HTTP服务器
func NewHTTPServer(binanceClient *binance.Binance, marketManager *core.MarketManager) *HTTPServer {
	// 设置Gin模式
	if config.GlobalConfig.LogLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.Default()

	// 添加CORS中间件
	engine.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		baseURL := config.GlobalConfig.BaseURL

		// 构建允许的源列表
		allowedOrigins := []string{
			"http://" + baseURL + ":3000",
			"http://" + baseURL + ":8080",
			"https://" + baseURL + ":8080",
			"http://" + baseURL,
			"https://" + baseURL,
		}

		// 检查是否为允许的源
		isAllowed := false
		for _, allowed := range allowedOrigins {
			if origin == allowed {
				isAllowed = true
				break
			}
		}

		if isAllowed {
			c.Header("Access-Control-Allow-Origin", origin)
		} else if origin == "" {
			// 允许无Origin的请求（如直接API调用）
			c.Header("Access-Control-Allow-Origin", "*")
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// 设置路由
	apis.SetupRoutes(engine, binanceClient, marketManager)

	return &HTTPServer{
		engine:        engine,
		port:          "8080",
		binanceClient: binanceClient,
		marketManager: marketManager,
	}
}

// Start 启动HTTP服务器
func (s *HTTPServer) Start() {
	addr := fmt.Sprintf(":%s", s.port)
	logrus.Infof("HTTP服务器启动在端口 %s", s.port)

	if err := s.engine.Run(addr); err != nil {
		logrus.Fatalf("HTTP服务器启动失败: %v", err)
	}
}
