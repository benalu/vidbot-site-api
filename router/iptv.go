package router

import (
	"vidbot-api/config"
	"vidbot-api/internal/services/iptv"
	"vidbot-api/middleware"

	"github.com/gin-gonic/gin"
)

func setupIPTV(r *gin.Engine, cfg *config.Config) {
	iptvHandler := iptv.NewHandler()

	// route existing — butuh API Key + Access Token
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

	// route playlist — API key via query param, langsung bisa di VLC/Tivimate
	r.GET("/iptv/playlist", middleware.RequireAPIKeyFromQuery(), middleware.RateLimit("iptv"), iptvHandler.GetPlaylist)

}
