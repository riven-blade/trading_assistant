package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
	"trading_assistant/core"
	"trading_assistant/pkg/config"
	"trading_assistant/pkg/exchanges/binance"
	"trading_assistant/pkg/exchanges/types"
	"trading_assistant/pkg/redis"
	"trading_assistant/pkg/telegram"
	"trading_assistant/servers"

	"github.com/sirupsen/logrus"
)

func main() {
	// 设置日志级别
	logrus.SetLevel(logrus.InfoLevel)
	logrus.Info("启动交易助手...")

	// 加载配置
	config.LoadConfig()

	// 初始化Redis
	if err := redis.InitRedis(); err != nil {
		// 发送错误通知
		if telegram.GlobalTelegramClient != nil {
			telegram.GlobalTelegramClient.SendServiceStatus("error", fmt.Sprintf("Redis初始化失败\n错误: %v\n服务即将停止", err))
		}
		logrus.Fatalf("Redis init fail: %v", err)
	}

	// 初始化Binance客户端
	binanceConfig := binance.DefaultConfig()
	binanceConfig.SetAPICredentials(
		config.GlobalConfig.BinanceAPIKey,
		config.GlobalConfig.BinanceSecretKey,
	)
	binanceConfig.MarketType = types.MarketTypeFuture          // 期货市场
	binanceConfig.TestNet = config.GlobalConfig.BinanceTestnet // 应用测试网配置

	// 输出配置信息用于调试
	logrus.WithFields(logrus.Fields{
		"has_api_key": len(config.GlobalConfig.BinanceAPIKey) > 0,
		"has_secret":  len(config.GlobalConfig.BinanceSecretKey) > 0,
		"testnet":     config.GlobalConfig.BinanceTestnet,
		"market_type": binanceConfig.MarketType,
	}).Info("Binance配置信息")

	binanceClient, err := binance.New(binanceConfig)
	if err != nil {
		// 发送错误通知
		if telegram.GlobalTelegramClient != nil {
			telegram.GlobalTelegramClient.SendServiceStatus("error", fmt.Sprintf("Binance客户端初始化失败\n错误: %v\n服务即将停止", err))
		}
		logrus.Fatalf("Binance client init fail: %v", err)
	}
	logrus.Info("Binance客户端已初始化")

	// 初始化Telegram客户端
	if err := telegram.InitTelegram(); err != nil {
		logrus.Errorf("Telegram init fail: %v", err)
	}

	// 初始化市场数据管理器并同步数据
	marketManager := core.NewMarketManager(binanceClient)
	if err := marketManager.SyncMarketAndPriceData(); err != nil {
		logrus.Errorf("同步市场数据和价格数据失败: %v", err)
	}

	// 初始化核心组件
	core.InitOrderQueue(binanceClient)
	core.InitPriceMonitor(binanceClient)
	core.InitAccountManager(binanceClient)

	// 启动WebSocket连接
	if err := binanceClient.StartWebSocket(); err != nil {
		logrus.Errorf("启动WebSocket失败: %v", err)
	}

	// 等待WebSocket连接稳定后再启动订阅
	time.Sleep(3 * time.Second)

	// 启动价格订阅
	if err := marketManager.StartPriceSubscriptions(); err != nil {
		logrus.Errorf("启动价格订阅失败: %v", err)
	}

	// 启动订单队列
	if err := core.GlobalOrderQueue.Start(); err != nil {
		logrus.Errorf("启动订单队列失败: %v", err)
	}

	// 启动价格监控
	core.GlobalPriceMonitor.Start()

	// 启动账户管理器
	core.GlobalAccountManager.Start()

	// 创建HTTP服务器
	server := servers.NewHTTPServer(binanceClient, marketManager)
	go func() {
		server.Start()
	}()

	logrus.Info("交易助手启动完成!")

	// 优雅关闭
	gracefulShutdown(server, binanceClient, marketManager)
}

// gracefulShutdown 优雅关闭
func gracefulShutdown(server *servers.HTTPServer, binanceClient *binance.Binance, marketManager *core.MarketManager) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logrus.Info("正在关闭交易助手...")

	// 停止HTTP服务器 (当前实现没有优雅关闭，直接退出)
	logrus.Info("HTTP服务器将随程序退出关闭")

	// 停止价格订阅
	if marketManager != nil {
		marketManager.StopPriceSubscriptions()
	}

	// 停止核心组件
	if core.GlobalOrderQueue != nil {
		core.GlobalOrderQueue.Stop()
	}

	if core.GlobalAccountManager != nil {
		core.GlobalAccountManager.Stop()
	}

	if core.GlobalPriceMonitor != nil {
		core.GlobalPriceMonitor.Stop()
	}

	// 停止WebSocket连接
	if binanceClient != nil {
		binanceClient.StopWebSocket()
	}

	// 发送服务完全停止的Telegram通知
	if telegram.GlobalTelegramClient != nil {
		if err := telegram.GlobalTelegramClient.SendServiceStatus("stopped", "交易助手已关闭"); err != nil {
			logrus.Errorf("发送关闭完成通知失败: %v", err)
		}
	}

	logrus.Info("交易助手已关闭")
}
