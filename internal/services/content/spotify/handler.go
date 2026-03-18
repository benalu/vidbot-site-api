package spotify

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

	if !validator.IsValidURL(req.URL) || !validator.IsAllowedDomain(req.URL, "spotify") {
		response.ErrorWithCode(c, 400, "INVALID_URL", "URL not supported for this endpoint.")
		return
	}

	if cached, err := downloader.CacheGet[mediaresponse.SpotifyResponse]("content", "spotify", req.URL); err == nil && cached != nil {
		cached.Download.Server1 = downloader.GenerateServer1URL(
			h.downloadWorkerURL, h.downloadWorkerSecret, h.workerXORKey,
			cached.Download.Original, cached.Data.Title, "", "", cached.Type, "content",
		)
		cached.Download.Server2 = downloader.GenerateServer2URL(
			h.appURL, h.streamSecret,
			cached.Download.Original, cached.Data.Title, "", "", cached.Type, "content",
		)
		writeJSONUnescaped(c, http.StatusOK, cached)
		return
	}

	result, err := h.service.Extract(req.URL)
	if err != nil {
		log.Printf("[spotify] extract error: %v", err)
		response.ErrorWithCode(c, 500, "EXTRACTION_FAILED", "Failed to extract media. Please check the URL and try again.")
		return
	}

	ext := result.AudioExt
	if ext == "" {
		ext = "mp3"
	}

	res := mediaresponse.SpotifyResponse{
		Success:  true,
		Services: "content",
		Sites:    "spotify",
		Type:     ext,
		Data: mediaresponse.SpotifyData{
			URL:       result.URL,
			Title:     result.Title,
			Author:    result.Author.Name,
			Thumbnail: result.Thumbnail,
			Duration:  result.Duration,
			TrackID:   result.TrackID,
			Quality:   result.Quality,
		},
		Download: mediaresponse.SpotifyDownload{
			Original: result.AudioURL,
		},
	}

	// simpan ke cache TANPA server_1 dan server_2
	downloader.CacheSet("content", "spotify", req.URL, &res)

	// tambah server_1 dan server_2 setelah cache
	res.Download.Server1 = downloader.GenerateServer1URL(
		h.downloadWorkerURL, h.downloadWorkerSecret, h.workerXORKey,
		result.AudioURL, result.Title, "", "", ext, "content",
	)
	res.Download.Server2 = downloader.GenerateServer2URL(
		h.appURL, h.streamSecret,
		result.AudioURL, result.Title, "", "", ext, "content",
	)

	writeJSONUnescaped(c, http.StatusOK, res)
}
