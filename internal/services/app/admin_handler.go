package app

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"vidbot-api/pkg/appstore"
	"vidbot-api/pkg/cdnstore"
	"vidbot-api/pkg/response"

	"github.com/gin-gonic/gin"
)

// ─── Admin: Add ───────────────────────────────────────────────────────────────

type addRequest struct {
	Name         string `json:"name"`
	Category     string `json:"category"`
	Overview     string `json:"overview"`
	Requirements string `json:"requirements"`
	Image        string `json:"image"`
	Version      string `json:"version"`
	// CDNQuery opsional: override keyword pencarian di CDN
	// kosong = pakai nama app
	// berguna jika nama file di CDN berbeda dari nama app
	// contoh: app "SpotiFLAC" tapi di CDN nama filenya "Spotiflac-Premium"
	CDNQuery string `json:"cdn_query,omitempty"`
}

func (h *Handler) AdminAdd(c *gin.Context) {
	platform, ok := normPlatform(c)
	if !ok {
		return
	}

	var req addRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "request body tidak valid")
		return
	}
	if err := validateEntry(req.Name, req.Category, req.Version); err != nil {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	result, err := appstore.Upsert(platform, appstore.UpsertEntry{
		Name:         strings.TrimSpace(req.Name),
		Category:     strings.TrimSpace(req.Category),
		Overview:     strings.TrimSpace(req.Overview),
		Requirements: strings.TrimSpace(req.Requirements),
		Image:        strings.TrimSpace(req.Image),
		Version:      strings.TrimSpace(req.Version),
		CDNQuery:     strings.TrimSpace(req.CDNQuery),
	})
	if err != nil {
		response.ErrorWithCode(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// ─── Admin: Bulk Add ──────────────────────────────────────────────────────────

type bulkRequest struct {
	Entries []addRequest `json:"entries"`
}

func (h *Handler) AdminBulkAdd(c *gin.Context) {
	platform, ok := normPlatform(c)
	if !ok {
		return
	}

	var req bulkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "request body tidak valid")
		return
	}
	if len(req.Entries) == 0 {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "entries tidak boleh kosong")
		return
	}
	if len(req.Entries) > 200 {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "maksimum 200 entries per request")
		return
	}

	type indexedEntry struct {
		originalIdx int
		entry       appstore.UpsertEntry
	}
	indexed := make([]indexedEntry, 0, len(req.Entries))
	validationErrs := []gin.H{}

	for i, e := range req.Entries {
		if err := validateEntry(e.Name, e.Category, e.Version); err != nil {
			validationErrs = append(validationErrs, gin.H{"index": i, "error": err.Error(), "name": e.Name})
			continue
		}
		indexed = append(indexed, indexedEntry{
			originalIdx: i,
			entry: appstore.UpsertEntry{
				Name:         strings.TrimSpace(e.Name),
				Category:     strings.TrimSpace(e.Category),
				Overview:     strings.TrimSpace(e.Overview),
				Requirements: strings.TrimSpace(e.Requirements),
				Image:        strings.TrimSpace(e.Image),
				Version:      strings.TrimSpace(e.Version),
				CDNQuery:     strings.TrimSpace(e.CDNQuery),
			},
		})
	}

	entries := make([]appstore.UpsertEntry, len(indexed))
	for i, ie := range indexed {
		entries[i] = ie.entry
	}

	results, bulkErrs := appstore.BulkUpsert(platform, entries)

	allErrs := validationErrs
	for sliceIdx, e := range bulkErrs {
		origIdx := indexed[sliceIdx].originalIdx
		allErrs = append(allErrs, gin.H{"index": origIdx, "error": e.Error()})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"processed": len(results),
			"failed":    len(allErrs),
			"results":   results,
			"errors":    allErrs,
		},
	})
}

// ─── Admin: Delete App ────────────────────────────────────────────────────────

func (h *Handler) AdminDelete(c *gin.Context) {
	platform, ok := normPlatform(c)
	if !ok {
		return
	}
	slug := c.Param("slug")

	deleted, err := appstore.Delete(platform, slug)
	if err != nil {
		response.ErrorWithCode(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if !deleted {
		response.ErrorWithCode(c, http.StatusNotFound, "NOT_FOUND", "app tidak ditemukan")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "app berhasil dihapus"})
}

// ─── Admin: Delete Version ────────────────────────────────────────────────────

func (h *Handler) AdminDeleteVersion(c *gin.Context) {
	platform, ok := normPlatform(c)
	if !ok {
		return
	}
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id < 1 {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "id versi tidak valid")
		return
	}

	deleted, err := appstore.DeleteVersion(platform, id)
	if err != nil {
		response.ErrorWithCode(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if !deleted {
		response.ErrorWithCode(c, http.StatusNotFound, "NOT_FOUND", "versi tidak ditemukan")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "versi berhasil dihapus"})
}

// ─── Admin: Invalidate CDN Cache ──────────────────────────────────────────────
// Gunakan ini kalau file di CDN di-update dan kamu mau paksa refresh signed URL

type invalidateCacheRequest struct {
	AppSlug string `json:"app_slug" binding:"required"`
	Version string `json:"version" binding:"required"`
}

func (h *Handler) AdminInvalidateCDNCache(c *gin.Context) {
	platform, ok := normPlatform(c)
	if !ok {
		return
	}

	var req invalidateCacheRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "app_slug dan version wajib diisi")
		return
	}

	if h.cdnResolver == nil {
		response.ErrorWithCode(c, http.StatusServiceUnavailable, "CDN_NOT_CONFIGURED", "CDN resolver tidak dikonfigurasi")
		return
	}

	if err := h.cdnResolver.InvalidateCache(c.Request.Context(), platform, req.AppSlug, req.Version); err != nil {
		response.ErrorWithCode(c, http.StatusInternalServerError, "CACHE_ERROR", err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("CDN cache untuk '%s' v%s berhasil dihapus. URL baru akan di-fetch pada request berikutnya.", req.AppSlug, req.Version),
	})
}

// ─── Admin: List ──────────────────────────────────────────────────────────────

func (h *Handler) AdminList(c *gin.Context) {
	platform, ok := normPlatform(c)
	if !ok {
		return
	}
	keyword := strings.TrimSpace(c.Query("q"))

	limit := 100
	offset := 0
	if v, _ := strconv.Atoi(c.Query("limit")); v > 0 && v <= 500 {
		limit = v
	}
	if page, _ := strconv.Atoi(c.Query("page")); page > 1 {
		offset = (page - 1) * limit
	}

	var apps []appstore.App
	var total int
	var err error
	if keyword == "" {
		apps, total, err = appstore.SearchAll(platform, limit, offset)
	} else {
		apps, err = appstore.Search(platform, keyword)
		total = len(apps)
	}

	if err != nil {
		response.ErrorWithCode(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	type versionItem struct {
		ID        int64  `json:"id"`
		Version   string `json:"version"`
		CDNQuery  string `json:"cdn_query,omitempty"`
		CreatedAt string `json:"created_at"`
	}
	type appAdminItem struct {
		Slug         string        `json:"slug"`
		Name         string        `json:"name"`
		Category     string        `json:"category"`
		Overview     string        `json:"overview"`
		Requirements string        `json:"requirements"`
		Image        string        `json:"image"`
		CreatedAt    string        `json:"created_at"`
		Versions     []versionItem `json:"versions"`
	}

	items := make([]appAdminItem, 0, len(apps))
	for _, a := range apps {
		vers := make([]versionItem, 0, len(a.Versions))
		for _, v := range a.Versions {
			vers = append(vers, versionItem{
				ID:        v.ID,
				Version:   v.Version,
				CDNQuery:  v.CDNQuery,
				CreatedAt: v.CreatedAt,
			})
		}
		items = append(items, appAdminItem{
			Slug:         a.Slug,
			Name:         a.Name,
			Category:     a.Category,
			Overview:     a.Overview,
			Requirements: a.Requirements,
			Image:        a.Image,
			CreatedAt:    a.CreatedAt,
			Versions:     vers,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"platform": platform,
		"total":    total,
		"page":     offset/limit + 1,
		"limit":    limit,
		"data":     items,
	})
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func validateEntry(name, category, version string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("name wajib diisi")
	}
	if strings.TrimSpace(category) == "" {
		return fmt.Errorf("category wajib diisi")
	}
	if strings.TrimSpace(version) == "" {
		return fmt.Errorf("version wajib diisi")
	}
	return nil
}

func normPlatform(c *gin.Context) (string, bool) {
	p := strings.ToLower(strings.TrimSpace(c.Param("platform")))
	if !appstore.IsValidPlatform(p) {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST",
			fmt.Sprintf("platform '%s' tidak valid, gunakan: android, windows", p))
		return "", false
	}
	return p, true
}

// normPlatformFromCDN helper untuk inject resolver ke handler yang sudah ada
// karena Handler tidak embeds platform context
var _ = (*cdnstore.Resolver)(nil) // compile check
