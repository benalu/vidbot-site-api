package appstore

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
	"vidbot-api/pkg/cache"
)

const (
	appSlPrefix    = "app:sl:"
	appSlIdxPrefix = "app:sl:idx:"
	appSlTTL       = 24 * time.Hour
	memCacheTTL    = 20 * time.Hour
)

type memEntry struct {
	key       string
	expiresAt time.Time
}

var (
	memCache   = map[string]memEntry{}
	memCacheMu sync.RWMutex
)

func init() {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			evictExpired()
		}
	}()
}

func evictExpired() {
	now := time.Now()
	memCacheMu.Lock()
	defer memCacheMu.Unlock()
	for url, entry := range memCache {
		if now.After(entry.expiresAt) {
			delete(memCache, url)
		}
	}
}

func setMemCache(rawURL, key string) {
	memCacheMu.Lock()
	memCache[rawURL] = memEntry{key: key, expiresAt: time.Now().Add(memCacheTTL)}
	memCacheMu.Unlock()
}

// MaskURL menyimpan raw URL ke Redis dan mengembalikan short key.
// Idempoten — URL yang sama kembalikan key yang sama selama TTL belum habis.
func MaskURL(rawURL string) (string, error) {
	// L1: memory
	memCacheMu.RLock()
	if entry, ok := memCache[rawURL]; ok && time.Now().Before(entry.expiresAt) {
		memCacheMu.RUnlock()
		return entry.key, nil
	}
	memCacheMu.RUnlock()

	// L2: Redis
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	idxKey := appSlIdxPrefix + hashKey(rawURL)
	if existing, err := cache.GetCache(ctx, idxKey); err == nil && existing != "" {
		_ = cache.ExpireCache(ctx, appSlPrefix+existing, appSlTTL)
		_ = cache.ExpireCache(ctx, idxKey, appSlTTL)
		setMemCache(rawURL, existing)
		return existing, nil
	}

	// Generate baru
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("appstore: mask url: %w", err)
	}
	key := hex.EncodeToString(raw)

	if err := cache.SetCache(ctx, appSlPrefix+key, rawURL, appSlTTL); err != nil {
		return "", fmt.Errorf("appstore: redis set: %w", err)
	}
	_ = cache.SetCache(ctx, idxKey, key, appSlTTL)
	setMemCache(rawURL, key)

	return key, nil
}

// MaskURLBatch memproses banyak URL sekaligus — L1 memory → L2 Redis → generate.
func MaskURLBatch(rawURLs []string) map[string]string {
	if len(rawURLs) == 0 {
		return map[string]string{}
	}

	out := make(map[string]string, len(rawURLs))
	var redisMiss []int

	// L1: memory
	now := time.Now()
	memCacheMu.RLock()
	for i, u := range rawURLs {
		if entry, ok := memCache[u]; ok && now.Before(entry.expiresAt) {
			out[u] = entry.key
		} else {
			redisMiss = append(redisMiss, i)
		}
	}
	memCacheMu.RUnlock()

	if len(redisMiss) == 0 {
		return out
	}

	// L2: Redis MGET untuk yang miss di memory
	missURLs := make([]string, len(redisMiss))
	for i, idx := range redisMiss {
		missURLs[i] = rawURLs[idx]
	}
	idxKeys := make([]string, len(missURLs))
	for i, u := range missURLs {
		idxKeys[i] = appSlIdxPrefix + hashKey(u)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var generateMiss []int
	existing, err := cache.MGetCache(ctx, idxKeys)
	if err == nil {
		for i, key := range existing {
			if key != "" {
				out[missURLs[i]] = key
				setMemCache(missURLs[i], key)
			} else {
				generateMiss = append(generateMiss, i)
			}
		}
	} else {
		for i := range missURLs {
			generateMiss = append(generateMiss, i)
		}
	}

	// Generate baru untuk yang benar-benar belum ada
	for _, i := range generateMiss {
		if key, err := MaskURL(missURLs[i]); err == nil {
			out[missURLs[i]] = key
		}
	}

	return out
}

// ResolveURL mengambil raw URL dari short key.
func ResolveURL(key string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	rawURL, err := cache.GetCache(ctx, appSlPrefix+key)
	if err != nil {
		return "", fmt.Errorf("appstore: link tidak ditemukan atau sudah kedaluwarsa")
	}
	return rawURL, nil
}

func hashKey(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
