package router

import (
	"vidbot-api/config"
	"vidbot-api/internal/admin"

	"github.com/gin-gonic/gin"
)

func setupAdmin(r *gin.Engine, cfg *config.Config) {
	adminHandler := admin.NewHandler(cfg.MasterKey)

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
	}
}
