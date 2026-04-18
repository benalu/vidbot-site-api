package router

import (
	"time"
	"vidbot-api/config"
	"vidbot-api/internal/admin"
	"vidbot-api/internal/health"
	"vidbot-api/middleware"

	"github.com/gin-gonic/gin"
)

func setupAdmin(r *gin.Engine, cfg *config.Config, healthHandler *health.Handler) {
	adminHandler := admin.NewHandler(cfg.MasterKey, healthHandler)

	// auth — tidak butuh middleware
	r.POST("/admin/auth/login",
		middleware.AdminLoginRateLimit(5, time.Minute),
		adminHandler.Login,
	)

	adminGroup := r.Group("/admin", middleware.RequireAdminAuth(cfg.MasterKey))
	{
		// auth
		adminGroup.POST("/auth/logout", adminHandler.Logout)
		adminGroup.GET("/auth/me", adminHandler.Me)

		// keys
		adminGroup.POST("/keys", adminHandler.CreateKey)
		adminGroup.GET("/keys", adminHandler.ListKeys)
		adminGroup.POST("/keys/lookup", adminHandler.LookupKey)
		adminGroup.DELETE("/keys/:keyHash", adminHandler.RevokeKey)
		adminGroup.POST("/keys/:keyHash/topup", adminHandler.TopUpQuota)
		adminGroup.GET("/keys/:keyHash/usage", adminHandler.GetKeyUsage)

		// feature flags
		adminGroup.GET("/features", adminHandler.GetFeatures)
		adminGroup.PUT("/features/:group", adminHandler.ToggleFeature)
		adminGroup.PUT("/features/:group/:platform", adminHandler.ToggleFeaturePlatform)

		// stats
		adminGroup.GET("/stats", adminHandler.GetStats)
		adminGroup.GET("/stats/realtime", adminHandler.GetRealtimeStats)
		adminGroup.GET("/stats/errors", adminHandler.GetErrorStats)

		// system
		adminGroup.GET("/system/health", adminHandler.GetHealth)
		adminGroup.GET("/system/redis", adminHandler.GetRedisStats)
		adminGroup.GET("/system/queue", adminHandler.GetSystemQueue)
		adminGroup.GET("/system/sessions", adminHandler.GetActiveSessions)
		adminGroup.DELETE("/system/sessions/:sessionId", adminHandler.RevokeSession)
	}
}
