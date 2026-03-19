package router

import (
	"vidbot-api/config"
	"vidbot-api/internal/admin"
	"vidbot-api/internal/auth"
	"vidbot-api/middleware"

	"github.com/gin-gonic/gin"
)

func setupAuth(r *gin.Engine, cfg *config.Config) {
	authHandler := &auth.Handler{MagicString: cfg.MagicString}

	r.GET("/auth/verify", middleware.RequireAPIKey(), authHandler.Verify)
	r.GET("/auth/quota", middleware.RequireAPIKey(), authHandler.Quota)
}

func setupAdmin(r *gin.Engine, cfg *config.Config) {
	adminHandler := admin.NewHandler(cfg.MasterKey)

	adminGroup := r.Group("/admin")
	{
		adminGroup.POST("/keys", adminHandler.CreateKey)
		adminGroup.DELETE("/keys/:key", adminHandler.RevokeKey)
		adminGroup.GET("/keys", adminHandler.ListKeys)
		adminGroup.POST("/keys/:key/topup", adminHandler.TopUpQuota)
	}
}
