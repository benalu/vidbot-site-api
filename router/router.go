package router

import (
	"log"
	"vidbot-api/config"
	contentprovider "vidbot-api/internal/services/content/provider"
	"vidbot-api/internal/services/content/provider/downr"
	"vidbot-api/internal/services/content/provider/vidown"
	convertprovider "vidbot-api/internal/services/convert/provider"
	ccprovider "vidbot-api/internal/services/convert/provider/cloudconvert"
	convertioprovider "vidbot-api/internal/services/convert/provider/convertio"
	"vidbot-api/internal/stream"
	"vidbot-api/pkg/iptvstore"
	"vidbot-api/pkg/proxy"

	"github.com/gin-gonic/gin"
)

func Setup(r *gin.Engine, cfg *config.Config) {
	// proxy clients
	proxyClient := proxy.NewClient(cfg.WorkerURLs, cfg.WorkerSecret)
	contentProxyClient := proxy.NewClient([]string{cfg.ContentWorkerURL}, cfg.ContentWorkerSecret)

	// IPTV store
	if err := iptvstore.Default.Init(); err != nil {
		log.Printf("[iptv] init error: %v", err)
	}

	// content providers
	contentProviders := buildContentProviders(contentProxyClient)

	// convert providers
	convertProviders := []convertprovider.Provider{
		ccprovider.New(cfg.CloudConvertAPIKey),
		convertioprovider.New(cfg.ConvertioAPIKey),
	}

	// stream handler (tidak butuh sub-router, hanya satu route)
	streamHandler := stream.NewHandler()
	r.GET("/dl", func(c *gin.Context) {
		streamHandler.Stream(c, cfg.StreamSecret, cfg.ToolsDir)
	})

	// sub-router per grup
	setupAuth(r, cfg)
	setupAdmin(r, cfg)
	setupIPTV(r, cfg)
	setupContent(r, cfg, contentProviders)
	setupVidhub(r, cfg, proxyClient)
	setupConvert(r, cfg, convertProviders)
}

// buildContentProviders menyusun provider per platform content.
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
			// vidown tidak support spotify
		},
	}
}
