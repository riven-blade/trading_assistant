package main

import (
	"os"
	"os/signal"
	"syscall"
	"trading_assistant/core"
	"trading_assistant/pkg/config"
	"trading_assistant/pkg/exchanges"
	"trading_assistant/pkg/redis"
	"trading_assistant/pkg/telegram"
	"trading_assistant/servers"

	"github.com/sirupsen/logrus"
)

func main() {
	// 初始化日志
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	logrus.SetReportCaller(false)

	logrus.Info("start trading assistant...")

	// 加载配置
	config.LoadConfig()

	// 初始化Redis
	if err := redis.InitRedis(); err != nil {
		logrus.Fatalf("Redis init fail: %v", err)
	}

	// 初始化Binance客户端
	if err := exchanges.InitBinance(); err != nil {
		logrus.Fatalf("Binance init fail: %v", err)
	}

	// 初始化Telegram客户端
	if err := telegram.InitTelegram(); err != nil {
		logrus.Errorf("Telegram init fail: %v", err)
	}

	// 初始化核心组件
	core.InitPriceMonitor()
	core.InitAccountManager()

	// 启动市场数据WebSocket流
	selectedCoins, err := redis.GlobalRedisClient.GetSelectedCoins()
	if err != nil {
		logrus.Errorf("获取筛选币种失败: %v", err)
	} else {
		var symbols []string
		for _, coin := range selectedCoins {
			symbols = append(symbols, coin.Symbol)
		}

		if len(symbols) > 0 {
			if err := exchanges.GlobalWebSocketManager.StartMarketData(symbols); err != nil {
				logrus.Errorf("启动市场数据WebSocket失败: %v", err)
			} else {
				logrus.Infof("市场数据WebSocket已启动，监听 %d 个币种", len(symbols))
			}
		}
	}

	// 启动价格监控
	core.GlobalPriceMonitor.Start()

	// 启动账户管理器（WebSocket监听持仓和余额）
	core.GlobalAccountManager.Start()

	// 创建HTTP服务器
	httpServer := servers.NewHTTPServer()

	// 启动HTTP服务器（在goroutine中）
	go httpServer.Start()

	// 等待中断信号
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	logrus.Info("trading assistant is running...")

	// 等待退出信号
	<-signalChan

	logrus.Info("stopping trading assistant...")

	// 优雅关闭
	gracefulShutdown()

	logrus.Info("trading assistant stopped")
}

// gracefulShutdown 优雅关闭
func gracefulShutdown() {
	// 停止价格监控
	if core.GlobalPriceMonitor != nil {
		core.GlobalPriceMonitor.Stop()
	}

	// 停止账户管理器
	if core.GlobalAccountManager != nil {
		core.GlobalAccountManager.Stop()
	}

	// 停止WebSocket管理器
	if exchanges.GlobalWebSocketManager != nil {
		err := exchanges.GlobalWebSocketManager.Stop()
		if err != nil {
			return
		}
	}

	// 发送关闭通知
	if telegram.GlobalTelegramClient != nil {
		err := telegram.GlobalTelegramClient.SendMessage("Trading Assistant Stopped")
		if err != nil {
			return
		}
	}
}
