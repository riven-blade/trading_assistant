package controllers

import (
	"net/http"
	"trading_assistant/pkg/auth"
	"trading_assistant/pkg/config"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type AuthController struct{}

// LoginRequest 登录请求结构
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应结构
type LoginResponse struct {
	Token     string `json:"token"`
	Username  string `json:"username"`
	ExpiresIn int    `json:"expires_in"` // 过期时间（秒）
}

// Login 用户登录
func (a *AuthController) Login(ctx *gin.Context) {
	var req LoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		logrus.Warnf("登录参数错误: %v", err)
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数格式错误",
			"code":  "INVALID_PARAMS",
		})
		return
	}

	// 检查管理员密码是否已配置
	if config.GlobalConfig.AdminPassword == "" {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "系统未配置管理员密码，请联系管理员",
			"code":  "PASSWORD_NOT_CONFIGURED",
		})
		return
	}

	// 验证用户名密码
	if !auth.ValidateCredentials(req.Username, req.Password) {
		logrus.Warnf("登录失败: 用户名或密码错误 - %s", req.Username)
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "用户名或密码错误",
			"code":  "INVALID_CREDENTIALS",
		})
		return
	}

	// 生成JWT token
	token, err := auth.GenerateToken(req.Username)
	if err != nil {
		logrus.Errorf("生成token失败: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "生成认证token失败",
			"code":  "TOKEN_GENERATION_FAILED",
		})
		return
	}

	logrus.Infof("用户登录成功: %s", req.Username)

	// 返回token
	ctx.JSON(http.StatusOK, gin.H{
		"message": "登录成功",
		"data": LoginResponse{
			Token:     token,
			Username:  req.Username,
			ExpiresIn: 24 * 3600, // 24小时
		},
	})
}

// GetProfile 获取用户信息
func (a *AuthController) GetProfile(ctx *gin.Context) {
	username := ctx.GetString("username")

	ctx.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"username": username,
			"role":     "admin",
		},
	})
}
