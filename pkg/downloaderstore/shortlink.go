package downloaderstore

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
	dlSlPrefix    = "dl:sl:"
	dlSlIdxPrefix = "dl:sl:idx:"
	dlSlTTL       = 5 * 24 * time.Hour
	dlMemCacheTTL = 4 * 24 * time.Hour
)

type dlMemEntry struct {
	key       string
	expiresAt time.Time
}

var (
	dlMemCache   = map[string]dlMemEntry{}
	dlMemCacheMu sync.RWMutex
)

func init() {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			evictDLExpired()
		}
	}()
}

func evictDLExpired() {
	now := time.Now()
	dlMemCacheMu.Lock()
	defer dlMemCacheMu.Unlock()
	for url, entry := range dlMemCache {
		if now.After(entry.expiresAt) {
			delete(dlMemCache, url)
		}
	}
}

func setDLMemCache(rawURL, key string) {
	dlMemCacheMu.Lock()
	dlMemCache[rawURL] = dlMemEntry{key: key, expiresAt: time.Now().Add(dlMemCacheTTL)}
	dlMemCacheMu.Unlock()
}

// MaskURL menyimpan raw URL ke Redis dan mengembalikan short key.
// Idempoten — URL yang sama kembalikan key yang sama selama TTL belum habis.
func MaskURL(rawURL string) (string, error) {
	// L1: memory
	dlMemCacheMu.RLock()
	if entry, ok := dlMemCache[rawURL]; ok && time.Now().Before(entry.expiresAt) {
		dlMemCacheMu.RUnlock()
		return entry.key, nil
	}
	dlMemCacheMu.RUnlock()

	// L2: Redis
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	idxKey := dlSlIdxPrefix + hashDLKey(rawURL)
	if existing, err := cache.GetCache(ctx, idxKey); err == nil && existing != "" {
		_ = cache.ExpireCache(ctx, dlSlPrefix+existing, dlSlTTL)
		_ = cache.ExpireCache(ctx, idxKey, dlSlTTL)
		setDLMemCache(rawURL, existing)
		return existing, nil
	}

	// Generate baru
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("downloaderstore: mask url: %w", err)
	}
	key := hex.EncodeToString(raw)

	if err := cache.SetCache(ctx, dlSlPrefix+key, rawURL, dlSlTTL); err != nil {
		return "", fmt.Errorf("downloaderstore: redis set: %w", err)
	}
	_ = cache.SetCache(ctx, idxKey, key, dlSlTTL)
	setDLMemCache(rawURL, key)

	return key, nil
}

// MaskURLBatch memproses banyak URL sekaligus.
func MaskURLBatch(rawURLs []string) map[string]string {
	if len(rawURLs) == 0 {
		return map[string]string{}
	}

	out := make(map[string]string, len(rawURLs))
	var redisMiss []int

	now := time.Now()
	dlMemCacheMu.RLock()
	for i, u := range rawURLs {
		if entry, ok := dlMemCache[u]; ok && now.Before(entry.expiresAt) {
			out[u] = entry.key
		} else {
			redisMiss = append(redisMiss, i)
		}
	}
	dlMemCacheMu.RUnlock()

	if len(redisMiss) == 0 {
		return out
	}

	missURLs := make([]string, len(redisMiss))
	for i, idx := range redisMiss {
		missURLs[i] = rawURLs[idx]
	}
	idxKeys := make([]string, len(missURLs))
	for i, u := range missURLs {
		idxKeys[i] = dlSlIdxPrefix + hashDLKey(u)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var generateMiss []int
	existing, err := cache.MGetCache(ctx, idxKeys)
	if err == nil {
		for i, key := range existing {
			if key != "" {
				out[missURLs[i]] = key
				setDLMemCache(missURLs[i], key)
			} else {
				generateMiss = append(generateMiss, i)
			}
		}
	} else {
		for i := range missURLs {
			generateMiss = append(generateMiss, i)
		}
	}

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
	rawURL, err := cache.GetCache(ctx, dlSlPrefix+key)
	if err != nil {
		return "", fmt.Errorf("downloaderstore: link tidak ditemukan atau sudah kedaluwarsa")
	}
	return rawURL, nil
}

func hashDLKey(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
