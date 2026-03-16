package threads

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"vidbot-api/internal/services/content/provider"
	"vidbot-api/pkg/downloader"
	"vidbot-api/pkg/mediaresponse"
	"vidbot-api/pkg/response"
	"vidbot-api/pkg/validator"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service              *Service
	downloadWorkerURL    string
	downloadWorkerSecret string
	workerXORKey         string
	appURL               string
	streamSecret         string
}

func NewHandler(
	providers []provider.Provider,
	downloadWorkerURL, downloadWorkerSecret, workerXORKey, appURL, streamSecret string,
) *Handler {
	return &Handler{
		service:              NewService(providers),
		downloadWorkerURL:    downloadWorkerURL,
		downloadWorkerSecret: downloadWorkerSecret,
		workerXORKey:         workerXORKey,
		appURL:               appURL,
		streamSecret:         streamSecret,
	}
}

type Request struct {
	URL string `json:"url" binding:"required"`
}

func writeJSONUnescaped(c *gin.Context, status int, data interface{}) {
	buf := &bytes.Buffer{}
	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	encoder.Encode(data)
	c.Data(status, "application/json; charset=utf-8", buf.Bytes())
}

func (h *Handler) Extract(c *gin.Context) {
	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "url is required")
		return
	}

	if !validator.IsValidURL(req.URL) || !validator.IsAllowedDomain(req.URL, "threads") {
		response.ErrorWithCode(c, 400, "INVALID_URL", "URL not supported for this endpoint.")
		return
	}

	if cached, err := downloader.CacheGet[mediaresponse.ThreadsResponse]("content", "threads", req.URL); err == nil && cached != nil {
		// regenerate server_1 dan server_2 untuk video
		for i, m := range cached.Download.Media {
			if m.Type == "video" {
				customTitle := cached.Data.Author
				cached.Download.Media[i].Server1 = downloader.GenerateServer1URL(
					h.downloadWorkerURL, h.downloadWorkerSecret, h.workerXORKey,
					m.Original, customTitle, "", "", "mp4", "content",
				)
				cached.Download.Media[i].Server2 = downloader.GenerateServer2URL(
					h.appURL, h.streamSecret,
					m.Original, customTitle, "", "", "mp4", "content",
				)
			}
		}
		writeJSONUnescaped(c, http.StatusOK, cached)
		return
	}

	result, err := h.service.Extract(req.URL)
	if err != nil {
		log.Printf("[threads] extract error: %v", err)
		response.ErrorWithCode(c, 500, "EXTRACTION_FAILED", "Failed to extract media. Please check the URL and try again.")
		return
	}

	mediaItems := []mediaresponse.ThreadsMediaItem{}
	for _, m := range result.MediaItems {
		item := mediaresponse.ThreadsMediaItem{
			Type:      m.Type,
			Original:  m.URL,
			Original1: m.URL2,
		}

		// server_1 dan server_2 hanya untuk video
		if m.Type == "video" {
			customTitle := result.Author.Name
			item.Server1 = downloader.GenerateServer1URL(
				h.downloadWorkerURL, h.downloadWorkerSecret, h.workerXORKey,
				m.URL, customTitle, "", "", m.Extension, "content",
			)
			item.Server2 = downloader.GenerateServer2URL(
				h.appURL, h.streamSecret,
				m.URL, customTitle, "", "", m.Extension, "content",
			)
		}

		mediaItems = append(mediaItems, item)
	}

	mediaType := resolveType(result.MediaItems)

	res := mediaresponse.ThreadsResponse{
		Success:  true,
		Services: "content",
		Sites:    "threads",
		Type:     mediaType,
		Data: mediaresponse.ThreadsData{
			URL:       result.URL,
			Author:    result.Author.Name,
			Title:     result.Title,
			Thumbnail: result.Thumbnail,
		},
		Download: mediaresponse.ThreadsDownload{
			Media: mediaItems,
		},
	}

	// cache — simpan tanpa server_1 dan server_2
	cacheItems := make([]mediaresponse.ThreadsMediaItem, len(res.Download.Media))
	for i, m := range res.Download.Media {
		cacheItems[i] = mediaresponse.ThreadsMediaItem{
			Type:      m.Type,
			Original:  m.Original,
			Original1: m.Original1,
		}
	}
	cacheRes := res
	cacheRes.Download.Media = cacheItems
	downloader.CacheSet("content", "threads", req.URL, &cacheRes)

	writeJSONUnescaped(c, http.StatusOK, res)
}
