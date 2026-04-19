package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"vidbot-api/pkg/apikey"
	"vidbot-api/pkg/cache"
	"vidbot-api/pkg/response"

	"github.com/gin-gonic/gin"
)

func RequireAPIKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-API-Key")
		if key == "" {
			response.Abort(c, response.ErrAPIKeyMissing)
			return
		}

		hash := sha256.Sum256([]byte(key))
		keyHash := hex.EncodeToString(hash[:])

		ctx := context.Background()
		raw, err := cache.Get(ctx, fmt.Sprintf("apikeys:%s", keyHash))
		if err != nil {
			response.Abort(c, response.ErrAPIKeyNotFound)
			return
		}

		var data apikey.Data
		if err := json.Unmarshal([]byte(raw), &data); err != nil || !data.Active {
			response.Abort(c, response.ErrAPIKeyInactive)
			return
		}

		quotaKey := fmt.Sprintf("apikeys:quota:%s", keyHash)
		allowed, err := cache.AtomicQuotaCheck(ctx, quotaKey, data.Quota)
		if err != nil || !allowed {
			response.Abort(c, response.ErrQuotaExceeded)
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
			response.Abort(c, response.ErrAPIKeyMissing)
			return
		}

		hash := sha256.Sum256([]byte(key))
		keyHash := hex.EncodeToString(hash[:])

		ctx := context.Background()
		raw, err := cache.Get(ctx, fmt.Sprintf("apikeys:%s", keyHash))
		if err != nil {
			response.Abort(c, response.ErrAPIKeyNotFound)
			return
		}

		var data apikey.Data
		if err := json.Unmarshal([]byte(raw), &data); err != nil || !data.Active {
			response.Abort(c, response.ErrAPIKeyInactive)
			return
		}

		quotaKey := fmt.Sprintf("apikeys:quota:%s", keyHash)
		allowed, err := cache.AtomicQuotaCheck(ctx, quotaKey, data.Quota)
		if err != nil || !allowed {
			response.Abort(c, response.ErrQuotaExceeded)
			return
		}
		c.Set("api_key_data", data)
		c.Next()
	}
}
