package leakcheck

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
	"vidbot-api/pkg/leakcheck"
	"vidbot-api/pkg/response"
	"vidbot-api/pkg/stats"

	"github.com/gin-gonic/gin"
)

var blockedTerms = map[string]struct{}{
	"admin": {}, "username": {}, "password": {},
	"123456": {}, "qwerty": {}, "email": {},
}

type Handler struct {
	leakcheckDir string
}

func NewHandler(leakcheckDir string) *Handler {
	return &Handler{leakcheckDir: leakcheckDir}
}

type SearchRequest struct {
	Id string `json:"id" binding:"required"`
}

type SearchResponse struct {
	Success  bool              `json:"success"`
	Services string            `json:"services"`
	Total    int               `json:"total"`
	Data     []leakcheck.Entry `json:"data"`
}

func (h *Handler) Search(c *gin.Context) {
	stats.Group(c, "leakcheck")
	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.WriteMsg(c, response.ErrBadRequest, "id is missing.")
		return
	}

	if len(req.Id) < 6 {
		stats.TrackError(c, "leakcheck", "", "TOO_SHORT")
		response.WriteMsg(c, response.ErrBadRequest, "id must be at least 6 characters.")
		return
	}

	idLower := strings.ToLower(strings.TrimSpace(req.Id))
	if _, blocked := blockedTerms[idLower]; blocked {
		stats.TrackError(c, "leakcheck", "", "BLOCKED_TERM")
		response.Write(c, response.ErrBadRequest)
		return
	}

	results := leakcheck.Default.Search(req.Id)

	if results == nil {
		slog.Error("leakcheck search failed, db may be down or uninitialized",
			"group", "leakcheck",
			"query_length", len(req.Id),
		)
		stats.TrackError(c, "leakcheck", "", "DB_ERROR")
		response.Write(c, response.ErrServiceError)
		return
	}

	if len(results) == 0 {
		stats.TrackError(c, "leakcheck", "", "NOT_FOUND")
		response.WriteMsg(c, response.ErrNotFound, "id not found")
		return
	}

	c.JSON(http.StatusOK, SearchResponse{
		Success:  true,
		Services: "leakcheck",
		Total:    len(results),
		Data:     results,
	})
}

func (h *Handler) Reload(c *gin.Context) {

	count, err := leakcheck.Default.Reload(h.leakcheckDir)
	if err != nil {
		response.AdminServiceError(c, "reload leakcheck", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Leakcheck reloaded successfully.",
		"total":   count,
	})
}

// AddDir menambah data dari subdirektori baru tanpa full rebuild.
// Body JSON: { "dir": "nama-subfolder" }
// Dir yang diterima adalah nama folder relatif terhadap leakcheckDir,
// contoh: POST body { "dir": "batch-2" } akan membaca dari data/leakcheck/batch-2/
func (h *Handler) AddDir(c *gin.Context) {

	var req struct {
		Dir string `json:"dir" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AdminBadRequest(c, "Field 'dir' is required.")
		return
	}

	// keamanan: tolak path traversal
	if strings.Contains(req.Dir, "..") || strings.ContainsAny(req.Dir, `/\`) {
		response.AdminBadRequest(c, "Invalid dir path.")
		return
	}

	targetDir := h.leakcheckDir + "/" + req.Dir
	start := time.Now()
	count, err := leakcheck.Default.AddDir(targetDir)
	if err != nil {
		response.AdminServiceError(c, "add leakcheck dir", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Data added successfully.",
		"added":      count,
		"total":      leakcheck.Default.CachedCount(),
		"elapsed_ms": time.Since(start).Milliseconds(),
	})
}

func (h *Handler) Count(c *gin.Context) {
	stats.Group(c, "leakcheck")

	count := leakcheck.Default.Count()

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"services": "leakcheck",
		"total":    count,
	})
}

func (h *Handler) Stats(c *gin.Context) {

	s := leakcheck.Default.Stats()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"db_ready":    s.DBReady,
			"entry_count": s.EntryCount,
			"latency_ms":  s.LatencyMs,
			"cache": gin.H{
				"size": s.CacheSize,
			},
		},
	})
}
