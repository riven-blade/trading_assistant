package apis

import (
	"path/filepath"
	"trading_assistant/controllers"
	"trading_assistant/pkg/middleware"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine) {
	// 创建控制器实例
	coinController := &controllers.CoinController{}
	priceController := &controllers.PriceController{}
	monitorController := &controllers.MonitorController{}
	authController := &controllers.AuthController{}
	klineController := &controllers.KLineController{}

	// 静态文件服务（前端构建后的文件）- 放在认证中间件之前
	webBuildPath := "./web/build"

	// 服务静态资源文件
	r.Static("/static", filepath.Join(webBuildPath, "static"))
	r.StaticFile("/favicon.ico", filepath.Join(webBuildPath, "favicon.ico"))
	r.StaticFile("/favicon.svg", filepath.Join(webBuildPath, "favicon.svg"))
	r.StaticFile("/manifest.json", filepath.Join(webBuildPath, "manifest.json"))

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "Trading Assistant API is running",
		})
	})

	// 添加认证中间件
	r.Use(middleware.AuthMiddleware())

	// 认证路由（不需要token）
	auth := r.Group("/api/v1/auth")
	{
		auth.POST("/login", authController.Login) // 用户登录
	}

	// API版本组（需要认证）
	v1 := r.Group("/api/v1")
	{
		// 用户信息路由
		user := v1.Group("/user")
		{
			user.GET("/profile", authController.GetProfile) // 获取用户信息
		}
		// 币种管理路由
		coins := v1.Group("/coins")
		{
			coins.GET("", coinController.GetCoins)                       // 获取所有币种
			coins.GET("/selected", coinController.GetSelectedCoins)      // 获取已筛选币种
			coins.POST("/select", coinController.SelectCoin)             // 筛选币种
			coins.POST("/sync", coinController.SyncCoins)                // 同步币种
			coins.GET("/precision/:symbol", coinController.GetPrecision) // 获取币种精度信息
		}

		// 价格预估路由
		estimates := v1.Group("/estimates")
		{
			estimates.POST("", priceController.CreatePriceEstimate)           // 创建价格预估
			estimates.GET("", priceController.GetPriceEstimates)              // 获取价格预估列表
			estimates.DELETE("/:id", priceController.DeletePriceEstimate)     // 删除价格预估
			estimates.PUT("/:id/toggle", priceController.TogglePriceEstimate) // 切换价格预估监听状态
		}

		// 监控管理路由
		monitor := v1.Group("/monitor")
		{
			monitor.GET("/account", monitorController.GetAccountStatus)       // 获取账户状态
			monitor.GET("/positions", monitorController.GetPositions)         // 获取持仓信息
			monitor.GET("/balances", monitorController.GetBalances)           // 获取余额信息
			monitor.GET("/orders", monitorController.GetOrders)               // 获取订单信息
			monitor.GET("/orderbook/:symbol", monitorController.GetOrderBook) // 获取订单薄
			monitor.POST("/refresh-cache", monitorController.RefreshCache)    // 手动刷新缓存
		}

		// K线分析路由
		klines := v1.Group("/klines")
		{
			klines.GET("", klineController.GetKLines) // 获取K线数据
		}

	}

	// 服务前端应用（SPA路由）
	r.NoRoute(func(c *gin.Context) {
		// 如果是API路由，返回404
		if len(c.Request.URL.Path) > 4 && c.Request.URL.Path[:5] == "/api/" {
			c.JSON(404, gin.H{"error": "API endpoint not found"})
			return
		}

		// 否则返回前端index.html
		c.File(filepath.Join(webBuildPath, "index.html"))
	})
}
