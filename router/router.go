package router

import (
	"vidbot-api/config"
	"vidbot-api/internal/admin"
	"vidbot-api/internal/auth"
	"vidbot-api/internal/services/content/instagram"
	contentprovider "vidbot-api/internal/services/content/provider"
	"vidbot-api/internal/services/content/provider/downr"
	"vidbot-api/internal/services/content/spotify"
	"vidbot-api/internal/services/content/tiktok"
	audioconvert "vidbot-api/internal/services/convert/audio"
	docconvert "vidbot-api/internal/services/convert/document"
	fontsconvert "vidbot-api/internal/services/convert/fonts"
	imgconvert "vidbot-api/internal/services/convert/image"
	convertprovider "vidbot-api/internal/services/convert/provider"
	ccprovider "vidbot-api/internal/services/convert/provider/cloudconvert"
	convertioprovider "vidbot-api/internal/services/convert/provider/convertio"
	"vidbot-api/internal/services/vidhub/vidarato"
	"vidbot-api/internal/services/vidhub/vidbos"
	"vidbot-api/internal/services/vidhub/videb"
	"vidbot-api/internal/services/vidhub/vidnest"
	"vidbot-api/internal/services/vidhub/vidoy"
	"vidbot-api/internal/stream"
	"vidbot-api/middleware"
	"vidbot-api/pkg/proxy"

	"github.com/gin-gonic/gin"
)

func Setup(r *gin.Engine, cfg *config.Config) {
	proxyClient := proxy.NewClient(cfg.WorkerURLs, cfg.WorkerSecret)
	contentProxyClient := proxy.NewClient([]string{cfg.ContentWorkerURL}, cfg.ContentWorkerSecret)

	providers := []contentprovider.Provider{
		downr.New(contentProxyClient),
	}

	convertProviders := []convertprovider.Provider{
		ccprovider.New(cfg.CloudConvertAPIKey),
		convertioprovider.New(cfg.ConvertioAPIKey),
	}

	streamHandler := stream.NewHandler()
	authHandler := &auth.Handler{MagicString: cfg.MagicString}
	adminHandler := admin.NewHandler(cfg.MasterKey)
	videbHandler := videb.NewHandler(proxyClient, cfg.DownloadWorkerURL, cfg.DownloadWorkerSecret, cfg.WorkerPayloadXORKey, cfg.AppURL, cfg.StreamSecret)
	vidoyHandler := vidoy.NewHandler(proxyClient, cfg.DownloadWorkerURL, cfg.DownloadWorkerSecret, cfg.WorkerPayloadXORKey, cfg.AppURL, cfg.StreamSecret)
	vidbosHandler := vidbos.NewHandler(cfg.DownloadWorkerURL, cfg.DownloadWorkerSecret, cfg.WorkerPayloadXORKey, cfg.AppURL, cfg.StreamSecret)
	vidaratoHandler := vidarato.NewHandler(proxyClient, cfg.DownloadWorkerURL, cfg.DownloadWorkerSecret, cfg.WorkerPayloadXORKey, cfg.AppURL, cfg.StreamSecret, cfg.ToolsDir)
	vidnestHandler := vidnest.NewHandler(proxyClient, cfg.DownloadWorkerURL, cfg.DownloadWorkerSecret, cfg.WorkerPayloadXORKey, cfg.AppURL, cfg.StreamSecret)
	spotifyHandler := spotify.NewHandler(providers, cfg.DownloadWorkerURL, cfg.DownloadWorkerSecret, cfg.WorkerPayloadXORKey, cfg.AppURL, cfg.StreamSecret)
	tiktokHandler := tiktok.NewHandler(providers, cfg.DownloadWorkerURL, cfg.DownloadWorkerSecret, cfg.WorkerPayloadXORKey, cfg.AppURL, cfg.StreamSecret)
	instagramHandler := instagram.NewHandler(providers, cfg.DownloadWorkerURL, cfg.DownloadWorkerSecret, cfg.WorkerPayloadXORKey, cfg.AppURL, cfg.StreamSecret)
	audioConvertHandler := audioconvert.NewHandler(
		convertProviders,
		cfg.DownloadWorkerURL,
		cfg.DownloadWorkerSecret,
		cfg.WorkerPayloadXORKey,
		cfg.AppURL,
		cfg.StreamSecret,
	)
	docConvertHandler := docconvert.NewHandler(
		convertProviders,
		cfg.DownloadWorkerURL,
		cfg.DownloadWorkerSecret,
		cfg.WorkerPayloadXORKey,
		cfg.AppURL,
		cfg.StreamSecret,
	)
	imgConvertHandler := imgconvert.NewHandler(
		convertProviders,
		cfg.DownloadWorkerURL,
		cfg.DownloadWorkerSecret,
		cfg.WorkerPayloadXORKey,
		cfg.AppURL,
		cfg.StreamSecret,
	)
	fontsConvertHandler := fontsconvert.NewHandler(
		convertProviders,
		cfg.DownloadWorkerURL,
		cfg.DownloadWorkerSecret,
		cfg.WorkerPayloadXORKey,
		cfg.AppURL,
		cfg.StreamSecret,
	)

	r.GET("/auth/verify", middleware.RequireAPIKey(), authHandler.Verify)
	r.GET("/auth/quota", middleware.RequireAPIKey(), authHandler.Quota)
	r.GET("/dl", func(c *gin.Context) {
		streamHandler.Stream(c, cfg.StreamSecret, cfg.ToolsDir)
	})

	adminGroup := r.Group("/admin")
	{
		adminGroup.POST("/keys", adminHandler.CreateKey)
		adminGroup.DELETE("/keys/:key", adminHandler.RevokeKey)
		adminGroup.GET("/keys", adminHandler.ListKeys)
		adminGroup.POST("/keys/:key/topup", adminHandler.TopUpQuota)
	}

	vidhub := r.Group("/vidhub",
		middleware.RequireAPIKey(),
		middleware.RequireAccessToken(cfg.MagicString),
		middleware.RateLimit("vidhub"),
	)
	{
		vidhub.POST("/videb", videbHandler.Extract)
		vidhub.POST("/vidoy", vidoyHandler.Extract)
		vidhub.POST("/vidbos", vidbosHandler.Extract)
		vidhub.POST("/vidarato", vidaratoHandler.Extract)
		vidhub.POST("/vidnest", vidnestHandler.Extract)
	}

	content := r.Group("/content",
		middleware.RequireAPIKey(),
		middleware.RequireAccessToken(cfg.MagicString),
		middleware.RateLimit("content"),
	)
	{
		content.POST("/spotify", spotifyHandler.Extract)
		content.POST("/tiktok", tiktokHandler.Extract)
		content.POST("/instagram", instagramHandler.Extract)
	}

	convert := r.Group("/convert",
		middleware.RequireAPIKey(),
		middleware.RequireAccessToken(cfg.MagicString),
		middleware.RateLimit("convert"),
	)
	{
		convert.POST("/audio", audioConvertHandler.Convert)
		convert.POST("/audio/upload", audioConvertHandler.Upload)
		convert.POST("/document", docConvertHandler.Convert)
		convert.POST("/document/upload", docConvertHandler.Upload)
		convert.POST("/image", imgConvertHandler.Convert)
		convert.POST("/image/upload", imgConvertHandler.Upload)
		convert.POST("/fonts", fontsConvertHandler.Convert)
		convert.POST("/fonts/upload", fontsConvertHandler.Upload)
		convert.GET("/status/:job_id", audioConvertHandler.Status)
	}
}
