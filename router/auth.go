package router

import (
	"vidbot-api/config"
	"vidbot-api/internal/auth"
	"vidbot-api/middleware"

	"github.com/gin-gonic/gin"
)

func setupAuth(r *gin.Engine, cfg *config.Config) {
	authHandler := &auth.Handler{MagicString: cfg.MagicString}

	r.GET("/auth/verify", middleware.RequireAPIKey(), authHandler.Verify)
	r.GET("/auth/quota", middleware.RequireAPIKey(), authHandler.Quota)
}
