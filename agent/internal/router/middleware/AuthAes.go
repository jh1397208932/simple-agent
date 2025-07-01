package middleware

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
)

var AesKey string

func AuthMiddleware(aesKey string) gin.HandlerFunc {
	AesKey = aesKey
	return func(c *gin.Context) {
		// 1. 获取加密token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "未提供认证信息"})
			return
		}

		c.Next()
	}
}
