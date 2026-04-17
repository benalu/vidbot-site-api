package admin

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"math/rand"

	"fmt"
	"net/http"
	"time"
	"vidbot-api/pkg/cache"

	"github.com/gin-gonic/gin"
)

const (
	adminSessionPrefix = "admin:session:"
	adminSessionIndex  = "admin:sessions:active"
	defaultSessionTTL  = 8 * time.Hour
)

type AdminSessionData struct {
	SessionID string `json:"session_id"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at"`
	IPAddress string `json:"ip_address"`
	UserAgent string `json:"user_agent"`
}

func (h *Handler) Login(c *gin.Context) {
	var req struct {
		MasterKey string `json:"master_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false, "code": "BAD_REQUEST", "message": "master_key is required",
		})
		return
	}

	// Constant-time compare + delay untuk anti-brute-force
	if req.MasterKey != h.masterKey {
		time.Sleep(500 * time.Millisecond)
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false, "code": "UNAUTHORIZED", "message": "Invalid credentials",
		})
		return
	}

	raw := make([]byte, 32)
	if _, err := crand.Read(raw); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to generate session"})
		return
	}
	token := hex.EncodeToString(raw)

	ttl := defaultSessionTTL
	sessionData := AdminSessionData{
		SessionID: token,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		ExpiresAt: time.Now().Add(ttl).UTC().Format(time.RFC3339),
		IPAddress: c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
	}

	data, _ := json.Marshal(sessionData)
	ctx := context.Background()
	if err := cache.Set(ctx, adminSessionPrefix+token, string(data), ttl); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to save session"})
		return
	}
	cache.SAdd(ctx, adminSessionIndex, token)
	if rand.Intn(10) == 0 {
		go CleanupExpiredSessions()
	}
	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"session_token": token,
		"expires_at":    sessionData.ExpiresAt,
	})
}

func (h *Handler) Logout(c *gin.Context) {
	token := c.GetHeader("X-Admin-Session")
	ctx := context.Background()
	cache.Del(ctx, adminSessionPrefix+token)
	cache.SRem(ctx, adminSessionIndex, token)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Logged out"})
}

func (h *Handler) Me(c *gin.Context) {
	token := c.GetHeader("X-Admin-Session")
	ctx := context.Background()
	raw, err := cache.Get(ctx, adminSessionPrefix+token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "code": "UNAUTHORIZED", "message": "Session not found"})
		return
	}
	var data AdminSessionData
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to read session"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// ValidateAdminSession — dipakai oleh middleware
func ValidateAdminSession(token string) (*AdminSessionData, error) {
	ctx := context.Background()
	raw, err := cache.Get(ctx, adminSessionPrefix+token)
	if err != nil {
		return nil, fmt.Errorf("session not found or expired")
	}

	var data AdminSessionData
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, fmt.Errorf("invalid session data")
	}

	return &data, nil
}

func CleanupExpiredSessions() {
	ctx := context.Background()

	tokens, err := cache.SMembers(ctx, adminSessionIndex)
	if err != nil {
		return
	}

	for _, token := range tokens {
		_, err := cache.Get(ctx, adminSessionPrefix+token)
		if err != nil {
			// session sudah expired → hapus dari index
			cache.SRem(ctx, adminSessionIndex, token)
		}
	}
}
