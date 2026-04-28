package router

import (
	"vidbot-api/config"
	ebookshandler "vidbot-api/internal/services/downloader/ebooks"
	"vidbot-api/middleware"

	"github.com/gin-gonic/gin"
)

func setupEbooks(r *gin.Engine, cfg *config.Config) {
	h := ebookshandler.NewHandler(cfg.AppURL)

	// ── Public endpoints ──────────────────────────────────────────────────────
	ebooksGroup := r.Group("/downloader/ebooks",
		middleware.RequireAPIKey(),
		middleware.RequireAccessToken(cfg.MagicString),
		middleware.RateLimit("downloader"),
		middleware.FeatureFlag("downloader"),
		middleware.FeatureFlagPlatform("downloader", "ebooks"),
	)
	{
		ebooksGroup.POST("/search", h.Search)
		ebooksGroup.GET("/genre", h.Genres)
		ebooksGroup.GET("/genre/:genre", h.BrowseByGenre)
		ebooksGroup.GET("/author", h.Authors)
		ebooksGroup.GET("/author/:author", h.BrowseByAuthor)
	}

	// ── Download redirect — public, tidak butuh auth ──────────────────────────
	r.GET("/downloader/ebooks/dl", h.Download)

	// ── Admin CRUD ────────────────────────────────────────────────────────────
	adminEbooks := r.Group("/admin/downloader/ebooks",
		middleware.RequireAdminAuth(cfg.MasterKey),
	)
	{
		adminEbooks.GET("", h.AdminList)
		adminEbooks.POST("", h.AdminAdd)
		adminEbooks.POST("/bulk", h.AdminBulkAdd)
		adminEbooks.GET("/:id", h.AdminGet)
		adminEbooks.PATCH("/:id", h.AdminEdit)
		adminEbooks.PATCH("/:id/links", h.AdminEditLinks)
		adminEbooks.DELETE("/:id", h.AdminDelete)
	}
}
