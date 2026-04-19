package threads

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
	stats.Platform(c, "content", "threads")
	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Write(c, response.ErrBadRequest)
		return
	}

	if !validator.IsValidURL(req.URL) || !validator.IsAllowedDomain(req.URL, "threads") {
		response.InvalidURLWarn(c, "content", "threads", req.URL)
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
					h.appURL, h.streamSecret, downloader.CacheKey("content", "threads", m.Original),
					m.Original, customTitle, "", "", "mp4", "content",
				)
			}
		}
		httputil.WriteJSONOK(c, cached)
		return
	}

	result, err := h.service.Extract(req.URL)
	if err != nil {
		response.Extraction(c, "content", "threads", err)
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
				h.appURL, h.streamSecret, downloader.CacheKey("content", "threads", m.URL),
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

	httputil.WriteJSONOK(c, res)
}
