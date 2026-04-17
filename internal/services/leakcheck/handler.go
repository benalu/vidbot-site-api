package leakcheck

import (
	"net/http"
	"strings"
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
	masterKey    string
}

func NewHandler(leakcheckDir, masterKey string) *Handler {
	return &Handler{leakcheckDir: leakcheckDir, masterKey: masterKey}
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
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "id is required.")
		return
	}

	if len(req.Id) < 6 {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "id must be at least 6 characters.")
		return
	}

	idLower := strings.ToLower(strings.TrimSpace(req.Id))
	if _, blocked := blockedTerms[idLower]; blocked {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "please use a more specific id.")
		return
	}

	results := leakcheck.Default.Search(req.Id)

	if len(results) == 0 {
		response.ErrorWithCode(c, http.StatusNotFound, "NOT_FOUND", "No results found.")
		return
	}

	c.JSON(http.StatusOK, SearchResponse{
		Success:  true,
		Services: "leakcheck",
		Total:    len(results),
		Data:     results,
	})
}

func (h *Handler) validateMasterKey(c *gin.Context) bool {
	if c.GetHeader("X-Master-Key") != h.masterKey {
		response.ErrorWithCode(c, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid master key.")
		return false
	}
	return true
}

func (h *Handler) Reload(c *gin.Context) {
	if !h.validateMasterKey(c) {
		return
	}

	count, err := leakcheck.Default.Reload(h.leakcheckDir)
	if err != nil {
		response.ErrorWithCode(c, http.StatusInternalServerError, "RELOAD_FAILED", err.Error())
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
	if !h.validateMasterKey(c) {
		return
	}

	var req struct {
		Dir string `json:"dir" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "dir is required.")
		return
	}

	// keamanan: tolak path traversal
	if strings.Contains(req.Dir, "..") || strings.ContainsAny(req.Dir, `/\`) {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "invalid dir.")
		return
	}

	targetDir := h.leakcheckDir + "/" + req.Dir
	count, err := leakcheck.Default.AddDir(targetDir)
	if err != nil {
		response.ErrorWithCode(c, http.StatusInternalServerError, "ADD_FAILED", err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Data added successfully.",
		"added":   count,
		"total":   leakcheck.Default.Count(),
	})
}
