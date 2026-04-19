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
	"vidbot-api/pkg/response"

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
		Key string `json:"key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AdminBadRequest(c, "key is required.")
		return
	}
	if req.Key != h.masterKey {
		time.Sleep(500 * time.Millisecond)
		response.Abort(c, response.ErrAdminUnauthorized)
		return
	}

	raw := make([]byte, 32)
	if _, err := crand.Read(raw); err != nil {
		response.AdminServiceError(c, "generate session token", err)
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
		response.AdminDB(c, "save session", err)
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
	if token == "" {
		// akses via X-Master-Key tidak punya session, tidak ada yang perlu di-logout
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "No session to logout"})
		return
	}
	ctx := context.Background()
	cache.Del(ctx, adminSessionPrefix+token)
	cache.SRem(ctx, adminSessionIndex, token)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Logged out"})
}

func (h *Handler) Me(c *gin.Context) {
	token := c.GetHeader("X-Admin-Session")
	if token == "" {
		// akses via X-Master-Key — tidak ada session data, kembalikan info minimal
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"auth_method": "master_key",
				"session_id":  nil,
			},
		})
		return
	}
	ctx := context.Background()
	raw, err := cache.Get(ctx, adminSessionPrefix+token)
	if err != nil {
		response.Write(c, response.ErrAdminSessionExpired)
		return
	}
	var data AdminSessionData
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		response.AdminServiceError(c, "read session", err)
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
