package middleware

import (
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

		// pakai in-memory cache, fallback ke Redis
		raw, err := cache.GetAPIKey(keyHash)
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
		// quota check tetap ke Redis — harus konsisten antar instance
		allowed, err := cache.AtomicQuotaCheck(
			c.Request.Context(), quotaKey, data.Quota,
		)
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

		raw, err := cache.GetAPIKey(keyHash)
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
		allowed, err := cache.AtomicQuotaCheck(
			c.Request.Context(), quotaKey, data.Quota,
		)
		if err != nil || !allowed {
			response.Abort(c, response.ErrQuotaExceeded)
			return
		}

		c.Set("api_key_data", data)
		c.Next()
	}
}
