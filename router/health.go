package router

import (
	"vidbot-api/config"
	"vidbot-api/internal/health"

	"github.com/gin-gonic/gin"
)

func setupHealth(r *gin.Engine, cfg *config.Config) {
	healthHandler := health.NewHandler(
		cfg.MasterKey,
		cfg.CloudConvertAPIKey,
		cfg.ConvertioAPIKey,
		cfg.WorkerURLs,
		cfg.WorkerSecret,
	)
	r.GET("/health", healthHandler.Check)
}
