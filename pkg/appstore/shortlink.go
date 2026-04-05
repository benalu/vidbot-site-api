package appstore

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
	"vidbot-api/pkg/cache"
)

const (
	appSlPrefix    = "app:sl:"
	appSlIdxPrefix = "app:sl:idx:"
	appSlTTL       = 720 * time.Hour
)

// MaskURL menyimpan raw URL ke Redis dan mengembalikan short key.
// Idempoten — URL yang sama kembalikan key yang sama selama TTL belum habis.
func MaskURL(rawURL string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	idxKey := appSlIdxPrefix + hashKey(rawURL)
	if existing, err := cache.Get(ctx, idxKey); err == nil && existing != "" {
		_ = cache.Expire(ctx, appSlPrefix+existing, appSlTTL)
		_ = cache.Expire(ctx, idxKey, appSlTTL)
		return existing, nil
	}

	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("appstore: mask url: %w", err)
	}
	key := hex.EncodeToString(raw)

	if err := cache.Set(ctx, appSlPrefix+key, rawURL, appSlTTL); err != nil {
		return "", fmt.Errorf("appstore: redis set: %w", err)
	}
	_ = cache.Set(ctx, idxKey, key, appSlTTL)

	return key, nil
}

// ResolveURL mengambil raw URL dari short key.
func ResolveURL(key string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	rawURL, err := cache.Get(ctx, appSlPrefix+key)
	if err != nil {
		return "", fmt.Errorf("appstore: link tidak ditemukan atau sudah kedaluwarsa")
	}
	return rawURL, nil
}

func hashKey(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
