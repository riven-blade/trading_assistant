package telegram

import (
	"fmt"
	"strconv"
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

// SendOrderNotification 发送订单通知
func (t *TelegramClient) SendOrderNotification(order *models.Order) error {
	message := fmt.Sprintf("%s %s %.4f @ %.4f %s",
		order.Symbol, order.Side, order.Quantity, order.Price, order.Status)

	return t.SendMessage(message)
}

// SendPositionUpdate 发送持仓更新
func (t *TelegramClient) SendPositionUpdate(position *models.Position) error {
	message := fmt.Sprintf("%s %s %.4f @ %.4f | PNL %.4f",
		position.Symbol, position.Side, position.Size, position.EntryPrice, position.UnrealizedPnl)

	return t.SendMessage(message)
}

// SendSystemStatus 发送系统状态
func (t *TelegramClient) SendSystemStatus(status map[string]interface{}) error {
	redisStatus := "X"
	binanceStatus := "X"

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

	message := fmt.Sprintf("R:%s B:%s C:%d E:%d P:%d",
		redisStatus, binanceStatus, selectedCoins, pendingEstimates, positions)

	return t.SendMessage(message)
}

// SendError 发送错误通知
func (t *TelegramClient) SendError(operation string, err error) error {
	message := fmt.Sprintf("ERROR %s", operation)

	return t.SendMessage(message)
}

// SendServiceStatus 发送服务状态通知
func (t *TelegramClient) SendServiceStatus(status, message string) error {
	var emoji string
	switch status {
	case "starting":
		emoji = "🟡"
	case "started":
		emoji = "🟢"
	case "stopping":
		emoji = "🔴"
	case "stopped":
		emoji = "⛔"
	case "error":
		emoji = "❌"
	default:
		emoji = "ℹ️"
	}

	text := fmt.Sprintf("%s **%s**", emoji, message)
	return t.SendMessage(text)
}
