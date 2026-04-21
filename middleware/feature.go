package middleware

import (
	"fmt"
	"vidbot-api/pkg/cache"
	"vidbot-api/pkg/response"

	"github.com/gin-gonic/gin"
)

func FeatureFlag(group string) gin.HandlerFunc {
	return func(c *gin.Context) {
		val := cache.GetFeatureFlag(fmt.Sprintf("feature:%s", group))
		if val == "off" {
			response.Abort(c, response.ErrServiceUnavailable)
			return
		}
		c.Next()
	}
}

func FeatureFlagPlatform(group, platform string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// cek group level
		groupVal := cache.GetFeatureFlag(fmt.Sprintf("feature:%s", group))
		if groupVal == "off" {
			response.AbortMsg(c, response.ErrServiceUnavailable,
				fmt.Sprintf("The '%s' service is temporarily unavailable. Please try again later.", group))
			return
		}

		// cek platform level
		platformVal := cache.GetFeatureFlag(fmt.Sprintf("feature:%s:%s", group, platform))
		if platformVal == "off" {
			response.AbortMsg(c, response.ErrServiceUnavailable,
				fmt.Sprintf("The '%s' service is temporarily unavailable. Please try again later.", platform))
			return
		}

		c.Next()
	}
}
