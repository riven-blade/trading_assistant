package exchanges

import (
	"context"
	"fmt"
	"strconv"
	"time"
	"trading_assistant/models"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/sirupsen/logrus"
)

// StartUserData 启动用户数据流
func (bws *BinanceWebSocketManager) StartUserData(handlers *UserDataHandlers) error {
	bws.userDataMu.Lock()
	defer bws.userDataMu.Unlock()

	if bws.userDataRunning {
		return fmt.Errorf("用户数据流已在运行")
	}

	if handlers == nil {
		return fmt.Errorf("处理器不能为空")
	}

	bws.userDataHandlers = handlers

	// 启动用户数据流
	if err := bws.startUserDataStream(); err != nil {
		return err
	}

	bws.userDataRunning = true
	bws.userDataReconnectAttempts = 0

	logrus.Info("用户数据流已启动")
	return nil
}

// startUserDataStream 启动用户数据流 (带重连机制)
func (bws *BinanceWebSocketManager) startUserDataStream() error {
	if GlobalBinanceClient == nil {
		return fmt.Errorf("Binance客户端未初始化")
	}

	// 创建监听key
	listenKey, err := GlobalBinanceClient.client.NewStartUserStreamService().Do(context.Background())
	if err != nil {
		return fmt.Errorf("创建用户数据流失败: %v", err)
	}

	bws.userDataListenKey = listenKey

	// 启动keepalive协程
	go bws.keepAliveUserDataStream()

	// 启动监听协程 (带重连)
	go bws.listenUserDataStreamWithReconnect()

	return nil
}

// keepAliveUserDataStream 保持用户数据流活跃
func (bws *BinanceWebSocketManager) keepAliveUserDataStream() {
	ticker := time.NewTicker(30 * time.Minute) // 每30分钟发送一次keepalive
	defer ticker.Stop()

	for {
		select {
		case <-bws.stopChan:
			return
		case <-ticker.C:
			if GlobalBinanceClient != nil && bws.userDataListenKey != "" {
				err := GlobalBinanceClient.client.NewKeepaliveUserStreamService().ListenKey(bws.userDataListenKey).Do(context.Background())
				if err != nil {
					logrus.Errorf("保持用户数据流活跃失败: %v", err)
				} else {
					logrus.Debug("用户数据流keepalive成功")
				}
			}
		}
	}
}

// listenUserDataStreamWithReconnect 监听用户数据流
func (bws *BinanceWebSocketManager) listenUserDataStreamWithReconnect() {
	for {
		select {
		case <-bws.stopChan:
			return
		default:
		}

		bws.userDataMu.RLock()
		if !bws.userDataRunning {
			bws.userDataMu.RUnlock()
			return
		}
		listenKey := bws.userDataListenKey
		handlers := bws.userDataHandlers
		bws.userDataMu.RUnlock()

		if listenKey == "" || handlers == nil {
			time.Sleep(5 * time.Second)
			continue
		}

		// 启动监听
		err := bws.listenUserDataStream(listenKey, handlers)
		if err != nil {
			logrus.Errorf("用户数据流监听失败: %v", err)

			// 处理错误
			if handlers.OnError != nil {
				handlers.OnError(err)
			}

			// 等待后重试
			bws.userDataReconnectAttempts++
			if bws.userDataReconnectAttempts >= bws.config.MaxReconnectAttempts {
				logrus.Errorf("用户数据流重连次数过多，暂停60秒...")
				time.Sleep(60 * time.Second)
				bws.userDataReconnectAttempts = 0
			} else {
				backoff := time.Duration(bws.userDataReconnectAttempts) * time.Duration(bws.config.ReconnectInterval) * time.Second
				logrus.Infof("用户数据流将在 %v 后重连...", backoff)
				time.Sleep(backoff)
			}

			// 尝试重新创建listenKey
			if GlobalBinanceClient != nil {
				newListenKey, err := GlobalBinanceClient.client.NewStartUserStreamService().Do(context.Background())
				if err != nil {
					logrus.Errorf("重新创建用户数据流失败: %v", err)
				} else {
					bws.userDataMu.Lock()
					bws.userDataListenKey = newListenKey
					bws.userDataMu.Unlock()
					logrus.Info("已重新创建用户数据流ListenKey")
				}
			}
		} else {
			// 正常退出，重置重连计数
			bws.userDataReconnectAttempts = 0
		}
	}
}

// listenUserDataStream 监听用户数据流
func (bws *BinanceWebSocketManager) listenUserDataStream(listenKey string, handlers *UserDataHandlers) error {
	wsHandler := func(event *futures.WsUserDataEvent) {
		switch event.Event {
		case "ACCOUNT_UPDATE":
			// 只处理持仓更新，忽略余额变化
			if event.AccountUpdate.Positions != nil && handlers.OnPosition != nil {
				// 处理持仓更新
				for _, position := range event.AccountUpdate.Positions {
					size, _ := strconv.ParseFloat(position.Amount, 64)
					entryPrice, _ := strconv.ParseFloat(position.EntryPrice, 64)
					unrealizedPnl, _ := strconv.ParseFloat(position.UnrealizedPnL, 64)

					if size != 0 { // 只处理有持仓的
						pos := &models.Position{
							Symbol:        position.Symbol,
							Side:          string(position.Side),
							Size:          size,
							EntryPrice:    entryPrice,
							UnrealizedPnl: unrealizedPnl,
							UpdatedAt:     time.Now(),
						}
						handlers.OnPosition(pos)
					}
				}
			}

		case "ORDER_TRADE_UPDATE":
			// 处理订单更新事件
			if handlers.OnOrder != nil {
				order := bws.parseOrderUpdate(event)
				if order != nil {
					logrus.Infof("订单状态更新 %s %s [%s]", order.Symbol, order.ID, order.Status)
					handlers.OnOrder(order)
				}
			}
		}
	}

	errHandler := func(err error) {
		logrus.Errorf("用户数据流WebSocket错误: %v", err)
		if handlers.OnError != nil {
			handlers.OnError(err)
		}
	}

	doneC, _, err := futures.WsUserDataServe(listenKey, wsHandler, errHandler)
	if err != nil {
		return fmt.Errorf("启动用户数据流WebSocket失败: %v", err)
	}

	logrus.Info("用户数据流WebSocket已启动")

	// 等待连接关闭
	<-doneC
	logrus.Info("用户数据流WebSocket连接已关闭")

	return nil // 正常关闭
}

// parseOrderUpdate 解析订单更新事件
func (bws *BinanceWebSocketManager) parseOrderUpdate(event *futures.WsUserDataEvent) *models.Order {
	if event.OrderTradeUpdate.Symbol == "" {
		return nil
	}

	// 基础订单信息
	price, _ := strconv.ParseFloat(event.OrderTradeUpdate.OriginalPrice, 64)
	quantity, _ := strconv.ParseFloat(event.OrderTradeUpdate.OriginalQty, 64)
	executedQty, _ := strconv.ParseFloat(event.OrderTradeUpdate.AccumulatedFilledQty, 64)

	return &models.Order{
		ID:          fmt.Sprintf("%d", event.OrderTradeUpdate.ID),
		Symbol:      event.OrderTradeUpdate.Symbol,
		Side:        string(event.OrderTradeUpdate.Side),
		Type:        string(event.OrderTradeUpdate.Type),
		Quantity:    quantity,
		ExecutedQty: executedQty,
		Price:       price,
		Status:      string(event.OrderTradeUpdate.Status),
		ExchangeID:  fmt.Sprintf("%d", event.OrderTradeUpdate.ID),
		CreatedAt:   time.Unix(event.OrderTradeUpdate.TradeTime/1000, 0),
		UpdatedAt:   time.Now(),
	}
}
