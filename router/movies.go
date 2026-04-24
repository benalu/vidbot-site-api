package router

import (
	"vidbot-api/config"
	movieshandler "vidbot-api/internal/services/downloader/movies"
	"vidbot-api/middleware"
	"vidbot-api/pkg/tmdb"

	"github.com/gin-gonic/gin"
)

func setupMovies(r *gin.Engine, cfg *config.Config) {
	var tmdbClient *tmdb.Client
	if cfg.TmdbAPIKey != "" {
		tmdbClient = tmdb.NewClient(cfg.TmdbAPIKey)
	}

	h := movieshandler.NewHandler(cfg.AppURL, tmdbClient)

	// ── Public endpoints ──────────────────────────────────────────────────────
	moviesGroup := r.Group("/downloader/movies",
		middleware.RequireAPIKey(),
		middleware.RequireAccessToken(cfg.MagicString),
		middleware.RateLimit("downloader"),
		middleware.FeatureFlag("downloader"),
		middleware.FeatureFlagPlatform("downloader", "movies"),
	)
	{
		moviesGroup.POST("/search", h.Search)
		moviesGroup.GET("/genres", h.Genres)
		moviesGroup.GET("/genres/:genre", h.BrowseByGenre)
		moviesGroup.GET("/years", h.Years)
		moviesGroup.GET("/years/:year", h.BrowseByYear)
	}

	// ── Download redirect — public, tidak butuh auth ──────────────────────────
	r.GET("/downloader/movies/dl", h.Download)

	// ── Admin CRUD ────────────────────────────────────────────────────────────
	adminMovies := r.Group("/admin/downloader/movies",
		middleware.RequireAdminAuth(cfg.MasterKey),
	)
	{
		adminMovies.GET("", h.AdminList)
		adminMovies.POST("", h.AdminAdd)
		adminMovies.POST("/bulk", h.AdminBulkAdd)
		adminMovies.GET("/:id", h.AdminGet)
		adminMovies.PATCH("/:id", h.AdminEdit)
		adminMovies.PATCH("/:id/links", h.AdminEditLinks)
		adminMovies.DELETE("/:id", h.AdminDelete)
		adminMovies.POST("/:id/refresh-tmdb", h.AdminRefreshTmdb)
	}
}
