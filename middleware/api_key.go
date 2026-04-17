package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"vidbot-api/pkg/apikey"
	"vidbot-api/pkg/cache"

	"github.com/gin-gonic/gin"
)

func RequireAPIKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-API-Key")
		if key == "" {
			c.AbortWithStatusJSON(401, gin.H{
				"success": false,
				"code":    "UNAUTHORIZED",
				"message": "API key is required.",
			})
			return
		}

		hash := sha256.Sum256([]byte(key))
		keyHash := hex.EncodeToString(hash[:])

		ctx := context.Background()
		raw, err := cache.Get(ctx, fmt.Sprintf("apikeys:%s", keyHash))
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{
				"success": false,
				"code":    "UNAUTHORIZED",
				"message": "Invalid API key.",
			})
			return
		}

		var data apikey.Data
		if err := json.Unmarshal([]byte(raw), &data); err != nil || !data.Active {
			c.AbortWithStatusJSON(401, gin.H{
				"success": false,
				"code":    "UNAUTHORIZED",
				"message": "API key is inactive or invalid.",
			})
			return
		}

		quotaKey := fmt.Sprintf("apikeys:quota:%s", keyHash)
		allowed, err := cache.AtomicQuotaCheck(ctx, quotaKey, data.Quota)
		if err != nil || !allowed {
			c.AbortWithStatusJSON(429, gin.H{
				"success": false,
				"code":    "QUOTA_EXCEEDED",
				"message": "Quota habis, silakan top-up.",
			})
			return
		}

		c.Set("api_key_data", data)
		c.Next()
	}
}

func RequireAPIKeyFromQuery() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Query("key")
		if key == "" {
			c.AbortWithStatusJSON(401, gin.H{
				"success": false,
				"code":    "UNAUTHORIZED",
				"message": "API key is required.",
			})
			return
		}

		hash := sha256.Sum256([]byte(key))
		keyHash := hex.EncodeToString(hash[:])

		ctx := context.Background()
		raw, err := cache.Get(ctx, fmt.Sprintf("apikeys:%s", keyHash))
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{
				"success": false,
				"code":    "UNAUTHORIZED",
				"message": "Invalid API key.",
			})
			return
		}

		var data apikey.Data
		if err := json.Unmarshal([]byte(raw), &data); err != nil || !data.Active {
			c.AbortWithStatusJSON(401, gin.H{
				"success": false,
				"code":    "UNAUTHORIZED",
				"message": "API key is inactive or invalid.",
			})
			return
		}

		quotaKey := fmt.Sprintf("apikeys:quota:%s", keyHash)
		allowed, err := cache.AtomicQuotaCheck(ctx, quotaKey, data.Quota)
		if err != nil || !allowed {
			c.AbortWithStatusJSON(429, gin.H{
				"success": false,
				"code":    "QUOTA_EXCEEDED",
				"message": "Quota habis, silakan top-up.",
			})
			return
		}
		c.Set("api_key_data", data)
		c.Next()
	}
}
