package app

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"vidbot-api/pkg/appstore"
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
	URL1         string `json:"url_1"`
	URL2         string `json:"url_2,omitempty"`
	URL3         string `json:"url_3,omitempty"`
}

func (h *Handler) AdminAdd(c *gin.Context) {
	platform, ok := normPlatform(c)
	if !ok {
		return
	}

	var req addRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AdminBadRequest(c, "request body tidak valid.")
		return
	}
	if err := validateEntry(req.Name, req.Category, req.Version, req.URL1); err != nil {
		response.AdminBadRequest(c, err.Error())
		return
	}

	result, err := appstore.Upsert(platform, appstore.UpsertEntry{
		Name:         strings.TrimSpace(req.Name),
		Category:     strings.TrimSpace(req.Category),
		Overview:     strings.TrimSpace(req.Overview),
		Requirements: strings.TrimSpace(req.Requirements),
		Image:        strings.TrimSpace(req.Image),
		Version:      strings.TrimSpace(req.Version),
		URL1:         strings.TrimSpace(req.URL1),
		URL2:         strings.TrimSpace(req.URL2),
		URL3:         strings.TrimSpace(req.URL3),
	})
	if err != nil {
		response.AdminDB(c, "upsert app", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

func (h *Handler) AdminGet(c *gin.Context) {
	platform, ok := normPlatform(c)
	if !ok {
		return
	}
	slug := c.Param("slug")

	app, err := appstore.GetBySlug(platform, slug)
	if err != nil {
		response.AdminDB(c, "get app", err)
		return
	}
	if app == nil {
		response.AdminNotFound(c, "app tidak ditemukan.")
		return
	}

	type versionItem struct {
		ID        int64  `json:"id"`
		Version   string `json:"version"`
		URL1      string `json:"url_1"`
		URL2      string `json:"url_2,omitempty"`
		URL3      string `json:"url_3,omitempty"`
		CreatedAt string `json:"created_at"`
	}
	vers := make([]versionItem, 0, len(app.Versions))
	for _, v := range app.Versions {
		vers = append(vers, versionItem{
			ID: v.ID, Version: v.Version,
			URL1: v.URL1, URL2: v.URL2, URL3: v.URL3,
			CreatedAt: v.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"slug": app.Slug, "name": app.Name,
			"category": app.Category, "overview": app.Overview,
			"requirements": app.Requirements, "image": app.Image,
			"created_at": app.CreatedAt, "versions": vers,
		},
	})
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
		response.AdminBadRequest(c, "request body tidak valid.")
		return
	}
	if len(req.Entries) == 0 {
		response.AdminBadRequest(c, "entries tidak boleh kosong.")
		return
	}
	if len(req.Entries) > 200 {
		response.AdminBadRequest(c, "maksimum 200 entries per request.")
		return
	}

	type indexedEntry struct {
		originalIdx int
		entry       appstore.UpsertEntry
	}
	indexed := make([]indexedEntry, 0, len(req.Entries))
	validationErrs := []gin.H{}

	for i, e := range req.Entries {
		if err := validateEntry(e.Name, e.Category, e.Version, e.URL1); err != nil {
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
				URL1:         strings.TrimSpace(e.URL1),
				URL2:         strings.TrimSpace(e.URL2),
				URL3:         strings.TrimSpace(e.URL3),
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
		response.AdminDB(c, "delete app", err)
		return
	}
	if !deleted {
		response.AdminNotFound(c, "app tidak ditemukan.")
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
		response.AdminBadRequest(c, "id versi tidak valid.")
		return
	}

	deleted, err := appstore.DeleteVersion(platform, id)
	if err != nil {
		response.AdminDB(c, "delete version", err)
		return
	}
	if !deleted {
		response.AdminNotFound(c, "versi tidak ditemukan.")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "versi berhasil dihapus"})
}

// ─── Admin: Edit App Metadata ─────────────────────────────────────────────────

type editAppRequest struct {
	Name         string `json:"name,omitempty"`
	Category     string `json:"category,omitempty"`
	Overview     string `json:"overview,omitempty"`
	Requirements string `json:"requirements,omitempty"`
	Image        string `json:"image,omitempty"`
}

func (h *Handler) AdminEdit(c *gin.Context) {
	platform, ok := normPlatform(c)
	if !ok {
		return
	}
	slug := c.Param("slug")

	var req editAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AdminBadRequest(c, "request body tidak valid.")
		return
	}

	result, err := appstore.UpdateApp(platform, slug, appstore.UpdateAppEntry{
		Name:         strings.TrimSpace(req.Name),
		Category:     strings.TrimSpace(req.Category),
		Overview:     strings.TrimSpace(req.Overview),
		Requirements: strings.TrimSpace(req.Requirements),
		Image:        strings.TrimSpace(req.Image),
	})
	if err != nil {
		response.AdminDB(c, "update app", err)
		return
	}
	if !result.Found {
		response.AdminNotFound(c, "app tidak ditemukan.")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// ─── Admin: Edit Version URL ──────────────────────────────────────────────────

type editVersionRequest struct {
	URL1 string `json:"url_1,omitempty"`
	URL2 string `json:"url_2,omitempty"`
	URL3 string `json:"url_3,omitempty"`
}

func (h *Handler) AdminEditVersion(c *gin.Context) {
	platform, ok := normPlatform(c)
	if !ok {
		return
	}
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id < 1 {
		response.AdminBadRequest(c, "id versi tidak valid.")
		return
	}

	var req editVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AdminBadRequest(c, "request body tidak valid.")
		return
	}
	if req.URL1 == "" && req.URL2 == "" && req.URL3 == "" {
		response.AdminBadRequest(c, "minimal satu field url harus diisi.")
		return
	}

	result, err := appstore.UpdateVersion(platform, id, appstore.UpdateVersionEntry{
		URL1: strings.TrimSpace(req.URL1),
		URL2: strings.TrimSpace(req.URL2),
		URL3: strings.TrimSpace(req.URL3),
	})
	if err != nil {
		response.AdminDB(c, "update version", err)
		return
	}
	if !result.Found {
		response.AdminNotFound(c, "versi tidak ditemukan.")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
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
		response.AdminDB(c, "list apps", err)
		return
	}

	type versionItem struct {
		ID        int64  `json:"id"`
		Version   string `json:"version"`
		URL1      string `json:"url_1"`
		URL2      string `json:"url_2,omitempty"`
		URL3      string `json:"url_3,omitempty"`
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
				URL1:      v.URL1,
				URL2:      v.URL2,
				URL3:      v.URL3,
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

func validateEntry(name, category, version, url1 string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("name wajib diisi")
	}
	if strings.TrimSpace(category) == "" {
		return fmt.Errorf("category wajib diisi")
	}
	if strings.TrimSpace(version) == "" {
		return fmt.Errorf("version wajib diisi")
	}
	if strings.TrimSpace(url1) == "" {
		return fmt.Errorf("url_1 wajib diisi")
	}
	return nil
}

func normPlatform(c *gin.Context) (string, bool) {
	p := strings.ToLower(strings.TrimSpace(c.Param("platform")))
	if !appstore.IsValidPlatform(p) {
		response.AdminBadRequest(c, fmt.Sprintf("platform '%s' tidak valid, gunakan: android, windows.", p))
		return "", false
	}
	return p, true
}
