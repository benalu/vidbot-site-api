package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"vidbot-api/pkg/apikey"
	"vidbot-api/pkg/cache"
	"vidbot-api/pkg/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	MagicString string
}

func (h *Handler) Verify(c *gin.Context) {
	key := c.GetHeader("X-API-Key")
	if key == "" {
		response.Error(c, 401, "invalid api key")
		return
	}

	hash := sha256.Sum256([]byte(key))
	keyHash := hex.EncodeToString(hash[:])

	raw, err := cache.Get(context.Background(), fmt.Sprintf("apikeys:%s", keyHash))
	if err != nil {
		response.Error(c, 401, "invalid api key")
		return
	}

	var data apikey.Data
	json.Unmarshal([]byte(raw), &data)
	if !data.Active {
		response.Error(c, 401, "api key inactive")
		return
	}

	token := GenerateToken(keyHash, h.MagicString)
	c.JSON(200, gin.H{
		"success":      true,
		"access_token": token,
	})
}

func (h *Handler) Quota(c *gin.Context) {
	if c.Request.ContentLength > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    "BAD_REQUEST",
			"message": "Request body not allowed.",
		})
		return
	}

	rawData, _ := c.Get("api_key_data")
	data := rawData.(apikey.Data)

	ctx := context.Background()
	quotaUsedStr, _ := cache.Get(ctx, fmt.Sprintf("apikeys:quota:%s", data.KeyHash))

	used := 0
	fmt.Sscanf(quotaUsedStr, "%d", &used)

	c.JSON(200, gin.H{
		"success": true,
		"data": gin.H{
			"name":            data.Name,
			"email":           data.Email,
			"quota":           data.Quota,
			"quota_used":      used,
			"quota_remaining": data.Quota - used,
		},
	})
}
