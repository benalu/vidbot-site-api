package cache

import (
	"context"
	"sync"
	"time"
)

const featureTTL = 30 * time.Second

type featureEntry struct {
	value     string
	expiresAt time.Time
}

var (
	featureCache   = map[string]featureEntry{}
	featureCacheMu sync.RWMutex
)

// GetFeatureFlag — baca dari memory dulu, fallback ke Redis.
// Default "on" kalau Redis error supaya tidak block semua request.
func GetFeatureFlag(key string) string {
	featureCacheMu.RLock()
	if e, ok := featureCache[key]; ok && time.Now().Before(e.expiresAt) {
		featureCacheMu.RUnlock()
		return e.value
	}
	featureCacheMu.RUnlock()

	val, err := Get(context.Background(), key)
	if err != nil {
		// key tidak ada di Redis berarti default "on"
		return "on"
	}

	featureCacheMu.Lock()
	featureCache[key] = featureEntry{
		value:     val,
		expiresAt: time.Now().Add(featureTTL),
	}
	featureCacheMu.Unlock()

	return val
}

// InvalidateFeatureFlag — hapus dari memory saat admin toggle.
// Tidak perlu hapus dari Redis karena Redis adalah source of truth.
func InvalidateFeatureFlag(key string) {
	featureCacheMu.Lock()
	delete(featureCache, key)
	featureCacheMu.Unlock()
}
