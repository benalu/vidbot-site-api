package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort              string
	APIKey               string
	MagicString          string
	RedisURL             string
	CacheRedisURL        string
	WorkerURLs           []string
	WorkerSecret         string
	DownloadWorkerURL    string
	DownloadWorkerSecret string
	AppURL               string
	StreamSecret         string
	ContentWorkerURL     string
	ContentWorkerSecret  string
	PayloadEncryptKey    string
	PayloadHMACKey       string
	WorkerPayloadXORKey  string
	ToolsDir             string
	MasterKey            string
	CloudConvertAPIKey   string
	ConvertioAPIKey      string
	DataDir              string
	AllowedOrigins       string
	StatsDSN             string
	LeakcheckDSN         string
	// CDN stor.co.id
	CDNAPIKey   string
	CDNFolderID string // folder ID Android/Windows
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	workerURLs := []string{}
	if raw := os.Getenv("WORKER_URLS"); raw != "" {
		for _, u := range strings.Split(raw, ",") {
			u = strings.TrimSpace(u)
			if u != "" {
				workerURLs = append(workerURLs, u)
			}
		}
	}

	return &Config{
		AppPort:              os.Getenv("APP_PORT"),
		APIKey:               os.Getenv("API_KEY"),
		MagicString:          os.Getenv("MAGIC_STRING"),
		RedisURL:             os.Getenv("REDIS_URL"),
		CacheRedisURL:        os.Getenv("CACHE_REDIS_URL"),
		WorkerURLs:           workerURLs,
		WorkerSecret:         os.Getenv("WORKER_SECRET"),
		DownloadWorkerURL:    os.Getenv("DOWNLOAD_WORKER_URL"),
		DownloadWorkerSecret: os.Getenv("DOWNLOAD_WORKER_SECRET"),
		AppURL:               os.Getenv("APP_URL"),
		StreamSecret:         os.Getenv("STREAM_SECRET"),
		ContentWorkerURL:     os.Getenv("CONTENT_WORKER_URL"),
		ContentWorkerSecret:  os.Getenv("CONTENT_WORKER_SECRET"),
		PayloadEncryptKey:    os.Getenv("PAYLOAD_ENCRYPT_KEY"),
		PayloadHMACKey:       os.Getenv("PAYLOAD_HMAC_KEY"),
		WorkerPayloadXORKey:  os.Getenv("WORKER_PAYLOAD_KEY"),
		ToolsDir:             os.Getenv("TOOLS_DIR"),
		MasterKey:            os.Getenv("MASTER_KEY"),
		CloudConvertAPIKey:   os.Getenv("CLOUDCONVERT_API_KEY"),
		ConvertioAPIKey:      os.Getenv("CONVERTIO_API_KEY"),
		DataDir:              getEnvDefault("DATA_DIR", "./data"),
		CDNAPIKey:            os.Getenv("CDN_API_KEY"),
		CDNFolderID:          os.Getenv("CDN_FOLDER_ID"),
		AllowedOrigins:       os.Getenv("ALLOWED_ORIGINS"),
		StatsDSN:             os.Getenv("STATS_DB_DSN"),
		LeakcheckDSN:         os.Getenv("LEAKCHECK_DB_DSN"),
	}
}

func getEnvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
