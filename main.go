package main

import (
	"log"
	"vidbot-api/config"
	"vidbot-api/pkg/cache"
	"vidbot-api/pkg/downloader"
	"vidbot-api/router"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	downloader.InitKeys(cfg.PayloadEncryptKey, cfg.PayloadHMACKey)

	cache.Init(cfg.RedisURL)

	r := gin.Default()
	router.Setup(r, cfg)

	log.Printf("Server running on port %s", cfg.AppPort)
	r.Run(":" + cfg.AppPort)
}
