package main

import (
	"context"
	"log/slog"
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
	"vidbot-api/pkg/downloaderstore"
	"vidbot-api/pkg/keyvault"
	"vidbot-api/pkg/leakcheck"
	"vidbot-api/pkg/logger"
	"vidbot-api/pkg/stats"
	"vidbot-api/router"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()

	// ── Logger init ───────────────────────────────────────────────────────────
	// LOG_FORMAT=json untuk production (default), "text" untuk dev
	// LOG_LEVEL=debug/info/warn/error (default: info)
	logFormat := getEnv("LOG_FORMAT", "json")
	logLevel := parseLogLevel(getEnv("LOG_LEVEL", "info"))
	logger.Init(logFormat, logLevel)

	slog.Info("vidbot-api starting",
		"log_format", logFormat,
		"log_level", logLevel.String(),
	)

	// ── Core init ─────────────────────────────────────────────────────────────
	downloader.InitKeys(cfg.PayloadEncryptKey, cfg.PayloadHMACKey)

	cache.Init(cfg.RedisURL)
	if cfg.CacheRedisURL != "" {
		cache.InitCache(cfg.CacheRedisURL)
		slog.Info("cache redis initialized", "separate", true)
	}

	keyvault.Init(cfg.KeyVaultSecret)
	if keyvault.IsReady() {
		slog.Info("key vault initialized — plain keys will be stored encrypted")
	} else {
		slog.Warn("key vault not configured (KEY_VAULT_SECRET empty) — RevealKey will not work")
	}

	dataDir := cfg.DataDir
	if dataDir == "" {
		dataDir = "data"
	}

	os.Setenv("LEAKCHECK_DB_DSN", cfg.LeakcheckDSN)
	if err := leakcheck.Default.Init(filepath.Join(dataDir, "leakcheck")); err != nil {
		slog.Warn("leakcheck init failed", "error", err)
	}
	if err := appstore.Init(filepath.Join(dataDir, "app")); err != nil {
		slog.Warn("appstore init failed", "error", err)
	}
	if err := downloaderstore.Init(filepath.Join(dataDir, "downloader")); err != nil {
		slog.Warn("downloaderstore init failed", "error", err)
	}
	if err := stats.Init(cfg.StatsDSN); err != nil {
		slog.Warn("stats db init failed", "error", err)
	}

	// ── Background tasks ──────────────────────────────────────────────────────
	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()
	leakcheck.Default.StartBackground(bgCtx)

	// ── Gin setup ─────────────────────────────────────────────────────────────
	gin.SetMode(gin.ReleaseMode)

	// Pakai gin.New() bukan gin.Default() — logger di-handle sendiri via middleware
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.RequestLogger()) // structured request log

	r.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		if strings.HasPrefix(c.Request.URL.Path, "/admin") {
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

	// ── Server start ──────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:    ":" + cfg.AppPort,
		Handler: r,
	}

	go func() {
		slog.Info("server listening", "port", cfg.AppPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	slog.Info("shutdown signal received", "signal", sig.String())

	bgCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	stream.CancelAllSessions()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped gracefully")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
