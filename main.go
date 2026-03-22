package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"vidbot-api/config"
	"vidbot-api/pkg/cache"
	"vidbot-api/pkg/downloader"
	"vidbot-api/pkg/leakcheck"
	"vidbot-api/pkg/stats"
	"vidbot-api/router"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	downloader.InitKeys(cfg.PayloadEncryptKey, cfg.PayloadHMACKey)

	cache.Init(cfg.RedisURL)

	leakcheckDir := cfg.LeakcheckDir
	if leakcheckDir == "" {
		leakcheckDir = "data/leakcheck"
	}
	if err := leakcheck.Default.Init(leakcheckDir); err != nil {
		log.Printf("[leakcheck] init error: %v", err)
	}

	if err := stats.Init("data/stats/stats.db"); err != nil {
		log.Printf("[stats] init error: %v", err)
	}

	r := gin.Default()
	router.Setup(r, cfg)

	srv := &http.Server{
		Addr:    ":" + cfg.AppPort,
		Handler: r,
	}

	go func() {
		log.Printf("Server running on port %s", cfg.AppPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
