package cache

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

var client *redis.Client
var cacheClient *redis.Client // redis response cache

func InitCache(redisURL string) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("Failed to parse Cache Redis URL: %v", err)
	}
	cacheClient = redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := cacheClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Cache Redis: %v", err)
	}
	log.Println("Cache Redis connected")
}

// GetCache, SetCache, DelCache — pakai cacheClient, fallback ke client kalau cacheClient nil
func GetCache(ctx context.Context, key string) (string, error) {
	c := cacheClient
	if c == nil {
		c = client
	}
	return c.Get(ctx, key).Result()
}

func SetCache(ctx context.Context, key string, value string, ttl time.Duration) error {
	c := cacheClient
	if c == nil {
		c = client
	}
	return c.Set(ctx, key, value, ttl).Err()
}

func DelCache(ctx context.Context, keys ...string) error {
	c := cacheClient
	if c == nil {
		c = client
	}
	return c.Del(ctx, keys...).Err()
}

func ExpireCache(ctx context.Context, key string, ttl time.Duration) error {
	c := cacheClient
	if c == nil {
		c = client
	}
	return c.Expire(ctx, key, ttl).Err()
}

func SAddCache(ctx context.Context, key string, members ...interface{}) error {
	c := cacheClient
	if c == nil {
		c = client
	}
	return c.SAdd(ctx, key, members...).Err()
}

func Init(redisURL string) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("Failed to parse Redis URL: %v", err)
	}
	client = redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Println("Redis connected")
}

func Get(ctx context.Context, key string) (string, error) {
	return client.Get(ctx, key).Result()
}

func Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return client.Set(ctx, key, value, ttl).Err()
}

func SMembers(ctx context.Context, key string) ([]string, error) {
	return client.SMembers(ctx, key).Result()
}

func SAdd(ctx context.Context, key string, members ...interface{}) error {
	return client.SAdd(ctx, key, members...).Err()
}

func Incr(ctx context.Context, key string) (int64, error) {
	return client.Incr(ctx, key).Result()
}

func Decr(ctx context.Context, key string) (int64, error) {
	return client.Decr(ctx, key).Result()
}

func Expire(ctx context.Context, key string, ttl time.Duration) error {
	return client.Expire(ctx, key, ttl).Err()
}

func LRange(ctx context.Context, key string) ([]string, error) {
	return client.LRange(ctx, key, 0, -1).Result()
}

func Del(ctx context.Context, keys ...string) error {
	return client.Del(ctx, keys...).Err()
}

func RPush(ctx context.Context, key string, values ...interface{}) error {
	return client.RPush(ctx, key, values...).Err()
}
func Ping(ctx context.Context) error {
	return client.Ping(ctx).Err()
}

func AtomicQuotaCheck(ctx context.Context, key string, limit int) (bool, error) {
	script := redis.NewScript(`
        local current = redis.call('GET', KEYS[1])
        if current and tonumber(current) >= tonumber(ARGV[1]) then
            return 0
        end
        return redis.call('INCR', KEYS[1])
    `)
	result, err := script.Run(ctx, client, []string{key}, limit).Int64()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}
