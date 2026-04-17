package cache

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type ProviderCache struct {
	mu       sync.RWMutex
	data     map[string][]string
	interval time.Duration
}

var providerCache = &ProviderCache{
	data:     make(map[string][]string),
	interval: 5 * time.Minute,
}

func InitProviderCache(keys []string) {
	providerCache.load(keys)
	go providerCache.startRefresh(keys)
}

func GetProviderOrder(key string) []string {
	providerCache.mu.RLock()
	defer providerCache.mu.RUnlock()
	return providerCache.data[key]
}

func (pc *ProviderCache) load(keys []string) {
	ctx := context.Background()
	newData := make(map[string][]string)

	for _, key := range keys {
		names, err := LRange(ctx, key)
		if err != nil || len(names) == 0 {
			continue
		}
		newData[key] = names
	}

	pc.mu.Lock()
	pc.data = newData
	pc.mu.Unlock()

	slog.Debug("provider cache loaded", "keys", len(newData))
}

func (pc *ProviderCache) startRefresh(keys []string) {
	ticker := time.NewTicker(pc.interval)
	defer ticker.Stop()
	for range ticker.C {
		pc.load(keys)
	}
}
