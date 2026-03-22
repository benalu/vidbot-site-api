package router

import (
	"vidbot-api/config"
	"vidbot-api/internal/services/vidhub/vidarato"
	"vidbot-api/internal/services/vidhub/vidbos"
	"vidbot-api/internal/services/vidhub/videb"
	"vidbot-api/internal/services/vidhub/vidnest"
	"vidbot-api/internal/services/vidhub/vidoy"
	"vidbot-api/middleware"
	"vidbot-api/pkg/proxy"

	"github.com/gin-gonic/gin"
)

func setupVidhub(r *gin.Engine, cfg *config.Config, proxyClient *proxy.Client) {
	videbHandler := videb.NewHandler(
		proxyClient,
		cfg.DownloadWorkerURL, cfg.DownloadWorkerSecret,
		cfg.WorkerPayloadXORKey, cfg.AppURL, cfg.StreamSecret,
	)
	vidoyHandler := vidoy.NewHandler(
		proxyClient,
		cfg.DownloadWorkerURL, cfg.DownloadWorkerSecret,
		cfg.WorkerPayloadXORKey, cfg.AppURL, cfg.StreamSecret,
	)
	vidbosHandler := vidbos.NewHandler(
		cfg.DownloadWorkerURL, cfg.DownloadWorkerSecret,
		cfg.WorkerPayloadXORKey, cfg.AppURL, cfg.StreamSecret,
	)
	vidaratoHandler := vidarato.NewHandler(
		proxyClient,
		cfg.DownloadWorkerURL, cfg.DownloadWorkerSecret,
		cfg.WorkerPayloadXORKey, cfg.AppURL, cfg.StreamSecret,
		cfg.ToolsDir,
	)
	vidnestHandler := vidnest.NewHandler(
		proxyClient,
		cfg.DownloadWorkerURL, cfg.DownloadWorkerSecret,
		cfg.WorkerPayloadXORKey, cfg.AppURL, cfg.StreamSecret,
	)

	group := r.Group("/vidhub",
		middleware.RequireAPIKey(),
		middleware.RequireAccessToken(cfg.MagicString),
		middleware.RateLimit("vidhub"),
		middleware.FeatureFlag("vidhub"),
	)
	{
		group.POST("/videb", middleware.FeatureFlagPlatform("vidhub", "videb"), videbHandler.Extract)
		group.POST("/vidoy", middleware.FeatureFlagPlatform("vidhub", "vidoy"), vidoyHandler.Extract)
		group.POST("/vidbos", middleware.FeatureFlagPlatform("vidhub", "vidbos"), vidbosHandler.Extract)
		group.POST("/vidarato", middleware.FeatureFlagPlatform("vidhub", "vidarato"), vidaratoHandler.Extract)
		group.POST("/vidnest", middleware.FeatureFlagPlatform("vidhub", "vidnest"), vidnestHandler.Extract)
	}
}
