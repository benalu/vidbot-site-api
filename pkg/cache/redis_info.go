package cache

import (
	"context"
	"time"
)

// HasSeparateCache — true kalau ada cache Redis terpisah
func HasSeparateCache() bool {
	return cacheClient != nil && cacheClient != client
}

// Info — ambil Redis INFO dari main client
func Info(ctx context.Context) (string, error) {
	return client.Info(ctx).Result()
}

// InfoCache — ambil Redis INFO dari cache client
func InfoCache(ctx context.Context) (string, error) {
	c := cacheClient
	if c == nil {
		c = client
	}
	return c.Info(ctx).Result()
}

// PingCache — ping cache client
func PingCache(ctx context.Context) error {
	c := cacheClient
	if c == nil {
		c = client
	}
	return c.Ping(ctx).Err()
}

// CountKeys — hitung jumlah key berdasarkan pattern (pakai SCAN bukan KEYS)
// Aman untuk production — tidak block Redis
func CountKeys(ctx context.Context, pattern string) int64 {
	var count int64
	var cursor uint64
	timeout := time.After(3 * time.Second)

	for {
		select {
		case <-timeout:
			return count
		default:
		}

		keys, nextCursor, err := client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return count
		}
		count += int64(len(keys))
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return count
}
