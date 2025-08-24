package exchanges

import (
	"context"
	"fmt"
	"trading_assistant/pkg/config"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/sirupsen/logrus"
)

type BinanceClient struct {
	client *futures.Client
}

var GlobalBinanceClient *BinanceClient

func init() {
	// 初始化全局WebSocket管理器
	GlobalWebSocketManager = NewBinanceWebSocketManager(nil)
}

// InitBinance 初始化Binance客户端
func InitBinance() error {
	client := futures.NewClient(config.GlobalConfig.BinanceAPIKey, config.GlobalConfig.BinanceSecretKey)

	// 如果是测试网
	if config.GlobalConfig.BinanceTestnet {
		client.BaseURL = "https://testnet.binancefuture.com"
	}

	GlobalBinanceClient = &BinanceClient{
		client: client,
	}

	// 测试连接
	_, err := client.NewServerTimeService().Do(context.Background())
	if err != nil {
		return fmt.Errorf("binance连接失败: %v", err)
	}

	// 设置持仓模式
	err = GlobalBinanceClient.SetPositionMode()
	if err != nil {
		logrus.Warnf("设置持仓模式失败: %v", err)
	}

	logrus.Info("Binance连接成功")
	return nil
}

// SetPositionMode 设置持仓模式
func (b *BinanceClient) SetPositionMode() error {
	dualSide := config.GlobalConfig.PositionMode == "both"

	err := b.client.NewChangePositionModeService().DualSide(dualSide).Do(context.Background())
	if err != nil {
		// 如果持仓模式已经设置过，会返回错误，但不影响功能
		logrus.Debugf("持仓模式设置响应: %v", err)
	}

	logrus.Infof("持仓模式设置为: %s", config.GlobalConfig.PositionMode)
	return nil
}

// SetSymbolMarginMode 设置指定交易对的保证金模式
func (b *BinanceClient) SetSymbolMarginMode(symbol, marginMode string) error {
	var marginType futures.MarginType
	if marginMode == "isolated" {
		marginType = futures.MarginTypeIsolated
	} else {
		marginType = futures.MarginTypeCrossed
	}

	err := b.client.NewChangeMarginTypeService().
		Symbol(symbol).
		MarginType(marginType).
		Do(context.Background())

	if err != nil {
		// 如果保证金模式已经设置过，会返回错误，但不影响功能
		logrus.Debugf("设置 %s 保证金模式为 %s 响应: %v", symbol, marginMode, err)
		return err
	}

	logrus.Infof("成功设置 %s 保证金模式为: %s", symbol, marginMode)
	return nil
}

// GetMarginMode 获取指定交易对的保证金模式
func (b *BinanceClient) GetMarginMode(symbol string) (string, error) {
	positions, err := b.client.NewGetPositionRiskService().Symbol(symbol).Do(context.Background())
	if err != nil {
		return "", fmt.Errorf("获取持仓风险失败: %v", err)
	}

	if len(positions) > 0 {
		if positions[0].MarginType == "isolated" {
			return "isolated", nil
		} else {
			return "cross", nil
		}
	}

	return "unknown", nil
}
