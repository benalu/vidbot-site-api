package cache

import (
	"context"
	"sync"
	"time"
)

const apiKeyTTL = 60 * time.Second

type apiKeyEntry struct {
	raw       string // JSON string dari Redis
	expiresAt time.Time
}

var (
	apiKeyMem   = map[string]apiKeyEntry{}
	apiKeyMemMu sync.RWMutex
)

// GetAPIKey — baca dari memory dulu, fallback ke Redis.
// key adalah keyHash (bukan plain key).
func GetAPIKey(keyHash string) (string, error) {
	apiKeyMemMu.RLock()
	if e, ok := apiKeyMem[keyHash]; ok && time.Now().Before(e.expiresAt) {
		apiKeyMemMu.RUnlock()
		return e.raw, nil
	}
	apiKeyMemMu.RUnlock()

	// miss — hit Redis
	raw, err := Get(context.Background(), "apikeys:"+keyHash)
	if err != nil {
		return "", err
	}

	apiKeyMemMu.Lock()
	apiKeyMem[keyHash] = apiKeyEntry{
		raw:       raw,
		expiresAt: time.Now().Add(apiKeyTTL),
	}
	apiKeyMemMu.Unlock()

	return raw, nil
}

// InvalidateAPIKey — hapus dari memory saat admin revoke atau update key.
func InvalidateAPIKey(keyHash string) {
	apiKeyMemMu.Lock()
	delete(apiKeyMem, keyHash)
	apiKeyMemMu.Unlock()
}
