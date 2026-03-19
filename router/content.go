package router

import (
	"vidbot-api/config"
	"vidbot-api/internal/services/content/instagram"
	"vidbot-api/internal/services/content/spotify"
	"vidbot-api/internal/services/content/threads"
	"vidbot-api/internal/services/content/tiktok"
	"vidbot-api/internal/services/content/twitter"
	"vidbot-api/middleware"

	"github.com/gin-gonic/gin"
)

func setupContent(r *gin.Engine, cfg *config.Config, providers contentProviderSet) {
	spotifyHandler := spotify.NewHandler(
		providers.spotify,
		cfg.DownloadWorkerURL, cfg.DownloadWorkerSecret,
		cfg.WorkerPayloadXORKey, cfg.AppURL, cfg.StreamSecret,
	)
	tiktokHandler := tiktok.NewHandler(
		providers.tiktok,
		cfg.DownloadWorkerURL, cfg.DownloadWorkerSecret,
		cfg.WorkerPayloadXORKey, cfg.AppURL, cfg.StreamSecret,
	)
	instagramHandler := instagram.NewHandler(
		providers.instagram,
		cfg.DownloadWorkerURL, cfg.DownloadWorkerSecret,
		cfg.WorkerPayloadXORKey, cfg.AppURL, cfg.StreamSecret,
	)
	twitterHandler := twitter.NewHandler(
		providers.twitter,
		cfg.DownloadWorkerURL, cfg.DownloadWorkerSecret,
		cfg.WorkerPayloadXORKey, cfg.AppURL, cfg.StreamSecret,
	)
	threadsHandler := threads.NewHandler(
		providers.threads,
		cfg.DownloadWorkerURL, cfg.DownloadWorkerSecret,
		cfg.WorkerPayloadXORKey, cfg.AppURL, cfg.StreamSecret,
	)

	group := r.Group("/content",
		middleware.RequireAPIKey(),
		middleware.RequireAccessToken(cfg.MagicString),
		middleware.RateLimit("content"),
	)
	{
		group.POST("/spotify", spotifyHandler.Extract)
		group.POST("/tiktok", tiktokHandler.Extract)
		group.POST("/instagram", instagramHandler.Extract)
		group.POST("/twitter", twitterHandler.Extract)
		group.POST("/threads", threadsHandler.Extract)
	}
}
