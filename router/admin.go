package router

import (
	"time"
	"vidbot-api/config"
	"vidbot-api/internal/admin"
	"vidbot-api/middleware"

	"github.com/gin-gonic/gin"
)

func setupAdmin(r *gin.Engine, cfg *config.Config) {
	adminHandler := admin.NewHandler(cfg.MasterKey)
	r.POST(
		"/admin/auth/login",
		middleware.AdminLoginRateLimit(5, time.Minute),
		adminHandler.Login,
	)
	adminGroup := r.Group("/admin")
	{
		// key management
		adminGroup.POST("/keys", adminHandler.CreateKey)
		adminGroup.DELETE("/keys/:key", adminHandler.RevokeKey)
		adminGroup.GET("/keys", adminHandler.ListKeys)
		adminGroup.POST("/keys/:key/topup", adminHandler.TopUpQuota)
		adminGroup.GET("/keys/:key/usage", adminHandler.GetKeyUsage)
		adminGroup.POST("/keys/lookup", adminHandler.LookupKey)

		// feature flags — group level
		adminGroup.GET("/features", adminHandler.GetFeatures)
		adminGroup.GET("/features/:group/enable", adminHandler.EnableFeature)
		adminGroup.GET("/features/:group/disable", adminHandler.DisableFeature)

		// feature flags — platform level
		adminGroup.GET("/features/:group/:platform/enable", adminHandler.EnablePlatform)
		adminGroup.GET("/features/:group/:platform/disable", adminHandler.DisablePlatform)

		// stats
		adminGroup.GET("/stats", adminHandler.GetStats)

		// redis / upstash monitoring
		adminGroup.GET("/redis/stats", adminHandler.GetRedisStats)

		// auth
		adminGroup.POST("/auth/logout", middleware.RequireAdminSession(), adminHandler.Logout)
		adminGroup.GET("/auth/me", middleware.RequireAdminSession(), adminHandler.Me)
	}
}
