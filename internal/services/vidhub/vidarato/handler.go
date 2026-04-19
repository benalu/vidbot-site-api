package vidarato

import (
	"log/slog"
	"net/http"
	"vidbot-api/pkg/downloader"
	"vidbot-api/pkg/mediaresponse"
	"vidbot-api/pkg/proxy"
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
	toolsDir             string
}

func NewHandler(proxyClient *proxy.Client, downloadWorkerURL, downloadWorkerSecret, workerXORKey, appURL, streamSecret, toolsDir string) *Handler {
	return &Handler{
		service:              NewService(proxyClient),
		downloadWorkerURL:    downloadWorkerURL,
		downloadWorkerSecret: downloadWorkerSecret,
		workerXORKey:         workerXORKey,
		appURL:               appURL,
		streamSecret:         streamSecret,
		toolsDir:             toolsDir,
	}
}

type Request struct {
	URL string `json:"url" binding:"required"`
}

func (h *Handler) Extract(c *gin.Context) {
	stats.Platform(c, "vidhub", "vidarato")
	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "URL is required.")
		return
	}

	if !validator.IsValidURL(req.URL) || !validator.IsAllowedDomain(req.URL, "vidarato") {
		slog.Warn("invalid or disallowed url attempt",
			"group", "vidhub",
			"platform", "vidarato",
			"url", req.URL,
		)
		response.ErrorWithCode(c, http.StatusBadRequest, "INVALID_URL", "URL not supported for this endpoint.")
		return
	}

	cacheKey := downloader.CacheKey("vidhub", "vidarato", req.URL)

	if cached, err := downloader.CacheGet[mediaresponse.VidhubResponse]("vidhub", "vidarato", req.URL); err == nil && cached != nil {
		ext := downloader.MediaTypeToExt(downloader.VideoType(cached.Type))
		cdnOrigin := downloader.ExtractCDNOrigin(cached.Download.Original) // ← derive langsung dari URL
		cached.Download.Server1 = downloader.GenerateServer1HLSURL(
			h.downloadWorkerURL, h.downloadWorkerSecret, h.workerXORKey,
			cached.Download.Original, cached.Data.Title, cached.Data.Filename,
			cached.Data.Filecode, ext, "vidhub", cdnOrigin,
		)
		cached.Download.Server2 = downloader.GenerateServer2HLSURL(
			h.appURL, h.streamSecret, cacheKey,
			cached.Download.Original, cached.Data.Title, cached.Data.Filename,
			cached.Data.Filecode, ext, "vidhub", cdnOrigin,
		)
		response.WriteJSON(c, http.StatusOK, cached)
		return
	}
	result, err := h.service.Extract(req.URL)
	if err != nil {
		slog.Error("extract failed", "group", "vidhub", "platform", "vidarato", "error", err)
		stats.TrackError(c, "vidhub", "vidarato", "EXTRACTION_FAILED")
		response.ErrorWithCode(c, 500, "EXTRACTION_FAILED", "Unable to process the requested URL. The content may be unavailable or the link has expired.")
		return
	}

	mediaType := downloader.DetectMediaType(result.M3U8URL)
	ext := downloader.MediaTypeToExt(mediaType)

	res := mediaresponse.VidhubResponse{
		Success:  true,
		Services: "vidhub",
		Sites:    "vidarato",
		Type:     mediaType,
		Data: mediaresponse.VidhubData{
			Filecode:  result.Filecode,
			Title:     result.Title,
			Filename:  result.Filename,
			Thumbnail: result.Thumbnail,
		},
		Download: mediaresponse.DownloadLinks{
			Original: result.M3U8URL,
		},
	}

	downloader.CacheSet("vidhub", "vidarato", req.URL, &res)

	res.Download.Server1 = downloader.GenerateServer1HLSURL(
		h.downloadWorkerURL, h.downloadWorkerSecret, h.workerXORKey,
		result.M3U8URL, result.Title, result.Filename, result.Filecode, ext, "vidhub", result.CDNOrigin,
	)
	res.Download.Server2 = downloader.GenerateServer2HLSURL(
		h.appURL, h.streamSecret, cacheKey,
		result.M3U8URL, result.Title, result.Filename, result.Filecode, ext, "vidhub", result.CDNOrigin,
	)

	response.WriteJSON(c, http.StatusOK, res)
}
