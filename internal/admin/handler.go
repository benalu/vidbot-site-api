package admin

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"vidbot-api/pkg/apikey"
	"vidbot-api/pkg/cache"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	masterKey string
}

func NewHandler(masterKey string) *Handler {
	return &Handler{masterKey: masterKey}
}

type CreateKeyRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Quota int    `json:"quota"`
}

func (h *Handler) validateMasterKey(c *gin.Context) bool {
	if c.GetHeader("X-Master-Key") != h.masterKey {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"code":    "UNAUTHORIZED",
			"message": "Invalid master key.",
		})
		return false
	}
	return true
}

func (h *Handler) CreateKey(c *gin.Context) {
	if !h.validateMasterKey(c) {
		return
	}

	var req CreateKeyRequest
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    "BAD_REQUEST",
			"message": err.Error(),
		})
		return
	}

	if req.Name == "" || req.Email == "" || req.Quota < 1 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    "BAD_REQUEST",
			"message": "name, email, dan quota wajib diisi.",
		})
		return
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate key"})
		return
	}
	plainKey := hex.EncodeToString(raw)

	hash := sha256.Sum256([]byte(plainKey))
	keyHash := hex.EncodeToString(hash[:])

	data := apikey.Data{
		KeyHash:   keyHash,
		Name:      req.Name,
		Email:     req.Email,
		Active:    true,
		Quota:     req.Quota,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	jsonData, _ := json.Marshal(data)
	ctx := context.Background()

	if err := cache.Set(ctx, fmt.Sprintf("apikeys:%s", keyHash), string(jsonData), 0); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save key"})
		return
	}

	cache.SAdd(ctx, "apikeys:index", keyHash)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "API key created successfully.",
		"data": gin.H{
			"api_key":    plainKey,
			"name":       data.Name,
			"email":      data.Email,
			"active":     data.Active,
			"quota":      data.Quota,
			"created_at": data.CreatedAt,
		},
	})
}

func (h *Handler) RevokeKey(c *gin.Context) {
	if !h.validateMasterKey(c) {
		return
	}

	if c.Request.ContentLength > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    "BAD_REQUEST",
			"message": "Request body not allowed.",
		})
		return
	}

	plainKey := c.Param("key")
	if plainKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key is required"})
		return
	}

	hash := sha256.Sum256([]byte(plainKey))
	keyHash := hex.EncodeToString(hash[:])
	redisKey := fmt.Sprintf("apikeys:%s", keyHash)

	ctx := context.Background()
	raw, err := cache.Get(ctx, redisKey)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"code":    "NOT_FOUND",
			"message": "API key not found.",
		})
		return
	}

	var data apikey.Data
	json.Unmarshal([]byte(raw), &data)
	data.Active = false

	jsonData, _ := json.Marshal(data)
	cache.Set(ctx, redisKey, string(jsonData), 0)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("API key for '%s' has been revoked.", data.Name),
	})
}

func (h *Handler) ListKeys(c *gin.Context) {
	if !h.validateMasterKey(c) {
		return
	}

	// parse filter
	activeFilter := c.Query("active") // "true", "false", atau "" (semua)

	ctx := context.Background()
	keyHashes, err := cache.SMembers(ctx, "apikeys:index")
	if err != nil || len(keyHashes) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": true, "total": 0, "data": []gin.H{}})
		return
	}

	result := []gin.H{}
	for _, keyHash := range keyHashes {
		raw, err := cache.Get(ctx, fmt.Sprintf("apikeys:%s", keyHash))
		if err != nil {
			continue
		}
		var data apikey.Data
		json.Unmarshal([]byte(raw), &data)

		// apply filter
		if activeFilter == "true" && !data.Active {
			continue
		}
		if activeFilter == "false" && data.Active {
			continue
		}

		result = append(result, gin.H{
			"key_hash":   data.KeyHash[:8] + "...",
			"name":       data.Name,
			"email":      data.Email,
			"active":     data.Active,
			"quota":      data.Quota,
			"created_at": data.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"total":   len(result),
		"data":    result,
	})
}

func (h *Handler) TopUpQuota(c *gin.Context) {
	if !h.validateMasterKey(c) {
		return
	}

	apiKey := c.Param("key")
	var req struct {
		Amount int `json:"amount" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount is required and must be positive"})
		return
	}

	hash := sha256.Sum256([]byte(apiKey))
	keyHash := hex.EncodeToString(hash[:])
	redisKey := fmt.Sprintf("apikeys:%s", keyHash)

	ctx := context.Background()
	raw, err := cache.Get(ctx, redisKey)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "key not found"})
		return
	}

	var data apikey.Data
	json.Unmarshal([]byte(raw), &data)

	oldQuota := data.Quota
	data.Quota += req.Amount

	jsonData, _ := json.Marshal(data)
	cache.Set(ctx, redisKey, string(jsonData), 0)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Quota top-up successful for '%s'", data.Name),
		"data": gin.H{
			"name":      data.Name,
			"old_quota": oldQuota,
			"added":     req.Amount,
			"new_quota": data.Quota,
		},
	})
}
