package app

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"vidbot-api/pkg/appstore"
	"vidbot-api/pkg/httputil"
	"vidbot-api/pkg/response"
	"vidbot-api/pkg/stats"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	appURL string
}

func NewHandler(appURL string) *Handler {
	return &Handler{appURL: appURL}
}

// ─── Response types (shape tidak berubah dari sebelumnya) ────────────────────

type downloadItem struct {
	Version string `json:"version"`
	URL1    string `json:"url_1"`
	URL2    string `json:"url_2,omitempty"`
	URL3    string `json:"url_3,omitempty"`
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

	items := h.buildAppItems(apps)
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

	items := h.buildAppItems(apps)

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

func (h *Handler) buildAppItems(apps []appstore.App) []appItem {
	base := strings.TrimRight(h.appURL, "/")

	var allURLs []string
	for _, a := range apps {
		for _, v := range a.Versions {
			if v.URL1 != "" {
				allURLs = append(allURLs, v.URL1)
			}
		}
	}
	maskedMap := appstore.MaskURLBatch(allURLs)

	items := make([]appItem, 0, len(apps))
	for _, a := range apps {
		items = append(items, appItem{
			Slug:         a.Slug,
			Name:         a.Name,
			Category:     a.Category,
			Overview:     a.Overview,
			Requirements: a.Requirements,
			Image:        a.Image,
			Download:     buildDownloadItems(base, a.Versions, maskedMap),
		})
	}
	return items
}

// BARU — url_1 dibungkus shortlink, url_2 dan url_3 raw
func buildDownloadItems(base string, versions []appstore.AppVersion, maskedMap map[string]string) []downloadItem {
	items := make([]downloadItem, 0, len(versions))
	for _, v := range versions {
		if v.URL1 == "" {
			continue
		}

		maskedKey, ok := maskedMap[v.URL1]
		if !ok {
			slog.Warn("failed to mask url_1", "version", v.Version)
			continue
		}

		item := downloadItem{
			Version: v.Version,
			URL1:    base + "/app/dl?k=" + maskedKey,
		}
		if v.URL2 != "" {
			item.URL2 = v.URL2
		}
		if v.URL3 != "" {
			item.URL3 = v.URL3
		}
		items = append(items, item)
	}
	return items
}
