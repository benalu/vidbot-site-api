package router

import (
	"vidbot-api/config"
	flachandler "vidbot-api/internal/services/downloader/flac"
	"vidbot-api/middleware"

	"github.com/gin-gonic/gin"
)

func setupDownloader(r *gin.Engine, cfg *config.Config) {
	h := flachandler.NewHandler(cfg.AppURL)

	// ── Public endpoints ──────────────────────────────────────────────────────
	dlGroup := r.Group("/downloader",
		middleware.RequireAPIKey(),
		middleware.RequireAccessToken(cfg.MagicString),
		middleware.RateLimit("downloader"),
		middleware.FeatureFlag("downloader"),
	)
	{
		flac := dlGroup.Group("/flac", middleware.FeatureFlagPlatform("downloader", "flac"))
		{
			flac.POST("/search", h.Search)
			flac.GET("/genre", h.Genres)
			flac.GET("/genre/:genre", h.BrowseByGenre)
		}
	}

	// ── Download redirect — public, tidak butuh auth ──────────────────────────
	r.GET("/downloader/dl", h.Download)

	// ── Admin CRUD ────────────────────────────────────────────────────────────
	adminDL := r.Group("/admin/downloader", middleware.RequireAdminAuth(cfg.MasterKey))
	{
		adminDL.GET("/flac", h.AdminList)
		adminDL.POST("/flac", h.AdminAdd)
		adminDL.POST("/flac/bulk", h.AdminBulkAdd)
		adminDL.PATCH("/flac/:id", h.AdminEdit)
		adminDL.DELETE("/flac/:id", h.AdminDelete)
	}
}
