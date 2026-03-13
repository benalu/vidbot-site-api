package middleware

import (
	"vidbot-api/pkg/apikey"
	"vidbot-api/pkg/limiter"

	"github.com/gin-gonic/gin"
)

func RateLimit(group string) gin.HandlerFunc {
	return func(c *gin.Context) {
		data, exists := c.Get("api_key_data")
		if !exists {
			c.Next()
			return
		}

		keyData, ok := data.(apikey.Data)
		if !ok {
			c.Next()
			return
		}

		keyHash := keyData.KeyHash

		allowed, err := limiter.CheckRateLimit(keyHash, group)
		if err != nil || !allowed {
			c.AbortWithStatusJSON(429, gin.H{
				"success": false,
				"code":    "RATE_LIMIT_EXCEEDED",
				"message": "Terlalu banyak request, coba lagi dalam 1 menit.",
			})
			return
		}

		c.Next()
	}
}
