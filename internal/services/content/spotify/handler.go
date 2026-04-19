package spotify

import (
	"vidbot-api/internal/services/content/provider"
	"vidbot-api/pkg/downloader"
	"vidbot-api/pkg/httputil"
	"vidbot-api/pkg/mediaresponse"
	"vidbot-api/pkg/response"
	"vidbot-api/pkg/stats"
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

func (h *Handler) Extract(c *gin.Context) {
	stats.Platform(c, "content", "spotify")
	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.WriteMsg(c, response.ErrBadRequest, "Url is required.")
		return
	}

	if !validator.IsValidURL(req.URL) || !validator.IsAllowedDomain(req.URL, "spotify") {
		response.InvalidURLWarn(c, "content", "spotify", req.URL)
		return
	}

	cacheKey := downloader.CacheKey("content", "spotify", req.URL)

	if cached, err := downloader.CacheGet[mediaresponse.SpotifyResponse]("content", "spotify", req.URL); err == nil && cached != nil {
		cached.Download.Server1 = downloader.GenerateServer1URL(
			h.downloadWorkerURL, h.downloadWorkerSecret, h.workerXORKey,
			cached.Download.Original, cached.Data.Title, "", "", cached.Type, "content",
		)
		cached.Download.Server2 = downloader.GenerateServer2URL(
			h.appURL, h.streamSecret, cacheKey,
			cached.Download.Original, cached.Data.Title, "", "", cached.Type, "content",
		)
		httputil.WriteJSONOK(c, cached)
		return
	}

	result, err := h.service.Extract(req.URL)
	if err != nil {
		response.Extraction(c, "content", "spotify", err)
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
		h.appURL, h.streamSecret, cacheKey,
		result.AudioURL, result.Title, "", "", ext, "content",
	)

	httputil.WriteJSONOK(c, res)
}
