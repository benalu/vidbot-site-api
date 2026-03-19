package router

import (
	"vidbot-api/config"
	"vidbot-api/internal/services/iptv"
	"vidbot-api/middleware"

	"github.com/gin-gonic/gin"
)

func setupIPTV(r *gin.Engine, cfg *config.Config) {
	iptvHandler := iptv.NewHandler()

	group := r.Group("/iptv",
		middleware.RequireAPIKey(),
		middleware.RequireAccessToken(cfg.MagicString),
		middleware.RateLimit("iptv"),
	)
	{
		group.GET("/channels", iptvHandler.GetChannels)
		group.GET("/countries", iptvHandler.GetCountries)
		group.GET("/categories", iptvHandler.GetCategories)
	}
}
