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
	"vidbot-api/pkg/stats"

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

type adminResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
}

type adminMessageResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type adminErrorResponse struct {
	Success bool   `json:"success"`
	Code    string `json:"code"`
	Message string `json:"message"`
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

	activeFilter := c.Query("active")

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

// validGroups — daftar group yang bisa di-toggle
var validGroups = map[string]bool{
	"content":   true,
	"convert":   true,
	"iptv":      true,
	"vidhub":    true,
	"leakcheck": true,
	"app":       true,
}

var validPlatforms = map[string][]string{
	"content": {"spotify", "tiktok", "instagram", "twitter", "threads"},
	"vidhub":  {"videb", "vidoy", "vidbos", "vidarato", "vidnest", "kingbokeptv"},
	"convert": {"audio", "document", "image", "fonts"},
	"app":     {"android", "windows"},
}

func isValidPlatform(group, platform string) bool {
	platforms, ok := validPlatforms[group]
	if !ok {
		return false
	}
	for _, p := range platforms {
		if p == platform {
			return true
		}
	}
	return false
}

func (h *Handler) GetFeatures(c *gin.Context) {
	if !h.validateMasterKey(c) {
		return
	}

	ctx := context.Background()
	result := gin.H{}

	// group dengan platform
	for group, platforms := range validPlatforms {
		groupVal, err := cache.Get(ctx, fmt.Sprintf("feature:%s", group))
		if err != nil {
			groupVal = "on"
		}

		platformStatus := gin.H{}
		for _, platform := range platforms {
			val, err := cache.Get(ctx, fmt.Sprintf("feature:%s:%s", group, platform))
			if err != nil {
				val = "on"
			}
			platformStatus[platform] = val
		}

		result[group] = gin.H{
			"status":    groupVal,
			"platforms": platformStatus,
		}
	}

	// group tanpa platform — dinamis dari validGroups
	for group := range validGroups {
		if _, hasPlatform := validPlatforms[group]; hasPlatform {
			continue
		}
		val, err := cache.Get(ctx, fmt.Sprintf("feature:%s", group))
		if err != nil {
			val = "on"
		}
		result[group] = gin.H{
			"status": val,
		}
	}

	c.JSON(http.StatusOK, adminResponse{
		Success: true,
		Data:    result,
	})
}

func (h *Handler) EnableFeature(c *gin.Context) {
	if !h.validateMasterKey(c) {
		return
	}

	group := c.Param("group")
	if !validGroups[group] {
		c.JSON(http.StatusBadRequest, adminErrorResponse{
			Success: false,
			Code:    "INVALID_GROUP",
			Message: fmt.Sprintf("Group '%s' is not recognized. Valid groups: content, convert, iptv, vidhub.", group),
		})
		return
	}

	ctx := context.Background()
	cache.Set(ctx, fmt.Sprintf("feature:%s", group), "on", 0)

	c.JSON(http.StatusOK, adminMessageResponse{
		Success: true,
		Message: fmt.Sprintf("Feature '%s' is now enabled.", group),
	})
}

func (h *Handler) DisableFeature(c *gin.Context) {
	if !h.validateMasterKey(c) {
		return
	}

	group := c.Param("group")
	if !validGroups[group] {
		c.JSON(http.StatusBadRequest, adminErrorResponse{
			Success: false,
			Code:    "INVALID_GROUP",
			Message: fmt.Sprintf("Group '%s' is not recognized. Valid groups: content, convert, iptv, vidhub.", group),
		})
		return
	}

	ctx := context.Background()
	cache.Set(ctx, fmt.Sprintf("feature:%s", group), "off", 0)

	c.JSON(http.StatusOK, adminMessageResponse{
		Success: true,
		Message: fmt.Sprintf("Feature '%s' is now disabled.", group),
	})
}

func (h *Handler) EnablePlatform(c *gin.Context) {
	if !h.validateMasterKey(c) {
		return
	}

	group := c.Param("group")
	platform := c.Param("platform")

	if !isValidPlatform(group, platform) {
		c.JSON(http.StatusBadRequest, adminErrorResponse{
			Success: false,
			Code:    "INVALID_PLATFORM",
			Message: fmt.Sprintf("Platform '%s' is not valid for group '%s'. Check /admin/features for available platforms.", platform, group),
		})
		return
	}

	ctx := context.Background()
	cache.Set(ctx, fmt.Sprintf("feature:%s:%s", group, platform), "on", 0)

	c.JSON(http.StatusOK, adminMessageResponse{
		Success: true,
		Message: fmt.Sprintf("Platform '%s/%s' is now enabled.", group, platform),
	})
}

func (h *Handler) DisablePlatform(c *gin.Context) {
	if !h.validateMasterKey(c) {
		return
	}

	group := c.Param("group")
	platform := c.Param("platform")

	if !isValidPlatform(group, platform) {
		c.JSON(http.StatusBadRequest, adminErrorResponse{
			Success: false,
			Code:    "INVALID_PLATFORM",
			Message: fmt.Sprintf("Platform '%s' is not valid for group '%s'. Check /admin/features for available platforms.", platform, group),
		})
		return
	}

	ctx := context.Background()
	cache.Set(ctx, fmt.Sprintf("feature:%s:%s", group, platform), "off", 0)

	c.JSON(http.StatusOK, adminMessageResponse{
		Success: true,
		Message: fmt.Sprintf("Platform '%s/%s' is now disabled.", group, platform),
	})
}

func (h *Handler) GetStats(c *gin.Context) {
	if !h.validateMasterKey(c) {
		return
	}

	ctx := context.Background()

	// total keys
	keyHashes, _ := cache.SMembers(ctx, "apikeys:index")
	totalKeys := len(keyHashes)
	activeKeys := 0
	for _, keyHash := range keyHashes {
		raw, err := cache.Get(ctx, fmt.Sprintf("apikeys:%s", keyHash))
		if err != nil {
			continue
		}
		var data apikey.Data
		json.Unmarshal([]byte(raw), &data)
		if data.Active {
			activeKeys++
		}
	}

	// hitung grand total dari semua group
	grandTotalReq := 0
	grandUniqueKeys := 0
	grandTodayReq := 0

	usageData := gin.H{}
	for group := range validGroups {
		totalReq, uniqueKeys := stats.GetGroupStats(group)
		todayReq := stats.GetTodayStats(group)

		grandTotalReq += totalReq
		grandTodayReq += todayReq
		grandUniqueKeys += uniqueKeys

		if platforms, ok := validPlatforms[group]; ok {
			platformData := gin.H{}
			for _, platform := range platforms {
				platReq, _ := stats.GetPlatformStats(group, platform)
				platformData[platform] = platReq
			}
			usageData[group] = gin.H{"platforms": platformData}
		} else {
			// group tanpa platform — tampilkan total request langsung
			usageData[group] = totalReq
		}
	}

	c.JSON(http.StatusOK, adminResponse{
		Success: true,
		Data: gin.H{
			"total_keys":     totalKeys,
			"active_keys":    activeKeys,
			"total_requests": grandTotalReq,
			"today_requests": grandTodayReq,
			"unique_keys":    grandUniqueKeys,
			"usage":          usageData,
		},
	})
}

func (h *Handler) GetKeyUsage(c *gin.Context) {
	if !h.validateMasterKey(c) {
		return
	}

	plainKey := c.Param("key")
	hash := sha256.Sum256([]byte(plainKey))
	keyHash := hex.EncodeToString(hash[:])

	ctx := context.Background()
	raw, err := cache.Get(ctx, fmt.Sprintf("apikeys:%s", keyHash))
	if err != nil {
		c.JSON(http.StatusNotFound, adminErrorResponse{
			Success: false,
			Code:    "NOT_FOUND",
			Message: "API key not found.",
		})
		return
	}

	var data apikey.Data
	json.Unmarshal([]byte(raw), &data)

	quotaUsedStr, _ := cache.Get(ctx, fmt.Sprintf("apikeys:quota:%s", keyHash))
	used := 0
	fmt.Sscanf(quotaUsedStr, "%d", &used)

	// ambil usage per group dari SQLite
	usagePerGroup := stats.GetKeyUsageByGroup(keyHash)

	c.JSON(http.StatusOK, adminResponse{
		Success: true,
		Data: gin.H{
			"name":            data.Name,
			"email":           data.Email,
			"active":          data.Active,
			"quota":           data.Quota,
			"quota_used":      used,
			"quota_remaining": data.Quota - used,
			"created_at":      data.CreatedAt,
			"usage_per_group": usagePerGroup,
		},
	})
}
