package router

import (
	"context"
	"log/slog"
	"time"
	"vidbot-api/config"
	contentprovider "vidbot-api/internal/services/content/provider"
	"vidbot-api/internal/services/content/provider/downr"
	"vidbot-api/internal/services/content/provider/vidown"
	convertprovider "vidbot-api/internal/services/convert/provider"
	ccprovider "vidbot-api/internal/services/convert/provider/cloudconvert"
	convertioprovider "vidbot-api/internal/services/convert/provider/convertio"

	"vidbot-api/internal/stream"
	"vidbot-api/pkg/cache"
	"vidbot-api/pkg/downloader"
	"vidbot-api/pkg/iptvstore"
	"vidbot-api/pkg/proxy"
	"vidbot-api/pkg/shortlink"

	"github.com/gin-gonic/gin"
)

func Setup(r *gin.Engine, cfg *config.Config) {
	proxyClient := proxy.NewClient(cfg.WorkerURLs, cfg.WorkerSecret)
	contentProxyClient := proxy.NewClient([]string{cfg.ContentWorkerURL}, cfg.ContentWorkerSecret)

	cache.InitProviderCache([]string{
		"content:provider:spotify",
		"content:provider:tiktok",
		"content:provider:instagram",
		"content:provider:twitter",
		"content:provider:threads",
		"convert:provider:audio",
		"convert:provider:document",
		"convert:provider:image",
		"convert:provider:fonts",
	})

	// wire shortlink ke downloader (hindari circular import)
	downloader.SetShortlinkCreator(func(payload downloader.Payload, cacheKey string) (string, error) {
		return shortlink.Create(payload, cacheKey)
	})
	downloader.SetShortlinkResolver(func(key string) (*downloader.Payload, error) {
		return shortlink.Resolve(key)
	})

	// IPTV store
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		done := make(chan error, 1)
		go func() { done <- iptvstore.Default.Init() }()

		select {
		case err := <-done:
			if err != nil {
				slog.Warn("iptv store init failed, all /iptv/* endpoints will return empty data", "error", err)
			} else {
				slog.Info("iptv store initialized")
			}
		case <-ctx.Done():
			slog.Warn("iptv store init timeout after 30s, will retry on next auto-refresh cycle")
		}
	}()

	// content providers
	contentProviders := buildContentProviders(contentProxyClient)

	// convert providers
	convertProviders := []convertprovider.Provider{
		ccprovider.New(cfg.CloudConvertAPIKey),
		convertioprovider.New(cfg.ConvertioAPIKey),
	}

	// stream handler
	streamHandler := stream.NewHandler()
	r.GET("/dl", func(c *gin.Context) {
		streamHandler.Stream(c, cfg.StreamSecret, cfg.ToolsDir)
	})

	healthHandler := setupHealth(r, cfg)
	setupAuth(r, cfg)
	setupAdmin(r, cfg, healthHandler)
	setupIPTV(r, cfg)
	setupContent(r, cfg, contentProviders)
	setupVidhub(r, cfg, proxyClient)
	setupConvert(r, cfg, convertProviders)
	setupLeakcheck(r, cfg)
	setupApp(r, cfg)
	setupDownloader(r, cfg)
	setupMovies(r, cfg)
	setupEbooks(r, cfg)
}

type contentProviderSet struct {
	tiktok    []contentprovider.Provider
	instagram []contentprovider.Provider
	twitter   []contentprovider.Provider
	threads   []contentprovider.Provider
	spotify   []contentprovider.Provider
}

func buildContentProviders(client *proxy.Client) contentProviderSet {
	return contentProviderSet{
		tiktok: []contentprovider.Provider{
			downr.New(client),
			vidown.New(client),
		},
		instagram: []contentprovider.Provider{
			downr.New(client),
			vidown.NewForPlatform(client, "instagram"),
		},
		twitter: []contentprovider.Provider{
			downr.New(client),
			vidown.NewForPlatform(client, "twitter"),
		},
		threads: []contentprovider.Provider{
			downr.New(client),
			vidown.NewForPlatform(client, "threads"),
		},
		spotify: []contentprovider.Provider{
			downr.New(client),
		},
	}
}
