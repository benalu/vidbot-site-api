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
	)
	{
		group.POST("/videb", videbHandler.Extract)
		group.POST("/vidoy", vidoyHandler.Extract)
		group.POST("/vidbos", vidbosHandler.Extract)
		group.POST("/vidarato", vidaratoHandler.Extract)
		group.POST("/vidnest", vidnestHandler.Extract)
	}
}
