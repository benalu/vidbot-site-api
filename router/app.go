package router

import (
	"vidbot-api/config"
	apphandler "vidbot-api/internal/services/app"
	"vidbot-api/middleware"

	"github.com/gin-gonic/gin"
)

func setupApp(r *gin.Engine, cfg *config.Config) {
	h := apphandler.NewHandler(cfg.AppURL)

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
	adminApp := r.Group("/admin/app", validateMasterKeyMW(cfg.MasterKey))
	{
		adminApp.GET("/:platform/list", h.AdminList)
		adminApp.POST("/:platform/add", h.AdminAdd)
		adminApp.POST("/:platform/bulk", h.AdminBulkAdd)
		adminApp.DELETE("/:platform/app/:slug", h.AdminDelete)
		adminApp.DELETE("/:platform/version/:id", h.AdminDeleteVersion)
	}
}

// validateMasterKeyMW — inline middleware cek X-Master-Key,
// konsisten dengan cara admin/handler.go tanpa perlu duplikat struct.
func validateMasterKeyMW(masterKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("X-Master-Key") != masterKey {
			c.AbortWithStatusJSON(401, gin.H{
				"success": false,
				"code":    "UNAUTHORIZED",
				"message": "Invalid master key.",
			})
			return
		}
		c.Next()
	}
}
