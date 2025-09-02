package controllers

import (
	"net/http"
	"trading_assistant/core"
	"trading_assistant/models"
	"trading_assistant/pkg/exchanges/binance"
	"trading_assistant/pkg/redis"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type CoinController struct {
	binanceClient *binance.Binance
	marketManager *core.MarketManager
}

// NewCoinController 创建币种控制器
func NewCoinController(binanceClient *binance.Binance, marketManager *core.MarketManager) *CoinController {
	return &CoinController{
		binanceClient: binanceClient,
		marketManager: marketManager,
	}
}

// SelectCoin 筛选币种
func (c *CoinController) SelectCoin(ctx *gin.Context) {
	var req struct {
		Symbol     string `json:"symbol" binding:"required"`
		IsSelected bool   `json:"is_selected"`
	}

	if err := ctx.ShouldBindJSON(&req); err != nil {
		logrus.Warnf("币种选择参数错误: %v", err)
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数格式错误",
		})
		return
	}

	// 验证币种是否存在
	coin, err := redis.GlobalRedisClient.GetCoin(req.Symbol)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{
			"error": "币种不存在，请先同步币种数据",
		})
		return
	}

	// 更新选择状态（使用专门的选择状态管理）
	var status string
	if req.IsSelected {
		status = models.CoinSelectionActive
	} else {
		status = models.CoinSelectionInactive
	}

	err = redis.GlobalRedisClient.SetCoinSelection(req.Symbol, status)
	if err != nil {
		logrus.Errorf("更新币种选择状态失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "更新币种选择状态失败",
		})
		return
	}

	if req.IsSelected {
		logrus.Infof("币种 %s 已标记为选中", req.Symbol)
	} else {
		logrus.Infof("币种 %s 已取消选中", req.Symbol)
	}

	// 刷新价格管理器的选中币种缓存
	if c.marketManager != nil {
		c.marketManager.RefreshSelectedCoinsCache()
	}

	// 获取选择状态用于响应
	selection, _ := redis.GlobalRedisClient.GetCoinSelection(req.Symbol)

	// 返回响应
	response := gin.H{
		"message": "币种选择状态更新成功",
		"data": gin.H{
			"coin":        coin,
			"selection":   selection,
			"is_selected": req.IsSelected,
		},
	}

	// 如果启用了价格管理器，返回全局订阅状态信息
	if c.marketManager != nil {
		priceStatus := c.marketManager.GetPriceSubscriptionStatus()
		response["price_subscriptions"] = gin.H{
			"mode":        priceStatus["mode"],
			"running":     priceStatus["running"],
			"subscribed":  priceStatus["subscribed"],
			"stream_name": priceStatus["stream_name"],
		}
	}

	ctx.JSON(http.StatusOK, response)
}

// SyncCoins 从交易所同步币种列表和价格数据
func (c *CoinController) SyncCoins(ctx *gin.Context) {
	if c.marketManager == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "市场数据管理器未初始化",
		})
		return
	}

	logrus.Info("开始同步币种列表和价格数据...")

	// 使用统一的同步方法
	if err := c.marketManager.SyncMarketAndPriceData(); err != nil {
		logrus.Errorf("同步市场数据和价格数据失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "同步市场数据和价格数据失败: " + err.Error(),
		})
		return
	}

	// 获取同步后的币种数量
	coins, err := redis.GlobalRedisClient.GetAllCoins()
	if err != nil {
		logrus.Errorf("获取币种数量失败: %v", err)
		ctx.JSON(http.StatusOK, gin.H{
			"message": "同步完成，但获取币种数量失败",
		})
		return
	}

	logrus.Infof("币种和价格数据同步完成，共 %d 个币种", len(coins))

	ctx.JSON(http.StatusOK, gin.H{
		"message": "币种和价格数据同步完成",
		"count":   len(coins),
	})
}

// GetCoins 获取币种列表
func (c *CoinController) GetCoins(ctx *gin.Context) {
	// 从Redis获取所有币种
	coins, err := redis.GlobalRedisClient.GetAllCoins()
	if err != nil {
		logrus.Errorf("获取币种列表失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取币种列表失败",
		})
		return
	}

	// 根据查询参数决定返回内容
	selectedOnly := ctx.Query("selected") == "true"
	if selectedOnly {
		// 使用新的选择状态管理获取选中币种
		selectedCoins, err := redis.GlobalRedisClient.GetSelectedCoins()
		if err != nil {
			logrus.Errorf("获取选中币种列表失败: %v", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"error": "获取选中币种列表失败",
			})
			return
		}
		coins = selectedCoins
	}

	// 如果需要包含选择状态信息
	includeSelection := ctx.Query("include_selection") == "true"
	if includeSelection {
		// 为每个币种添加选择状态信息
		type CoinWithSelection struct {
			*models.Coin
			IsSelected bool `json:"is_selected"`
		}

		var coinsWithSelection []CoinWithSelection
		for _, coin := range coins {
			isSelected := redis.GlobalRedisClient.IsCoinSelected(coin.Symbol)
			coinsWithSelection = append(coinsWithSelection, CoinWithSelection{
				Coin:       coin,
				IsSelected: isSelected,
			})
		}

		ctx.JSON(http.StatusOK, gin.H{
			"data":  coinsWithSelection,
			"count": len(coinsWithSelection),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data":  coins,
		"count": len(coins),
	})
}

// GetSelectedCoins 获取选中的币种列表
func (c *CoinController) GetSelectedCoins(ctx *gin.Context) {
	// 获取选中的币种
	selectedCoins, err := redis.GlobalRedisClient.GetSelectedCoins()
	if err != nil {
		logrus.Errorf("获取选中币种列表失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "获取选中币种列表失败",
		})
		return
	}

	var result []models.CoinWithSelection
	for i := range selectedCoins {
		coin := selectedCoins[i]
		result = append(result, models.CoinWithSelection{
			Coin:       *coin,
			IsSelected: true,
		})
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data":  result,
		"count": len(result),
	})
}
