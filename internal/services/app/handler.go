package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"vidbot-api/pkg/appstore"
	"vidbot-api/pkg/cdnstore"
	"vidbot-api/pkg/httputil"
	"vidbot-api/pkg/response"
	"vidbot-api/pkg/stats"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	appURL      string
	cdnResolver *cdnstore.Resolver
	platform    string // diset per-handler di router
}

func NewHandler(appURL string, cdnResolver *cdnstore.Resolver) *Handler {
	return &Handler{
		appURL:      appURL,
		cdnResolver: cdnResolver,
	}
}

// ─── Response types (shape tidak berubah dari sebelumnya) ────────────────────

type variantItem struct {
	Variant string `json:"variant"`
	URL     string `json:"url"`
}

type downloadItem struct {
	Version  string        `json:"version"`
	Variants []variantItem `json:"variants"`
}

type appItem struct {
	Slug         string         `json:"slug"`
	Name         string         `json:"name"`
	Category     string         `json:"category"`
	Overview     string         `json:"overview"`
	Requirements string         `json:"requirements"`
	Image        string         `json:"image"`
	Download     []downloadItem `json:"download"`
}

type searchResponse struct {
	Success  bool      `json:"success"`
	Services string    `json:"services"`
	Platform string    `json:"platform"`
	Total    int       `json:"total"`
	Data     []appItem `json:"data"`
}

type browseResponse struct {
	Success  bool      `json:"success"`
	Services string    `json:"services"`
	Platform string    `json:"platform"`
	Category string    `json:"category"`
	Page     int       `json:"page"`
	Limit    int       `json:"limit"`
	Total    int       `json:"total"`
	Data     []appItem `json:"data"`
}

// ─── Search ───────────────────────────────────────────────────────────────────

func (h *Handler) SearchAndroid(c *gin.Context) {
	stats.Platform(c, "app", "android")
	h.search(c, "android")
}

func (h *Handler) SearchWindows(c *gin.Context) {
	stats.Platform(c, "app", "windows")
	h.search(c, "windows")
}

func (h *Handler) search(c *gin.Context, platform string) {
	var body map[string]string
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body.")
		return
	}

	keyword := strings.TrimSpace(body["apk"])
	if keyword == "" {
		keyword = strings.TrimSpace(body["app"])
	}
	if keyword == "" {
		response.WriteMsg(c, response.ErrBadRequest, "Search keyword is required. Use the 'app' or 'apk' field.")
		return
	}
	if len(keyword) < 3 {
		response.WriteMsg(c, response.ErrBadRequest, "Search keyword must be at least 3 characters.")
		return
	}

	apps, err := appstore.Search(platform, keyword)
	if err != nil {
		slog.Error("app search db query failed",
			"group", "app",
			"platform", platform,
			"keyword", keyword,
			"error", err,
		)
		stats.TrackError(c, "app", platform, "DB_ERROR")
		response.DB(c, "app", platform, err)
		return
	}

	items := h.buildAppItems(c.Request.Context(), apps, platform)
	if len(items) == 0 {
		stats.TrackError(c, "app", platform, "NOT_FOUND")
		// user: clear but not exposing internal state (e.g. "db returned 0 rows")
		response.WriteMsg(c, response.ErrNotFound, "app/apk not found")
		return
	}

	httputil.WriteJSONOK(c, searchResponse{
		Success:  true,
		Services: "app",
		Platform: platform,
		Total:    len(items),
		Data:     items,
	})
}

// ─── Browse by category ───────────────────────────────────────────────────────

func (h *Handler) BrowseAndroid(c *gin.Context) {
	stats.Platform(c, "app", "android")
	h.browseByCategory(c, "android")
}

func (h *Handler) BrowseWindows(c *gin.Context) {
	stats.Platform(c, "app", "windows")
	h.browseByCategory(c, "windows")
}

func (h *Handler) browseByCategory(c *gin.Context, platform string) {
	category := strings.TrimSpace(c.Param("category"))
	if category == "" {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "Category is required.")
		return
	}

	limit := 20
	page := 1
	if p, err := strconv.Atoi(c.Query("page")); err == nil && p > 0 {
		page = p
	}
	offset := (page - 1) * limit

	apps, total, err := appstore.SearchByCategory(platform, category, limit, offset)
	if err != nil {
		slog.Error("app browse by category db query failed",
			"group", "app",
			"platform", platform,
			"category", category,
			"page", page,
			"offset", offset,
			"error", err,
		)
		stats.TrackError(c, "app", platform, "DB_ERROR")
		response.DB(c, "app", platform, err)
		return
	}
	if total == 0 {
		stats.TrackError(c, "app", platform, "NOT_FOUND")
		response.WriteMsg(c, response.ErrNotFound,
			fmt.Sprintf("Category '%s' not found. See /app/%s/category for valid categories.", category, platform))
		return
	}

	items := h.buildAppItems(c.Request.Context(), apps, platform)

	httputil.WriteJSONOK(c, browseResponse{
		Success:  true,
		Services: "app",
		Platform: platform,
		Category: category,
		Page:     page,
		Limit:    limit,
		Total:    total,
		Data:     items,
	})
}

// ─── Categories ───────────────────────────────────────────────────────────────

func (h *Handler) CategoriesAndroid(c *gin.Context) {
	stats.Platform(c, "app", "android")
	h.getCategories(c, "android")
}

func (h *Handler) CategoriesWindows(c *gin.Context) {
	stats.Platform(c, "app", "windows")
	h.getCategories(c, "windows")
}

func (h *Handler) getCategories(c *gin.Context, platform string) {
	categories, err := appstore.GetCategories(platform)
	if err != nil {
		slog.Error("app get categories db query failed",
			"group", "app",
			"platform", platform,
			"error", err,
		)
		stats.TrackError(c, "app", platform, "DB_ERROR")
		response.DB(c, "app", platform, err)
		return
	}

	type categoriesResponse struct {
		Success  bool                     `json:"success"`
		Services string                   `json:"services"`
		Platform string                   `json:"platform"`
		Total    int                      `json:"total"`
		Data     []appstore.CategoryCount `json:"data"`
	}

	httputil.WriteJSONOK(c, categoriesResponse{
		Success:  true,
		Services: "app",
		Platform: platform,
		Total:    len(categories),
		Data:     categories,
	})
}

// ─── Download redirect ────────────────────────────────────────────────────────

func (h *Handler) Download(c *gin.Context) {
	key := strings.TrimSpace(c.Query("k"))
	if key == "" {
		response.WriteMsg(c, response.ErrBadRequest, "Download key is required.")
		return
	}

	rawURL, err := appstore.ResolveURL(key)
	if err != nil {
		slog.Warn("app download shortlink not found or expired",
			"group", "app",
			"key", key,
			"error", err,
		)
		response.WriteMsg(c, response.ErrNotFound, "Download link not found or has expired.")
		return
	}

	c.Redirect(http.StatusFound, rawURL)
}

// ─── Core: build app items dengan CDN URLs ────────────────────────────────────

func (h *Handler) buildAppItems(ctx context.Context, apps []appstore.App, platform string) []appItem {
	base := strings.TrimRight(h.appURL, "/")
	items := make([]appItem, 0, len(apps))

	for _, a := range apps {
		dls := h.buildDownloadItems(ctx, a, platform, base)
		items = append(items, appItem{
			Slug:         a.Slug,
			Name:         a.Name,
			Category:     a.Category,
			Overview:     a.Overview,
			Requirements: a.Requirements,
			Image:        a.Image,
			Download:     dls,
		})
	}
	return items
}

// buildDownloadItems — untuk tiap versi di DB, fetch signed URLs dari CDN
// lalu kelompokkan per versi dengan variants
func (h *Handler) buildDownloadItems(ctx context.Context, a appstore.App, platform, base string) []downloadItem {
	if h.cdnResolver == nil {
		return []downloadItem{}
	}

	// ambil cdn_query dari versi pertama kalau ada, fallback ke nama app
	cdnKeyword := a.Name
	if len(a.Versions) > 0 && a.Versions[0].CDNQuery != "" {
		cdnKeyword = a.Versions[0].CDNQuery
	}

	// fetch semua file sekaligus, tanpa filter versi (version = "")
	files, err := h.cdnResolver.GetOrFetchFiles(ctx, platform, a.Slug, cdnKeyword, "")
	if err != nil {
		slog.Error("cdn fetch failed for app",
			"group", "app",
			"platform", platform,
			"app_name", a.Name,
			"app_slug", a.Slug,
			"cdn_keyword", cdnKeyword,
			"error", err,
		)
		return []downloadItem{}
	}

	versionOrder := []string{}
	versionMap := map[string][]variantItem{}

	for _, f := range files {
		masked, err := appstore.MaskURL(f.SignedURL)
		if err != nil {
			slog.Error("failed to mask cdn url",
				"group", "app",
				"platform", platform,
				"app_slug", a.Slug,
				"file_id", f.FileID,
				"file_name", f.OriginalName,
				"error", err,
			)
			continue
		}

		ver := f.Version
		if ver == "" {
			ver = "unknown"
		}

		if _, exists := versionMap[ver]; !exists {
			versionOrder = append(versionOrder, ver)
		}
		versionMap[ver] = append(versionMap[ver], variantItem{
			Variant: f.Variant,
			URL:     base + "/app/dl?k=" + masked,
		})
	}

	dls := make([]downloadItem, 0, len(versionOrder))
	for _, ver := range versionOrder {
		dls = append(dls, downloadItem{
			Version:  ver,
			Variants: versionMap[ver],
		})
	}
	return dls
}
