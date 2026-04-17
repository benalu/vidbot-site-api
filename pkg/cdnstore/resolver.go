package cdnstore

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"
	"vidbot-api/pkg/cache"
)

const (
	// signed URL di-cache 6 hari (url CDN expire 7 hari, margin 1 hari)
	signedURLTTL = 6 * 24 * time.Hour
	// expiresIn yang di-request ke CDN: 168 jam = 7 hari
	expiresInHours = 168
)

// CachedFile — yang disimpan di Redis
type CachedFile struct {
	FileID       string `json:"file_id"`
	OriginalName string `json:"original_name"`
	Size         int64  `json:"size"`
	SignedURL    string `json:"signed_url"`
	Variant      string `json:"variant"` // misal: Universal, arm64-v8a, x86_64
	Version      string `json:"version"` // misal: 4.3.1 (diekstrak dari filename)
}

// cacheKey Redis untuk signed URL per app+platform+version
// format: cdn:app:{platform}:{appSlug}:{version}
func cdnCacheKey(platform, appSlug, version string) string {
	if version == "" {
		version = "all"
	}
	return fmt.Sprintf("cdn:app:%s:%s:%s", platform, appSlug, version)
}

// Resolver menggabungkan CDN Client + Redis cache
type Resolver struct {
	client *Client
}

func NewResolver(client *Client) *Resolver {
	return &Resolver{client: client}
}

// GetOrFetchFiles mengambil signed URLs dari cache atau CDN.
// appName digunakan sebagai keyword pencarian ke CDN.
// version digunakan untuk filter (kosong = ambil semua versi yang match).
func (r *Resolver) GetOrFetchFiles(ctx context.Context, platform, appSlug, appName, version string) ([]CachedFile, error) {
	cacheKey := cdnCacheKey(platform, appSlug, version)

	// coba dari Redis dulu
	if files, err := r.loadFromCache(ctx, cacheKey); err == nil && len(files) > 0 {
		return files, nil
	}

	// miss — fetch dari CDN
	files, err := r.fetchAndCache(ctx, platform, appSlug, appName, version, cacheKey)
	if err != nil {
		return nil, err
	}
	return files, nil
}

func (r *Resolver) loadFromCache(ctx context.Context, cacheKey string) ([]CachedFile, error) {
	raw, err := cache.GetCache(ctx, cacheKey)
	if err != nil {
		return nil, err
	}
	var files []CachedFile
	if err := json.Unmarshal([]byte(raw), &files); err != nil {
		return nil, err
	}
	return files, nil
}

func (r *Resolver) fetchAndCache(ctx context.Context, platform, appSlug, appName, version, cacheKey string) ([]CachedFile, error) {
	// search files di CDN
	searchResult, err := r.client.SearchFiles(ctx, appName, 1, 50)
	if err != nil {
		return nil, fmt.Errorf("cdn fetch: %w", err)
	}
	if len(searchResult.Files) == 0 {
		return nil, fmt.Errorf("cdn: no files found for '%s'", appName)
	}

	// filter yang relevan
	relevant := filterRelevantFiles(searchResult.Files, appName, version)
	if len(relevant) == 0 {
		return nil, fmt.Errorf("cdn: no matching files for '%s' v%s", appName, version)
	}

	// untuk tiap file, ambil signed URL
	var result []CachedFile
	for _, f := range relevant {
		dlResp, err := r.client.GetDownloadURL(ctx, f.ID, expiresInHours)
		if err != nil {
			slog.Warn("cdnstore skip file", "file_id", f.ID, "name", f.OriginalName, "error", err)
			continue
		}
		result = append(result, CachedFile{
			FileID:       f.ID,
			OriginalName: f.OriginalName,
			Size:         f.Size,
			SignedURL:    dlResp.URL,
			Variant:      extractVariant(f.OriginalName),
			Version:      extractVersion(f.OriginalName),
		})
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("cdn: failed to get download URLs")
	}

	// simpan ke Redis
	data, _ := json.Marshal(result)
	if err := cache.SetCache(ctx, cacheKey, string(data), signedURLTTL); err != nil {
		slog.Warn("cdnstore cache set failed", "error", err)
	}

	return result, nil
}

// filterRelevantFiles — filter berdasarkan nama app dan versi
func filterRelevantFiles(files []CDNFile, appName, version string) []CDNFile {
	var result []CDNFile
	for _, f := range files {
		nameLower := strings.ToLower(f.OriginalName)
		slugLower := strings.ToLower(strings.ReplaceAll(appName, " ", "-"))

		// nama file harus diawali slug app
		if !strings.HasPrefix(nameLower, slugLower+"_") {
			continue
		}
		// filter versi kalau di-specify
		if version != "" {
			cleanVer := strings.TrimPrefix(version, "v")
			if !strings.Contains(nameLower, cleanVer) {
				continue
			}
		}
		result = append(result, f)
	}
	return result
}

// extractVariant — ambil variant dari nama file
// contoh: "SpotiFLAC+v4.3.1+Universal.apk" → "Universal"
//
//	"SpotiFLAC+v4.3.1+arm64-v8a.apk" → "arm64-v8a"
func extractVariant(originalName string) string {
	name := strings.TrimSuffix(originalName, filepath.Ext(originalName))
	parts := strings.SplitN(name, "_", 3)
	if len(parts) >= 3 {
		return parts[2]
	}
	return "default"
}

// extractVersion — ambil versi dari nama file
// contoh: "SpotiFLAC+v4.3.1+Universal.apk" → "4.3.1"
func extractVersion(originalName string) string {
	name := strings.TrimSuffix(originalName, filepath.Ext(originalName))
	parts := strings.SplitN(name, "_", 3)
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

// InvalidateCache — hapus cache untuk app tertentu (misal setelah update)
func (r *Resolver) InvalidateCache(ctx context.Context, platform, appSlug, version string) error {
	return cache.DelCache(ctx, cdnCacheKey(platform, appSlug, version))
}
