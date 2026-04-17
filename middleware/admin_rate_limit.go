package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"vidbot-api/pkg/cache"

	"github.com/gin-gonic/gin"
)

func AdminLoginRateLimit(limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		ua := c.GetHeader("User-Agent")

		// kombinasi IP + UA biar lebih ketat
		key := fmt.Sprintf("rl:admin:login:%s:%s", ip, ua)

		ctx := context.Background()

		count, err := cache.Incr(ctx, key)
		if err != nil {
			// fail-open: kalau Redis error, jangan block login
			c.Next()
			return
		}

		if count == 1 {
			_ = cache.Expire(ctx, key, window)
		}

		if count > int64(limit) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"code":    "RATE_LIMIT_EXCEEDED",
				"message": "Too many login attempts. Please try again later.",
			})
			return
		}

		c.Next()
	}
}
