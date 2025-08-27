package middleware

import (
	"net/http"
	"strings"
	"trading_assistant/pkg/auth"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// AuthMiddleware JWT认证中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 跳过健康检查、登录接口和静态文件
		path := c.Request.URL.Path
		if path == "/health" ||
			path == "/api/v1/auth/login" ||
			strings.HasPrefix(path, "/static/") ||
			path == "/favicon.ico" ||
			path == "/favicon.svg" ||
			path == "/manifest.json" ||
			path == "/" ||
			(!strings.HasPrefix(path, "/api/") && path != "/ws") {
			c.Next()
			return
		}

		var tokenString string
		if path == "/ws" {
			tokenString = c.Query("token")
			if tokenString == "" {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "缺少token参数",
					"code":  "MISSING_TOKEN_PARAM",
				})
				c.Abort()
				return
			}
		} else {
			// 其他接口从Authorization头获取token
			authHeader := c.GetHeader("Authorization")
			if authHeader == "" {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "缺少Authorization头",
					"code":  "MISSING_AUTH_HEADER",
				})
				c.Abort()
				return
			}

			// 检查Bearer token格式
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenString = strings.TrimPrefix(authHeader, "Bearer ")
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "无效的Authorization格式，应为 'Bearer <token>'",
					"code":  "INVALID_AUTH_FORMAT",
				})
				c.Abort()
				return
			}
		}

		// 验证token
		claims, err := auth.ValidateToken(tokenString)
		if err != nil {
			logrus.Warnf("Token验证失败: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "无效的token",
				"code":  "INVALID_TOKEN",
			})
			c.Abort()
			return
		}

		// 将用户信息存储到上下文中
		c.Set("username", claims.Username)
		c.Next()
	}
}

// GetCurrentUser 从上下文中获取当前用户
func GetCurrentUser(c *gin.Context) string {
	if username, exists := c.Get("username"); exists {
		return username.(string)
	}
	return ""
}
