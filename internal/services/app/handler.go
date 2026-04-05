package app

import (
	"net/http"
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
		// group downloads by version
		versionOrder := []string{}
		versionMap := map[string][]variantItem{}
		for _, d := range a.Downloads {
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
