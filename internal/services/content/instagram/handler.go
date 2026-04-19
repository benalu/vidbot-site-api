package instagram

import (
	"fmt"
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
	stats.Platform(c, "content", "instagram")
	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.WriteMsg(c, response.ErrBadRequest, "Url is required.")
		return
	}

	if !validator.IsValidURL(req.URL) || !validator.IsAllowedDomain(req.URL, "instagram") {
		response.InvalidURLWarn(c, "content", "instagram", req.URL)
		return
	}

	if cached, err := downloader.CacheGet[mediaresponse.InstagramResponse]("content", "instagram", req.URL); err == nil && cached != nil {
		for i, v := range cached.Download.Video {
			ext := "mp4"
			customTitle := fmt.Sprintf("%s_%s", cached.Data.Author, v.Quality)
			cached.Download.Video[i].Server1 = downloader.GenerateServer1URL(
				h.downloadWorkerURL, h.downloadWorkerSecret, h.workerXORKey,
				v.Original, customTitle, "", "", ext, "content",
			)
			cached.Download.Video[i].Server2 = downloader.GenerateServer2URL(
				h.appURL, h.streamSecret, downloader.CacheKey("content", "instagram", v.Original),
				v.Original, customTitle, "", "", ext, "content",
			)
		}
		if cached.Download.Audio != nil {
			audioTitle := fmt.Sprintf("%s_audio", cached.Data.Author)
			cached.Download.Audio.Server1 = downloader.GenerateServer1URL(
				h.downloadWorkerURL, h.downloadWorkerSecret, h.workerXORKey,
				cached.Download.Audio.Original, audioTitle, "", "", "mp3", "content",
			)
			cached.Download.Audio.Server2 = downloader.GenerateServer2URL(
				h.appURL, h.streamSecret, downloader.CacheKey("content", "instagram", cached.Download.Audio.Original),
				cached.Download.Audio.Original, audioTitle, "", "", "mp3", "content",
			)
		}
		httputil.WriteJSONOK(c, cached)
		return
	}

	result, err := h.service.Extract(req.URL)
	if err != nil {
		response.Extraction(c, "content", "instagram", err)
		return
	}

	videos := []mediaresponse.ContentVideoQuality{}
	for _, v := range result.Videos {
		ext := v.Extension
		if ext == "" {
			ext = "mp4"
		}
		customTitle := fmt.Sprintf("%s_%s", result.Author.Name, v.Quality)
		server1 := downloader.GenerateServer1URL(
			h.downloadWorkerURL, h.downloadWorkerSecret, h.workerXORKey,
			v.URL, customTitle, "", "", ext, "content",
		)
		server2 := downloader.GenerateServer2URL(
			h.appURL, h.streamSecret, downloader.CacheKey("content", "instagram", v.URL),
			v.URL, customTitle, "", "", ext, "content",
		)
		videos = append(videos, mediaresponse.ContentVideoQuality{
			Quality:   v.Quality,
			Original:  v.URL,
			Original1: v.URL2,
			Server1:   server1,
			Server2:   server2,
		})
	}

	var audio *mediaresponse.ContentAudio
	if result.AudioURL != "" {
		audioExt := result.AudioExt
		if audioExt == "" {
			audioExt = "mp3"
		}
		audioTitle := fmt.Sprintf("%s_audio", result.Author.Name)
		audio = &mediaresponse.ContentAudio{
			Original:  result.AudioURL,
			Original1: result.AudioURL2,
			Server1: downloader.GenerateServer1URL(
				h.downloadWorkerURL, h.downloadWorkerSecret, h.workerXORKey,
				result.AudioURL, audioTitle, "", "", audioExt, "content",
			),
			Server2: downloader.GenerateServer2URL(
				h.appURL, h.streamSecret, downloader.CacheKey("content", "instagram", result.AudioURL),
				result.AudioURL, audioTitle, "", "", audioExt, "content",
			),
		}
	}

	mediaType := "mp4"
	if len(result.Videos) == 0 && result.AudioURL != "" {
		mediaType = result.AudioExt
	}

	res := mediaresponse.InstagramResponse{
		Success:  true,
		Services: "content",
		Sites:    "instagram",
		Type:     mediaType,
		Data: mediaresponse.InstagramData{
			URL:       result.URL,
			Username:  result.Author.Username,
			Author:    result.Author.Name,
			ViewCount: result.ViewCount,
			LikeCount: result.LikeCount,
			Duration:  mediaresponse.ParseDuration(result.Duration),
			Title:     result.Title,
			Thumbnail: result.Thumbnail,
		},
		Download: mediaresponse.ContentMultiDownload{
			Video: videos,
			Audio: audio,
		},
	}

	cacheVideos := make([]mediaresponse.ContentVideoQuality, len(res.Download.Video))
	for i, v := range res.Download.Video {
		cacheVideos[i] = mediaresponse.ContentVideoQuality{
			Quality:   v.Quality,
			Original:  v.Original,
			Original1: v.Original1,
		}
	}
	var cacheAudio *mediaresponse.ContentAudio
	if res.Download.Audio != nil {
		a := mediaresponse.ContentAudio{
			Original:  res.Download.Audio.Original,
			Original1: res.Download.Audio.Original1,
		}
		cacheAudio = &a
	}
	cacheRes := res
	cacheRes.Download.Video = cacheVideos
	cacheRes.Download.Audio = cacheAudio
	downloader.CacheSet("content", "instagram", req.URL, &cacheRes)

	httputil.WriteJSONOK(c, res)
}
