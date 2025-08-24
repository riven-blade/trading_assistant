package auth

import (
	"fmt"
	"time"
	"trading_assistant/pkg/config"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// GenerateToken 生成JWT token
func GenerateToken(username string) (string, error) {
	claims := Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)), // 24小时过期
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "trading-assistant",
			Subject:   username,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(config.GlobalConfig.JWTSecret))
	if err != nil {
		return "", fmt.Errorf("生成token失败: %v", err)
	}

	return tokenString, nil
}

// ValidateToken 验证JWT token
func ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// 确保token方法符合预期
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("无效的签名方法: %v", token.Header["alg"])
		}
		return []byte(config.GlobalConfig.JWTSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("解析token失败: %v", err)
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("无效的token")
}

// ValidateCredentials 验证用户名密码
func ValidateCredentials(username, password string) bool {
	return username == config.GlobalConfig.AdminUsername &&
		password == config.GlobalConfig.AdminPassword &&
		config.GlobalConfig.AdminPassword != "" // 确保密码不为空
}
