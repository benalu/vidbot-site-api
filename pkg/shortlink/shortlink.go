package shortlink

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
	"vidbot-api/pkg/cache"
	"vidbot-api/pkg/downloader"
)

const (
	keyPrefix   = "sl:"
	indexPrefix = "sl:idx:"
	keyLen      = 8
	defaultTTL  = 2 * time.Hour
)

var ttlByService = map[string]time.Duration{
	"content": 2 * time.Hour,
	"vidhub":  2 * time.Hour,
	"convert": 30 * time.Minute,
}

func getTTL(service string) time.Duration {
	if ttl, ok := ttlByService[service]; ok {
		return ttl
	}
	return defaultTTL
}

// Create menyimpan payload ke Redis dan mengembalikan short key.
// cacheKey adalah key cache metadata (misal: "vidhub:vidnest:abc123").
// Jika sudah ada shortlink untuk cacheKey yang sama, kembalikan key lama (idempoten).
// BARU
func Create(payload downloader.Payload, cacheKey string) (string, error) {
	ctx := context.Background()
	ttl := getTTL(payload.Service)

	// cek index dulu — idempoten berdasarkan cacheKey metadata
	idxKey := indexPrefix + cacheKey
	if existing, err := cache.Get(ctx, idxKey); err == nil && existing != "" {
		_ = cache.Expire(ctx, keyPrefix+existing, ttl)
		_ = cache.Expire(ctx, idxKey, ttl)
		return existing, nil
	}

	// belum ada — generate key baru
	raw := make([]byte, keyLen)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("shortlink: generate key: %w", err)
	}
	key := hex.EncodeToString(raw)

	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("shortlink: marshal: %w", err)
	}
	if err := cache.Set(ctx, keyPrefix+key, string(data), ttl); err != nil {
		return "", fmt.Errorf("shortlink: redis set: %w", err)
	}

	// simpan index: cacheKey → short key
	if err := cache.Set(ctx, idxKey, key, ttl); err != nil {
		fmt.Printf("shortlink: warn: index set failed: %v\n", err)
	}

	return key, nil
}

// Resolve mengambil payload dari Redis berdasarkan short key
func Resolve(key string) (*downloader.Payload, error) {
	ctx := context.Background()
	raw, err := cache.Get(ctx, keyPrefix+key)
	if err != nil {
		return nil, fmt.Errorf("shortlink: not found or expired")
	}

	var p downloader.Payload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil, fmt.Errorf("shortlink: unmarshal: %w", err)
	}
	return &p, nil
}
