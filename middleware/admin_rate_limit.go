package middleware

import (
	"context"
	"fmt"
	"time"

	"vidbot-api/pkg/cache"
	"vidbot-api/pkg/response"

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
			response.Abort(c, response.ErrAdminRateLimit)
			return
		}

		c.Next()
	}
}
