package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
	"vidbot-api/pkg/cache"

	"github.com/redis/go-redis/v9"
)

// TTL per service:situs
var cacheTTL = map[string]time.Duration{
	"vidhub:videb":      2 * time.Hour,
	"vidhub:vidoy":      1 * time.Hour,
	"vidhub:vidbos":     2 * time.Hour,
	"vidhub:vidarato":   2 * time.Hour,
	"vidhub:vidnest":    2 * time.Hour,
	"content:spotify":   30 * 24 * time.Hour,
	"content:tiktok":    2 * time.Hour,
	"content:instagram": 30 * time.Minute,
	"content:twitter":   2 * time.Hour,
	"content:threads":   30 * time.Minute,
}

func defaultTTL() time.Duration {
	return 15 * time.Minute
}

func CacheKey(service, site, rawURL string) string {
	return fmt.Sprintf("%s:%s:%x", service, site, hashURL(rawURL))
}

func CacheGet[T any](service, site, rawURL string) (*T, error) {
	ctx := context.Background()
	key := CacheKey(service, site, rawURL)

	val, err := cache.Get(ctx, key)
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var result T
	if err := json.Unmarshal([]byte(val), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func CacheSet[T any](service, site, rawURL string, data *T) error {
	ctx := context.Background()
	key := CacheKey(service, site, rawURL)

	ttlKey := service + ":" + site
	ttl, ok := cacheTTL[ttlKey]
	if !ok {
		ttl = defaultTTL()
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return cache.Set(ctx, key, string(bytes), ttl)
}

func hashURL(rawURL string) [32]byte {
	return sha256.Sum256([]byte(rawURL))
}
