package middleware

import (
	"context"
	"fmt"
	"vidbot-api/pkg/cache"

	"github.com/gin-gonic/gin"
)

type featureErrorResponse struct {
	Success bool   `json:"success"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func FeatureFlag(group string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := context.Background()
		val, err := cache.Get(ctx, fmt.Sprintf("feature:%s", group))
		if err == nil && val == "off" {
			c.AbortWithStatusJSON(503, featureErrorResponse{
				Success: false,
				Code:    "SERVICE_UNAVAILABLE",
				Message: "This service is temporarily unavailable for maintenance. Please try again later.",
			})
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
			c.AbortWithStatusJSON(503, featureErrorResponse{
				Success: false,
				Code:    "SERVICE_UNAVAILABLE",
				Message: "This service is temporarily unavailable for maintenance. Please try again later.",
			})
			return
		}

		// cek platform level
		platformVal, err := cache.Get(ctx, fmt.Sprintf("feature:%s:%s", group, platform))
		if err == nil && platformVal == "off" {
			c.AbortWithStatusJSON(503, featureErrorResponse{
				Success: false,
				Code:    "SERVICE_UNAVAILABLE",
				Message: fmt.Sprintf("The '%s' service is temporarily unavailable for maintenance. Please try again later.", platform),
			})
			return
		}

		c.Next()
	}
}
