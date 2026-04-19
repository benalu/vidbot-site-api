package middleware

import (
	"context"
	"fmt"
	"vidbot-api/pkg/cache"
	"vidbot-api/pkg/response"

	"github.com/gin-gonic/gin"
)

func FeatureFlag(group string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		val, err := cache.Get(ctx, fmt.Sprintf("feature:%s", group))
		if err == nil && val == "off" {
			response.Abort(c, response.ErrServiceUnavailable)
			return
		}
		c.Next()
	}
}

func FeatureFlagPlatform(group, platform string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()

		// cek group level dulu
		groupVal, err := cache.Get(ctx, fmt.Sprintf("feature:%s", group))
		if err == nil && groupVal == "off" {
			response.AbortMsg(c, response.ErrServiceUnavailable,
				fmt.Sprintf("The '%s' service is temporarily unavailable. Please try again later.", group))
			return
		}

		// cek platform level
		platformVal, err := cache.Get(ctx, fmt.Sprintf("feature:%s:%s", group, platform))
		if err == nil && platformVal == "off" {
			response.AbortMsg(c, response.ErrServiceUnavailable,
				fmt.Sprintf("The '%s' service is temporarily unavailable. Please try again later.", platform))
			return
		}

		c.Next()
	}
}
