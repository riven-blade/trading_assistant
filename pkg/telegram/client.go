package telegram

import (
	"fmt"
	"strconv"
	"time"
	"trading_assistant/models"
	"trading_assistant/pkg/config"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
)

type TelegramClient struct {
	bot    *tgbotapi.BotAPI
	chatID int64
}

var GlobalTelegramClient *TelegramClient

// InitTelegram 初始化Telegram客户端
func InitTelegram() error {
	if config.GlobalConfig.TelegramBotToken == "" {
		logrus.Warn("未配置Telegram Bot Token，跳过Telegram初始化")
		return nil
	}

	bot, err := tgbotapi.NewBotAPI(config.GlobalConfig.TelegramBotToken)
	if err != nil {
		return fmt.Errorf("创建Telegram Bot失败: %v", err)
	}

	bot.Debug = false

	chatID, err := strconv.ParseInt(config.GlobalConfig.TelegramChatID, 10, 64)
	if err != nil {
		return fmt.Errorf("telegram chat ID格式错误: %v", err)
	}

	GlobalTelegramClient = &TelegramClient{
		bot:    bot,
		chatID: chatID,
	}

	logrus.Info("Telegram客户端初始化成功")
	return nil
}

// SendMessage 发送普通消息
func (t *TelegramClient) SendMessage(text string) error {
	if t == nil || t.bot == nil {
		return fmt.Errorf("Telegram客户端未初始化")
	}

	msg := tgbotapi.NewMessage(t.chatID, text)
	msg.ParseMode = "Markdown"

	_, err := t.bot.Send(msg)
	if err != nil {
		return fmt.Errorf("发送消息失败: %v", err)
	}

	return nil
}

// SendPriceAlert 发送价格警报
func (t *TelegramClient) SendPriceAlert(symbol string, currentPrice, targetPrice float64, side string) error {
	message := fmt.Sprintf(
		"%s %s | 价格 %s -> %s | %s",
		symbol, side, formatFloat(currentPrice), formatFloat(targetPrice), getCurrentTimeShort(),
	)

	return t.SendMessage(message)
}

// SendOrderNotification 发送订单通知
func (t *TelegramClient) SendOrderNotification(order *models.Order) error {
	message := fmt.Sprintf(
		"%s %s %s | %s @ %s | %s",
		order.Symbol, order.Side, order.Type, formatFloat(order.Quantity), formatFloat(order.Price), order.Status,
	)

	return t.SendMessage(message)
}

// SendPositionUpdate 发送持仓更新
func (t *TelegramClient) SendPositionUpdate(position *models.Position) error {
	message := fmt.Sprintf(
		"%s %s | %s @ %s | PNL %s USDT",
		position.Symbol, position.Side, formatFloat(position.Size), formatFloat(position.EntryPrice), formatFloat(position.UnrealizedPnl),
	)

	return t.SendMessage(message)
}

// SendSystemStatus 发送系统状态
func (t *TelegramClient) SendSystemStatus(status map[string]interface{}) error {
	redisStatus := "FAIL"
	binanceStatus := "FAIL"

	if redisConnected, ok := status["redis_connected"].(bool); ok && redisConnected {
		redisStatus = "OK"
	}

	if binanceConnected, ok := status["binance_connected"].(bool); ok && binanceConnected {
		binanceStatus = "OK"
	}

	selectedCoins := 0
	pendingEstimates := 0
	positions := 0

	if val, ok := status["selected_coins"].(int); ok {
		selectedCoins = val
	}
	if val, ok := status["pending_estimates"].(int); ok {
		pendingEstimates = val
	}
	if val, ok := status["positions"].(int); ok {
		positions = val
	}

	message := fmt.Sprintf(
		"STATUS Redis %s | Binance %s | Coins %d | Estimates %d | Positions %d | %s",
		redisStatus, binanceStatus, selectedCoins, pendingEstimates, positions, getCurrentTimeShort(),
	)

	return t.SendMessage(message)
}

// SendError 发送错误通知
func (t *TelegramClient) SendError(operation string, err error) error {
	message := fmt.Sprintf(
		"ERROR %s | %s | %s",
		operation, err.Error(), getCurrentTimeShort(),
	)

	return t.SendMessage(message)
}

// getCurrentTimeString 获取当前时间字符串
func getCurrentTimeString() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// getCurrentTimeShort 获取简短时间字符串
func getCurrentTimeShort() string {
	return time.Now().Format("15:04:05")
}

// formatFloat 格式化浮点数，去掉多余的零
func formatFloat(value float64) string {
	if value == 0 {
		return "0"
	}
	// 对于小于1的数，保留更多精度
	if value < 1 {
		return fmt.Sprintf("%.8g", value)
	}
	// 对于大于1的数，保留2-4位小数
	return fmt.Sprintf("%.4g", value)
}
