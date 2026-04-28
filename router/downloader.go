package router

import (
	"vidbot-api/config"
	ebookshandler "vidbot-api/internal/services/downloader/ebooks"
	flachandler "vidbot-api/internal/services/downloader/flac"
	movieshandler "vidbot-api/internal/services/downloader/movies"
	"vidbot-api/middleware"
	"vidbot-api/pkg/tmdb"

	"github.com/gin-gonic/gin"
)

func setupDownloader(r *gin.Engine, cfg *config.Config) {
	flacH := flachandler.NewHandler(cfg.AppURL)
	ebooksH := ebookshandler.NewHandler(cfg.AppURL)

	var tmdbClient *tmdb.Client
	if cfg.TmdbAPIKey != "" {
		tmdbClient = tmdb.NewClient(cfg.TmdbAPIKey)
	}
	moviesH := movieshandler.NewHandler(cfg.AppURL, tmdbClient)

	// ── Base middleware group ──────────────────────────────────────────────────
	dlGroup := r.Group("/downloader",
		middleware.RequireAPIKey(),
		middleware.RequireAccessToken(cfg.MagicString),
		middleware.RateLimit("downloader"),
		middleware.FeatureFlag("downloader"),
	)
	{
		// flac
		flac := dlGroup.Group("/flac", middleware.FeatureFlagPlatform("downloader", "flac"))
		{
			flac.POST("/search", flacH.Search)
			flac.GET("/genre", flacH.Genres)
			flac.GET("/genre/:genre", flacH.BrowseByGenre)
			flac.GET("/artist", flacH.Artists)
			flac.GET("/artist/:artist", flacH.BrowseByArtist)
		}

		// ebooks
		ebooks := dlGroup.Group("/ebooks", middleware.FeatureFlagPlatform("downloader", "ebooks"))
		{
			ebooks.POST("/search", ebooksH.Search)
			ebooks.GET("/genre", ebooksH.Genres)
			ebooks.GET("/genre/:genre", ebooksH.BrowseByGenre)
			ebooks.GET("/author", ebooksH.Authors)
			ebooks.GET("/author/:author", ebooksH.BrowseByAuthor)
		}

		// movies
		movies := dlGroup.Group("/movies", middleware.FeatureFlagPlatform("downloader", "movies"))
		{
			movies.POST("/search", moviesH.Search)
			movies.GET("/genres", moviesH.Genres)
			movies.GET("/genres/:genre", moviesH.BrowseByGenre)
			movies.GET("/years", moviesH.Years)
			movies.GET("/years/:year", moviesH.BrowseByYear)
		}
	}

	// ── Download redirects — public, tidak butuh auth ─────────────────────────
	r.GET("/downloader/dl", flacH.Download)
	r.GET("/downloader/ebooks/dl", ebooksH.Download)
	r.GET("/downloader/movies/dl", moviesH.Download)

	// ── Admin CRUD ────────────────────────────────────────────────────────────
	adminDL := r.Group("/admin/downloader", middleware.RequireAdminAuth(cfg.MasterKey))
	{
		// flac
		adminDL.GET("/flac", flacH.AdminList)
		adminDL.GET("/flac/:id", flacH.AdminGet)
		adminDL.POST("/flac", flacH.AdminAdd)
		adminDL.POST("/flac/bulk", flacH.AdminBulkAdd)
		adminDL.PATCH("/flac/:id", flacH.AdminEdit)
		adminDL.PATCH("/flac/:id/links", flacH.AdminEditLinks)
		adminDL.DELETE("/flac/:id", flacH.AdminDelete)

		// ebooks
		adminDL.GET("/ebooks", ebooksH.AdminList)
		adminDL.GET("/ebooks/:id", ebooksH.AdminGet)
		adminDL.POST("/ebooks", ebooksH.AdminAdd)
		adminDL.POST("/ebooks/bulk", ebooksH.AdminBulkAdd)
		adminDL.PATCH("/ebooks/:id", ebooksH.AdminEdit)
		adminDL.PATCH("/ebooks/:id/links", ebooksH.AdminEditLinks)
		adminDL.DELETE("/ebooks/:id", ebooksH.AdminDelete)

		// movies
		adminDL.GET("/movies", moviesH.AdminList)
		adminDL.GET("/movies/:id", moviesH.AdminGet)
		adminDL.POST("/movies", moviesH.AdminAdd)
		adminDL.POST("/movies/bulk", moviesH.AdminBulkAdd)
		adminDL.PATCH("/movies/:id", moviesH.AdminEdit)
		adminDL.PATCH("/movies/:id/links", moviesH.AdminEditLinks)
		adminDL.DELETE("/movies/:id", moviesH.AdminDelete)
		adminDL.POST("/movies/:id/refresh-tmdb", moviesH.AdminRefreshTmdb)
	}
}
