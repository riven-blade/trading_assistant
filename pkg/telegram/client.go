package telegram

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"trading_assistant/pkg/config"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
)

const (
	MaxMessageLength = 4096 // Telegram单条消息最大长度
)

type TelegramClient struct {
	bot    *tgbotapi.BotAPI
	chatID int64
}

var GlobalTelegramClient *TelegramClient

// 获取中国时区
func getChinaLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		logrus.Warnf("无法加载中国时区，使用UTC: %v", err)
		return time.UTC
	}
	return loc
}

// 格式化创建时间为完整的年月日时间格式
func formatCreationTime(t time.Time) string {
	chinaLoc := getChinaLocation()
	localTime := t.In(chinaLoc)
	return localTime.Format("2006-01-02 15:04:05")
}

// 安全发送消息，处理长消息分割
func (t *TelegramClient) sendMessageSafely(text string) error {
	if t == nil || t.bot == nil {
		return fmt.Errorf("Telegram客户端未初始化")
	}

	// 如果消息长度超过限制，进行分割
	if len(text) <= MaxMessageLength {
		return t.SendMessage(text)
	}

	// 分割长消息
	parts := splitLongMessage(text, MaxMessageLength)
	for i, part := range parts {
		if i > 0 {
			time.Sleep(100 * time.Millisecond) // 避免发送过快
		}
		if err := t.SendMessage(part); err != nil {
			return fmt.Errorf("发送消息第%d部分失败: %v", i+1, err)
		}
	}
	return nil
}

// 分割长消息
func splitLongMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var parts []string
	lines := strings.Split(text, "\n")
	currentPart := ""

	for i := range lines {
		line := lines[i]
		if len(line) > maxLen {
			if currentPart != "" {
				parts = append(parts, currentPart)
				currentPart = ""
			}
			for len(line) > maxLen {
				parts = append(parts, line[:maxLen])
				line = line[maxLen:]
			}
			if line != "" {
				currentPart = line
			}
			continue
		}

		testPart := currentPart
		if testPart != "" {
			testPart += "\n"
		}
		testPart += line

		if len(testPart) > maxLen {
			if currentPart != "" {
				parts = append(parts, currentPart)
			}
			currentPart = line
		} else {
			currentPart = testPart
		}
	}

	if currentPart != "" {
		parts = append(parts, currentPart)
	}

	return parts
}

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
		return fmt.Errorf("telegram客户端未初始化")
	}

	if len(text) > MaxMessageLength {
		return t.sendMessageSafely(text)
	}

	msg := tgbotapi.NewMessage(t.chatID, text)
	msg.ParseMode = "Markdown"

	_, err := t.bot.Send(msg)
	if err != nil {
		return fmt.Errorf("发送消息失败: %v", err)
	}

	return nil
}

// SendError 发送错误通知
func (t *TelegramClient) SendError(operation string, err error) error {
	message := fmt.Sprintf("❌ ERROR %s\n\n错误详情: %v", operation, err)

	return t.SendMessage(message)
}

// SendServiceStatus 发送服务状态通知
func (t *TelegramClient) SendServiceStatus(status, message string) error {
	statusMap := map[string]string{
		"starting": "启动中",
		"started":  "已启动",
		"stopping": "停止中",
		"stopped":  "已停止",
		"error":    "错误",
	}

	statusText, exists := statusMap[status]
	if !exists {
		statusText = "信息"
	}

	text := fmt.Sprintf(`%s

%s

时间: %s`, statusText, message, formatCreationTime(time.Now()))

	return t.SendMessage(text)
}
