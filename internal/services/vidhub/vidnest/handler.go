package vidnest

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"vidbot-api/pkg/downloader"
	"vidbot-api/pkg/mediaresponse"
	"vidbot-api/pkg/proxy"
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

	if !validator.IsValidURL(req.URL) || !validator.IsAllowedDomain(req.URL, "vidnest") {
		response.ErrorWithCode(c, 400, "INVALID_URL", "URL not supported for this endpoint.")
		return
	}

	if cached, err := downloader.CacheGet[mediaresponse.VidhubResponse]("vidhub", "vidnest", req.URL); err == nil && cached != nil {
		ext := downloader.MediaTypeToExt(downloader.VideoType(cached.Type))
		cached.Download.Server1 = downloader.GenerateServer1URL(
			h.downloadWorkerURL, h.downloadWorkerSecret, h.workerXORKey,
			cached.Download.Original, cached.Data.Title, cached.Data.Filename, cached.Data.Filecode, ext, "vidhub",
		)
		cached.Download.Server2 = downloader.GenerateServer2URL(
			h.appURL, h.streamSecret,
			cached.Download.Original, cached.Data.Title, cached.Data.Filename, cached.Data.Filecode, ext, "vidhub",
		)
		writeJSONUnescaped(c, http.StatusOK, cached)
		return
	}

	result, err := h.service.Extract(req.URL)
	if err != nil {
		log.Printf("[vidnest] extract error: %v", err)
		response.ErrorWithCode(c, 500, "EXTRACTION_FAILED", "Failed to extract media. Please check the URL and try again.")
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
		h.appURL, h.streamSecret,
		result.DownloadURL, result.Title, result.Filename, result.Filecode, ext, "vidhub",
	)

	writeJSONUnescaped(c, http.StatusOK, res)
}
