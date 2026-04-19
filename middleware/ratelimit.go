package middleware

import (
	"vidbot-api/pkg/apikey"
	"vidbot-api/pkg/limiter"
	"vidbot-api/pkg/response"

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
			response.Abort(c, response.ErrRateLimitExceeded)
			return
		}

		c.Next()
	}
}
