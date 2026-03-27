package vidnest

import (
	"log"
	"vidbot-api/pkg/downloader"
	"vidbot-api/pkg/httputil"
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
}

func NewHandler(proxyClient *proxy.Client, downloadWorkerURL, downloadWorkerSecret, workerXORKey, appURL, streamSecret string) *Handler {
	return &Handler{
		service:              NewService(proxyClient),
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
	stats.Platform(c, "vidhub", "vidnest")
	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, "url is required")
		return
	}

	if !validator.IsValidURL(req.URL) || !validator.IsAllowedDomain(req.URL, "vidnest") {
		response.ErrorWithCode(c, 400, "INVALID_URL", "URL not supported for this endpoint.")
		return
	}

	cacheKey := downloader.CacheKey("vidhub", "vidnest", req.URL)

	if cached, err := downloader.CacheGet[mediaresponse.VidhubResponse]("vidhub", "vidnest", req.URL); err == nil && cached != nil {
		ext := downloader.MediaTypeToExt(downloader.VideoType(cached.Type))
		cached.Download.Server1 = downloader.GenerateServer1URL(
			h.downloadWorkerURL, h.downloadWorkerSecret, h.workerXORKey,
			cached.Download.Original, cached.Data.Title, cached.Data.Filename, cached.Data.Filecode, ext, "vidhub",
		)
		cached.Download.Server2 = downloader.GenerateServer2URL(
			h.appURL, h.streamSecret, cacheKey,
			cached.Download.Original, cached.Data.Title, cached.Data.Filename, cached.Data.Filecode, ext, "vidhub",
		)
		httputil.WriteJSONOK(c, cached)
		return
	}

	result, err := h.service.Extract(req.URL)
	if err != nil {
		log.Printf("[vidnest] extract error: %v", err)
		response.ErrorWithCode(c, 500, "EXTRACTION_FAILED", "Unable to process the requested URL. The content may be unavailable or the link has expired.")
		return
	}

	mediaType := downloader.DetectMediaType(result.DownloadURL)
	ext := downloader.MediaTypeToExt(mediaType)

	res := mediaresponse.VidhubResponse{
		Success:  true,
		Services: "vidhub",
		Sites:    "vidnest",
		Type:     mediaType,
		Data: mediaresponse.VidhubData{
			Filecode:  result.Filecode,
			Title:     result.Title,
			Filename:  result.Filename,
			Thumbnail: result.Thumbnail,
			Size:      result.Size,
		},
		Download: mediaresponse.DownloadLinks{
			Original: result.DownloadURL,
		},
	}

	downloader.CacheSet("vidhub", "vidnest", req.URL, &res)

	res.Download.Server1 = downloader.GenerateServer1URL(
		h.downloadWorkerURL, h.downloadWorkerSecret, h.workerXORKey,
		result.DownloadURL, result.Title, result.Filename, result.Filecode, ext, "vidhub",
	)
	res.Download.Server2 = downloader.GenerateServer2URL(
		h.appURL, h.streamSecret, cacheKey,
		result.DownloadURL, result.Title, result.Filename, result.Filecode, ext, "vidhub",
	)

	httputil.WriteJSONOK(c, res)
}
