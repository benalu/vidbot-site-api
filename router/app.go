package router

import (
	"vidbot-api/config"
	apphandler "vidbot-api/internal/services/app"
	"vidbot-api/middleware"
	"vidbot-api/pkg/cdnstore"

	"github.com/gin-gonic/gin"
)

func setupApp(r *gin.Engine, cfg *config.Config) {
	// CDN resolver — nil kalau tidak dikonfigurasi (graceful degradation)
	var cdnResolver *cdnstore.Resolver
	if cfg.CDNAPIKey != "" {
		cdnClient := cdnstore.NewClient(cfg.CDNAPIKey, cfg.CDNFolderID)
		cdnResolver = cdnstore.NewResolver(cdnClient)
	}

	h := apphandler.NewHandler(cfg.AppURL, cdnResolver)

	// ── Public search ─────────────────────────────────────────────────────────
	appGroup := r.Group("/app",
		middleware.RequireAPIKey(),
		middleware.RequireAccessToken(cfg.MagicString),
		middleware.RateLimit("app"),
		middleware.FeatureFlag("app"),
	)
	{
		appGroup.POST("/android", middleware.FeatureFlagPlatform("app", "android"), h.SearchAndroid)
		appGroup.GET("/android/category", middleware.FeatureFlagPlatform("app", "android"), h.CategoriesAndroid)
		appGroup.GET("/android/category/:category", middleware.FeatureFlagPlatform("app", "android"), h.BrowseAndroid)
		appGroup.POST("/windows", middleware.FeatureFlagPlatform("app", "windows"), h.SearchWindows)
		appGroup.GET("/windows/category", middleware.FeatureFlagPlatform("app", "windows"), h.CategoriesWindows)
		appGroup.GET("/windows/category/:category", middleware.FeatureFlagPlatform("app", "windows"), h.BrowseWindows)
	}

	// ── Download redirect ─────────────────────────────────────────────────────
	r.GET("/app/dl", h.Download)

	// ── Admin CRUD ────────────────────────────────────────────────────────────
	adminApp := r.Group("/admin/apps", middleware.RequireAdminAuth(cfg.MasterKey))
	{
		adminApp.GET("/:platform", h.AdminList)
		adminApp.POST("/:platform", h.AdminAdd)
		adminApp.POST("/:platform/bulk", h.AdminBulkAdd)
		adminApp.DELETE("/:platform/:slug", h.AdminDelete)
		adminApp.DELETE("/:platform/versions/:id", h.AdminDeleteVersion)
		adminApp.POST("/:platform/cdn/invalidate", h.AdminInvalidateCDNCache)
	}
}
