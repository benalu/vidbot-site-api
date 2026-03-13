package limiter

import (
	"context"
	"fmt"
	"time"
	"vidbot-api/pkg/cache"
)

var endpointLimits = map[string]int{
	"content": 10,
	"convert": 20,
	"vidhub":  30,
}

func CheckRateLimit(keyHash, group string) (bool, error) {
	limit, ok := endpointLimits[group]
	if !ok {
		return true, nil
	}

	ctx := context.Background()
	redisKey := fmt.Sprintf("ratelimit:%s:%s", keyHash, group)

	count, err := cache.Incr(ctx, redisKey)
	if err != nil {
		// kalau Redis error, loloskan saja
		return true, nil
	}

	// set TTL hanya saat pertama kali (count == 1)
	if count == 1 {
		cache.Expire(ctx, redisKey, 60*time.Second)
	}

	return int(count) <= limit, nil
}
