package instagram

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

	if !validator.IsValidURL(req.URL) || !validator.IsAllowedDomain(req.URL, "instagram") {
		response.ErrorWithCode(c, 400, "INVALID_URL", "URL not supported for this endpoint.")
		return
	}

	// cek cache dulu
	if cached, err := downloader.CacheGet[mediaresponse.TikTokResponse]("content", "instagram", req.URL); err == nil && cached != nil {
		for i, v := range cached.Download.Video {
			ext := "mp4"
			cached.Download.Video[i].Server1 = downloader.GenerateServer1URL(
				h.downloadWorkerURL, h.downloadWorkerSecret, h.workerXORKey,
				v.Original, cached.Data.Title, "", "", ext, "content",
			)
			cached.Download.Video[i].Server2 = downloader.GenerateServer2URL(
				h.appURL, h.streamSecret,
				v.Original, cached.Data.Title, "", "", ext, "content",
			)
		}
		if cached.Download.Audio != nil {
			cached.Download.Audio.Server1 = downloader.GenerateServer1URL(
				h.downloadWorkerURL, h.downloadWorkerSecret, h.workerXORKey,
				cached.Download.Audio.Original, cached.Data.Title, "", "", "mp3", "content",
			)
			cached.Download.Audio.Server2 = downloader.GenerateServer2URL(
				h.appURL, h.streamSecret,
				cached.Download.Audio.Original, cached.Data.Title, "", "", "mp3", "content",
			)
		}
		writeJSONUnescaped(c, http.StatusOK, cached)
		return
	}

	result, err := h.service.Extract(req.URL)
	if err != nil {
		log.Printf("[instagram] extract error: %v", err)
		response.ErrorWithCode(c, 500, "EXTRACTION_FAILED", "Failed to extract media. Please check the URL and try again.")
		return
	}

	videos := []mediaresponse.TikTokVideoQuality{}
	for _, v := range result.Videos {
		ext := v.Extension
		if ext == "" {
			ext = "mp4"
		}
		server1 := downloader.GenerateServer1URL(
			h.downloadWorkerURL, h.downloadWorkerSecret, h.workerXORKey,
			v.URL, result.Title, "", "", ext, "content",
		)
		server2 := downloader.GenerateServer2URL(
			h.appURL, h.streamSecret,
			v.URL, result.Title, "", "", ext, "content",
		)
		videos = append(videos, mediaresponse.TikTokVideoQuality{
			Quality:  v.Quality,
			Original: v.URL,
			Server1:  server1,
			Server2:  server2,
		})
	}

	var audio *mediaresponse.TikTokAudio
	if result.AudioURL != "" {
		audioExt := result.AudioExt
		if audioExt == "" {
			audioExt = "mp3"
		}
		audio = &mediaresponse.TikTokAudio{
			Original: result.AudioURL,
			Server1: downloader.GenerateServer1URL(
				h.downloadWorkerURL, h.downloadWorkerSecret, h.workerXORKey,
				result.AudioURL, result.Title, "", "", audioExt, "content",
			),
			Server2: downloader.GenerateServer2URL(
				h.appURL, h.streamSecret,
				result.AudioURL, result.Title, "", "", audioExt, "content",
			),
		}
	}

	mediaType := "mp4"
	if len(result.Videos) == 0 && result.AudioURL != "" {
		mediaType = result.AudioExt
	}

	res := mediaresponse.TikTokResponse{
		Success:  true,
		Services: "content",
		Sites:    "instagram",
		Type:     mediaType,
		Data: mediaresponse.TikTokData{
			ID:        result.ID,
			Title:     result.Title,
			Thumbnail: result.Thumbnail,
			Duration:  result.Duration,
			Author: mediaresponse.Author{
				Name:     result.Author.Name,
				Username: result.Author.Username,
			},
		},
		Download: mediaresponse.TikTokDownload{
			Video: videos,
			Audio: audio,
		},
	}

	// simpan ke cache TANPA server_1 dan server_2
	cacheRes := res
	for i := range cacheRes.Download.Video {
		cacheRes.Download.Video[i].Server1 = ""
		cacheRes.Download.Video[i].Server2 = ""
	}
	if cacheRes.Download.Audio != nil {
		audioCache := *cacheRes.Download.Audio
		audioCache.Server1 = ""
		audioCache.Server2 = ""
		cacheRes.Download.Audio = &audioCache
	}
	downloader.CacheSet("content", "instagram", req.URL, &cacheRes)

	writeJSONUnescaped(c, http.StatusOK, res)
}
