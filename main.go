package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"vidbot-api/config"
	"vidbot-api/internal/stream"
	"vidbot-api/middleware"
	"vidbot-api/pkg/appstore"
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
	if cfg.CacheRedisURL != "" {
		cache.InitCache(cfg.CacheRedisURL)
	}

	dataDir := cfg.DataDir
	if dataDir == "" {
		dataDir = "data"
	}
	if err := leakcheck.Default.Init(filepath.Join(dataDir, "leakcheck")); err != nil {
		log.Printf("[leakcheck] init error: %v", err)
	}
	if err := appstore.Init(filepath.Join(dataDir, "app")); err != nil {
		log.Printf("[appstore] init error: %v", err)
	}
	if err := stats.Init("data/stats/stats.db"); err != nil {
		log.Printf("[stats] init error: %v", err)
	}

	// background goroutine: WAL checkpoint + stats log
	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()
	leakcheck.Default.StartBackground(bgCtx)

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(middleware.RequestID())
	r.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		if strings.HasPrefix(c.Request.URL.Path, "/admin") {
			// Admin: restrict origin + expose header internal
			allowed := false
			if cfg.AllowedOrigins == "" {
				allowed = origin == ""
			} else {
				for _, o := range strings.Split(cfg.AllowedOrigins, ",") {
					if strings.TrimSpace(o) == origin {
						allowed = true
						break
					}
				}
			}
			if !allowed && origin != "" {
				c.AbortWithStatus(403)
				return
			}
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Headers", "Content-Type, X-Master-Key, X-Admin-Session")
			c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		} else {
			// Public API: allow semua origin, hanya expose header publik
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Headers", "Content-Type, X-API-Key, X-Access-Token")
			c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})
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

	bgCancel() // hentikan background goroutine

	// timeout diperpanjang: reload bisa makan waktu lama
	// kalau tidak ada reload aktif, shutdown selesai dalam detik
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	stream.CancelAllSessions()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
