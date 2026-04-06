package app

import (
	"fmt"
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

// ─── Search ───────────────────────────────────────────────────────────────────

type searchRequest struct {
	APK string `json:"apk"`
}

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

// SearchAndroid — POST /app/android
func (h *Handler) SearchAndroid(c *gin.Context) {
	stats.Platform(c, "app", "android")
	h.search(c, "android")
}

// SearchWindows — POST /app/windows
func (h *Handler) SearchWindows(c *gin.Context) {
	stats.Platform(c, "app", "windows")
	h.search(c, "windows")
}

// BrowseAndroid — GET /app/android/:category
func (h *Handler) BrowseAndroid(c *gin.Context) {
	stats.Platform(c, "app", "android")
	h.browseByCategory(c, "android")
}

// CategoriesAndroid — GET /app/android/category
func (h *Handler) CategoriesAndroid(c *gin.Context) {
	stats.Platform(c, "app", "android")
	h.getCategories(c, "android")
}

// CategoriesWindows — GET /app/windows/category
func (h *Handler) CategoriesWindows(c *gin.Context) {
	stats.Platform(c, "app", "windows")
	h.getCategories(c, "windows")
}

func (h *Handler) getCategories(c *gin.Context, platform string) {
	categories, err := appstore.GetCategories(platform)
	if err != nil {
		response.ErrorWithCode(c, http.StatusInternalServerError, "DB_ERROR", "gagal membaca database")
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

// BrowseWindows — GET /app/windows/:category
func (h *Handler) BrowseWindows(c *gin.Context) {
	stats.Platform(c, "app", "windows")
	h.browseByCategory(c, "windows")
}

func (h *Handler) browseByCategory(c *gin.Context, platform string) {
	category := strings.TrimSpace(c.Param("category"))
	if category == "" {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "category wajib diisi")
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
		response.ErrorWithCode(c, http.StatusInternalServerError, "DB_ERROR", "gagal membaca database")
		return
	}
	if total == 0 {
		response.ErrorWithCode(c, http.StatusNotFound, "NOT_FOUND",
			fmt.Sprintf("category '%s' tidak ditemukan", category))
		return
	}

	base := strings.TrimRight(h.appURL, "/")
	items := make([]appItem, 0, len(apps))
	for _, a := range apps {
		items = append(items, appItem{
			Slug:         a.Slug,
			Name:         a.Name,
			Category:     a.Category,
			Overview:     a.Overview,
			Requirements: a.Requirements,
			Image:        a.Image,
			Download:     buildDownloadItems(a.Downloads, base),
		})
	}

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

func buildDownloadItems(downloads []appstore.Download, base string) []downloadItem {
	versionOrder := []string{}
	versionMap := map[string][]variantItem{}
	for _, d := range downloads {
		masked, err := appstore.MaskURL(d.RawURL)
		if err != nil {
			continue
		}
		if _, exists := versionMap[d.Version]; !exists {
			versionOrder = append(versionOrder, d.Version)
		}
		versionMap[d.Version] = append(versionMap[d.Version], variantItem{
			Variant: d.Variant,
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

func (h *Handler) search(c *gin.Context, platform string) {
	var body map[string]string
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "request body tidak valid")
		return
	}

	keyword := strings.TrimSpace(body["apk"])
	if keyword == "" {
		keyword = strings.TrimSpace(body["app"])
	}

	if keyword == "" {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "required name parameter (apk/app) is missing")
		return
	}
	if len(keyword) < 3 {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "too short keyword, minimum 3 characters")
		return
	}

	apps, err := appstore.Search(platform, keyword)
	if err != nil {
		response.ErrorWithCode(c, http.StatusInternalServerError, "DB_ERROR", "gagal membaca database")
		return
	}

	base := strings.TrimRight(h.appURL, "/")
	items := make([]appItem, 0, len(apps))
	for _, a := range apps {
		items = append(items, appItem{
			Slug:         a.Slug,
			Name:         a.Name,
			Category:     a.Category,
			Overview:     a.Overview,
			Requirements: a.Requirements,
			Image:        a.Image,
			Download:     buildDownloadItems(a.Downloads, base),
		})
	}

	httputil.WriteJSONOK(c, searchResponse{
		Success:  true,
		Services: "app",
		Platform: platform,
		Total:    len(items),
		Data:     items,
	})
}

// ─── Redirect shortlink ───────────────────────────────────────────────────────

// Download — GET /app/dl?k={key}
// Redirect ke raw URL. Tidak butuh auth — URL sudah di-mask.
func (h *Handler) Download(c *gin.Context) {
	key := strings.TrimSpace(c.Query("k"))
	if key == "" {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "parameter k wajib diisi")
		return
	}

	rawURL, err := appstore.ResolveURL(key)
	if err != nil {
		response.ErrorWithCode(c, http.StatusNotFound, "NOT_FOUND", "link tidak ditemukan atau sudah kedaluwarsa")
		return
	}

	c.Redirect(http.StatusFound, rawURL)
}
