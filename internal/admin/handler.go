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
	"vidbot-api/internal/health"
	"vidbot-api/pkg/apikey"
	"vidbot-api/pkg/cache"
	"vidbot-api/pkg/limiter"
	"vidbot-api/pkg/response"
	"vidbot-api/pkg/stats"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	masterKey     string
	healthHandler *health.Handler
}

func NewHandler(masterKey string, healthHandler *health.Handler) *Handler {
	return &Handler{
		masterKey:     masterKey,
		healthHandler: healthHandler,
	}
}

func (h *Handler) GetHealth(c *gin.Context) {
	h.healthHandler.Check(c)
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

func (h *Handler) CreateKey(c *gin.Context) {

	var req CreateKeyRequest
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		response.AdminBadRequest(c, err.Error())
		return
	}

	if req.Name == "" || req.Email == "" || req.Quota < 1 {
		response.AdminBadRequest(c, "name, email, dan quota wajib diisi.")
		return
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		response.AdminServiceError(c, "generate key", err)
		return
	}
	plainKey := hex.EncodeToString(raw)
	keyPrefix := plainKey[:8]
	hash := sha256.Sum256([]byte(plainKey))
	keyHash := hex.EncodeToString(hash[:])

	data := apikey.Data{
		KeyHash:   keyHash,
		KeyPrefix: keyPrefix,
		Name:      req.Name,
		Email:     req.Email,
		Active:    true,
		Quota:     req.Quota,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	jsonData, _ := json.Marshal(data)
	ctx := context.Background()

	if err := cache.Set(ctx, fmt.Sprintf("apikeys:%s", keyHash), string(jsonData), 0); err != nil {
		response.AdminDB(c, "save key", err)
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

	if c.Request.ContentLength > 0 {
		response.AdminBadRequest(c, "Request body not allowed.")
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
		response.AdminNotFound(c, "API key not found.")
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

	activeFilter := c.Query("active")
	emailFilter := c.Query("email")

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

		prefix := data.KeyPrefix
		if prefix == "" {
			prefix = data.KeyHash[:8]
		}

		if activeFilter == "true" && !data.Active {
			continue
		}
		if activeFilter == "false" && data.Active {
			continue
		}
		if emailFilter != "" && data.Email != emailFilter {
			continue
		}

		quotaUsedStr, _ := cache.Get(ctx, fmt.Sprintf("apikeys:quota:%s", keyHash))
		used := 0
		fmt.Sscanf(quotaUsedStr, "%d", &used)

		result = append(result, gin.H{
			"key_hash":        data.KeyHash,
			"key_prefix":      prefix,
			"name":            data.Name,
			"email":           data.Email,
			"active":          data.Active,
			"quota":           data.Quota,
			"quota_used":      used,
			"quota_remaining": data.Quota - used,
			"created_at":      data.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"total":   len(result),
		"data":    result,
	})
}

func (h *Handler) LookupKey(c *gin.Context) {

	var req struct {
		APIKey string `json:"api_key"`
	}

	if err := c.ShouldBindJSON(&req); err != nil || req.APIKey == "" {
		response.AdminBadRequest(c, "api_key is required.")
		return
	}

	// hash dari API key
	hash := sha256.Sum256([]byte(req.APIKey))
	keyHash := hex.EncodeToString(hash[:])

	ctx := context.Background()

	// ambil data key
	raw, err := cache.Get(ctx, fmt.Sprintf("apikeys:%s", keyHash))
	if err != nil {
		response.AdminNotFound(c, "API key tidak ditemukan.")
		return
	}

	var data apikey.Data
	json.Unmarshal([]byte(raw), &data)

	// ambil usage
	quotaUsedStr, _ := cache.Get(ctx, fmt.Sprintf("apikeys:quota:%s", keyHash))
	used := 0
	fmt.Sscanf(quotaUsedStr, "%d", &used)

	usagePerGroup := stats.GetKeyUsageByGroup(keyHash)

	c.JSON(http.StatusOK, adminResponse{
		Success: true,
		Data: gin.H{
			"key_hash":        keyHash,
			"key_prefix":      data.KeyPrefix,
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

func (h *Handler) TopUpQuota(c *gin.Context) {

	apiKey := c.Param("key")
	var req struct {
		Amount int `json:"amount" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Amount <= 0 {
		response.AdminBadRequest(c, "amount is required and must be positive.")
		return
	}

	hash := sha256.Sum256([]byte(apiKey))
	keyHash := hex.EncodeToString(hash[:])
	redisKey := fmt.Sprintf("apikeys:%s", keyHash)

	ctx := context.Background()
	raw, err := cache.Get(ctx, redisKey)
	if err != nil {
		response.AdminNotFound(c, "API key not found.")
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

func (h *Handler) ToggleFeature(c *gin.Context) {
	group := c.Param("group")
	if !validGroups[group] {
		response.AdminInvalidGroup(c, group)
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || (req.Status != "on" && req.Status != "off") {
		response.AdminBadRequest(c, "status must be 'on' or 'off'.")
		return
	}

	ctx := context.Background()
	cache.Set(ctx, fmt.Sprintf("feature:%s", group), req.Status, 0)

	c.JSON(http.StatusOK, adminMessageResponse{
		Success: true,
		Message: fmt.Sprintf("Feature '%s' is now %s.", group, req.Status),
	})
}

func (h *Handler) ToggleFeaturePlatform(c *gin.Context) {
	group := c.Param("group")
	platform := c.Param("platform")

	if !isValidPlatform(group, platform) {
		response.AdminInvalidPlatform(c, platform, group)
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || (req.Status != "on" && req.Status != "off") {
		c.JSON(http.StatusBadRequest, adminErrorResponse{
			Success: false,
			Code:    "BAD_REQUEST",
			Message: "status must be 'on' or 'off'",
		})
		return
	}

	ctx := context.Background()
	cache.Set(ctx, fmt.Sprintf("feature:%s:%s", group, platform), req.Status, 0)

	c.JSON(http.StatusOK, adminMessageResponse{
		Success: true,
		Message: fmt.Sprintf("Platform '%s/%s' is now %s.", group, platform, req.Status),
	})
}

func (h *Handler) GetStats(c *gin.Context) {
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
			usageData[group] = totalReq
		}
	}

	// query params opsional
	days := 7
	if d := c.Query("days"); d != "" {
		fmt.Sscanf(d, "%d", &days)
		if days < 1 || days > 90 {
			days = 7
		}
	}

	topLimit := 5
	if t := c.Query("top"); t != "" {
		fmt.Sscanf(t, "%d", &topLimit)
		if topLimit < 1 || topLimit > 20 {
			topLimit = 5
		}
	}

	// daily trend per group
	dailyTrend := gin.H{}
	for group := range validGroups {
		dailyTrend[group] = stats.GetDailyStats(group, days)
	}

	// hourly breakdown hari ini — semua group digabung atau per group
	hourlyBreakdown := gin.H{}
	if c.Query("hourly") == "1" {
		for group := range validGroups {
			hourlyBreakdown[group] = stats.GetHourlyStats(group)
		}
	}

	// top keys — enrich dengan nama kalau bisa
	topKeysRaw := stats.GetTopKeys(topLimit)
	topKeys := make([]gin.H, 0, len(topKeysRaw))
	for _, k := range topKeysRaw {
		keyHash, _ := k["key_hash"].(string)
		entry := gin.H{
			"key_hash": keyHash,
			"count":    k["count"],
		}
		if raw, err := cache.Get(ctx, fmt.Sprintf("apikeys:%s", keyHash)); err == nil {
			var data apikey.Data
			if json.Unmarshal([]byte(raw), &data) == nil {
				entry["name"] = data.Name
				entry["email"] = data.Email
				entry["active"] = data.Active
			}
		}
		topKeys = append(topKeys, entry)
	}

	resp := gin.H{
		"total_keys":     totalKeys,
		"active_keys":    activeKeys,
		"total_requests": grandTotalReq,
		"today_requests": grandTodayReq,
		"unique_keys":    grandUniqueKeys,
		"usage":          usageData,
		"trend": gin.H{
			"days":  days,
			"daily": dailyTrend,
		},
		"top_keys": topKeys,
	}

	if c.Query("hourly") == "1" {
		resp["hourly"] = hourlyBreakdown
	}

	c.JSON(http.StatusOK, adminResponse{
		Success: true,
		Data:    resp,
	})
}

func (h *Handler) GetKeyUsage(c *gin.Context) {
	keyHash := c.Param("key")

	ctx := context.Background()
	raw, err := cache.Get(ctx, fmt.Sprintf("apikeys:%s", keyHash))
	if err != nil {
		response.AdminNotFound(c, "API key not found.")
		return
	}

	var data apikey.Data
	json.Unmarshal([]byte(raw), &data)

	quotaUsedStr, _ := cache.Get(ctx, fmt.Sprintf("apikeys:quota:%s", keyHash))
	used := 0
	fmt.Sscanf(quotaUsedStr, "%d", &used)

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

func (h *Handler) GetRealtimeStats(c *gin.Context) {
	minutes := 30
	if m := c.Query("minutes"); m != "" {
		fmt.Sscanf(m, "%d", &minutes)
		if minutes < 1 || minutes > 1440 {
			minutes = 30
		}
	}

	c.JSON(http.StatusOK, adminResponse{
		Success: true,
		Data: gin.H{
			"realtime":      stats.GetRealtimeStats(minutes),
			"today_by_hour": stats.GetTodayByHour(),
		},
	})
}

func (h *Handler) GetErrorStats(c *gin.Context) {
	group := c.Query("group")
	platform := c.Query("platform")
	hours := 24
	if h := c.Query("hours"); h != "" {
		fmt.Sscanf(h, "%d", &hours)
		if hours < 1 || hours > 168 {
			hours = 24
		}
	}
	limit := 20
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
		if limit < 1 || limit > 100 {
			limit = 20
		}
	}

	c.JSON(http.StatusOK, adminResponse{
		Success: true,
		Data: gin.H{
			"by_code": stats.GetErrorStats(group, platform, hours),
			"recent":  stats.GetRecentErrors(limit),
		},
	})
}

func (h *Handler) GetActiveSessions(c *gin.Context) {
	ctx := context.Background()
	tokens, err := cache.SMembers(ctx, "admin:sessions:active")
	if err != nil {
		response.AdminDB(c, "fetch sessions", err)
		return
	}

	sessions := []gin.H{}
	for _, token := range tokens {
		raw, err := cache.Get(ctx, "admin:session:"+token)
		if err != nil {
			// session sudah expired, skip
			continue
		}
		var data AdminSessionData
		if err := json.Unmarshal([]byte(raw), &data); err != nil {
			continue
		}
		sessions = append(sessions, gin.H{
			"session_id": data.SessionID,
			"ip_address": data.IPAddress,
			"user_agent": data.UserAgent,
			"created_at": data.CreatedAt,
			"expires_at": data.ExpiresAt,
		})
	}

	c.JSON(http.StatusOK, adminResponse{
		Success: true,
		Data: gin.H{
			"total":    len(sessions),
			"sessions": sessions,
		},
	})
}

func (h *Handler) RevokeSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	ctx := context.Background()
	cache.Del(ctx, "admin:session:"+sessionID)
	cache.SRem(ctx, "admin:sessions:active", sessionID)
	c.JSON(http.StatusOK, adminMessageResponse{
		Success: true,
		Message: "Session revoked",
	})
}

func (h *Handler) GetSystemQueue(c *gin.Context) {
	c.JSON(http.StatusOK, adminResponse{
		Success: true,
		Data: gin.H{
			"hls_download": gin.H{
				"current": limiter.HLSDownload.Current(),
				"max":     limiter.HLSDownload.Max(),
			},
			"direct_stream": gin.H{
				"current": limiter.DirectStream.Current(),
				"max":     limiter.DirectStream.Max(),
			},
		},
	})
}
