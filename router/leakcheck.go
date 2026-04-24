package router

import (
	"path/filepath"
	"vidbot-api/config"
	"vidbot-api/internal/services/leakcheck"
	"vidbot-api/middleware"

	"github.com/gin-gonic/gin"
)

func setupLeakcheck(r *gin.Engine, cfg *config.Config) {
	leakcheckHandler := leakcheck.NewHandler(filepath.Join(cfg.DataDir, "leakcheck"))

	searchGroup := r.Group("/leakcheck",
		middleware.RequireAPIKey(),
		middleware.RequireAccessToken(cfg.MagicString),
		middleware.RateLimit("leakcheck"),
		middleware.FeatureFlag("leakcheck"),
	)
	{
		searchGroup.POST("/search", leakcheckHandler.Search)
		searchGroup.GET("/count", leakcheckHandler.Count)
	}

	// endpoint admin — pakai middleware RequireAdminAuth
	adminGroup := r.Group("/admin/leakcheck", middleware.RequireAdminAuth(cfg.MasterKey))
	{
		adminGroup.POST("/reload", leakcheckHandler.Reload)
		adminGroup.POST("/add-dir", leakcheckHandler.AddDir)
		adminGroup.GET("/stats", leakcheckHandler.Stats)
	}
}
