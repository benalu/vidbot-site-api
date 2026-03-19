package router

import (
	"vidbot-api/config"
	audioconvert "vidbot-api/internal/services/convert/audio"
	docconvert "vidbot-api/internal/services/convert/document"
	fontsconvert "vidbot-api/internal/services/convert/fonts"
	imgconvert "vidbot-api/internal/services/convert/image"
	convertprovider "vidbot-api/internal/services/convert/provider"
	"vidbot-api/middleware"

	"github.com/gin-gonic/gin"
)

func setupConvert(r *gin.Engine, cfg *config.Config, providers []convertprovider.Provider) {
	audioHandler := audioconvert.NewHandler(
		providers,
		cfg.DownloadWorkerURL, cfg.DownloadWorkerSecret,
		cfg.WorkerPayloadXORKey, cfg.AppURL, cfg.StreamSecret,
	)
	docHandler := docconvert.NewHandler(
		providers,
		cfg.DownloadWorkerURL, cfg.DownloadWorkerSecret,
		cfg.WorkerPayloadXORKey, cfg.AppURL, cfg.StreamSecret,
	)
	imgHandler := imgconvert.NewHandler(
		providers,
		cfg.DownloadWorkerURL, cfg.DownloadWorkerSecret,
		cfg.WorkerPayloadXORKey, cfg.AppURL, cfg.StreamSecret,
	)
	fontsHandler := fontsconvert.NewHandler(
		providers,
		cfg.DownloadWorkerURL, cfg.DownloadWorkerSecret,
		cfg.WorkerPayloadXORKey, cfg.AppURL, cfg.StreamSecret,
	)

	group := r.Group("/convert",
		middleware.RequireAPIKey(),
		middleware.RequireAccessToken(cfg.MagicString),
		middleware.RateLimit("convert"),
	)
	{
		group.POST("/audio", audioHandler.Convert)
		group.POST("/audio/upload", audioHandler.Upload)
		group.POST("/document", docHandler.Convert)
		group.POST("/document/upload", docHandler.Upload)
		group.POST("/image", imgHandler.Convert)
		group.POST("/image/upload", imgHandler.Upload)
		group.POST("/fonts", fontsHandler.Convert)
		group.POST("/fonts/upload", fontsHandler.Upload)
		group.GET("/status/:job_id", audioHandler.Status)
	}
}
