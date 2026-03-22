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
		middleware.FeatureFlag("convert"),
	)
	{
		group.POST("/audio", middleware.FeatureFlagPlatform("convert", "audio"), audioHandler.Convert)
		group.POST("/audio/upload", middleware.FeatureFlagPlatform("convert", "audio"), audioHandler.Upload)
		group.POST("/document", middleware.FeatureFlagPlatform("convert", "document"), docHandler.Convert)
		group.POST("/document/upload", middleware.FeatureFlagPlatform("convert", "document"), docHandler.Upload)
		group.POST("/image", middleware.FeatureFlagPlatform("convert", "image"), imgHandler.Convert)
		group.POST("/image/upload", middleware.FeatureFlagPlatform("convert", "image"), imgHandler.Upload)
		group.POST("/fonts", middleware.FeatureFlagPlatform("convert", "fonts"), fontsHandler.Convert)
		group.POST("/fonts/upload", middleware.FeatureFlagPlatform("convert", "fonts"), fontsHandler.Upload)
		group.GET("/status/:job_id", audioHandler.Status)
	}
}
