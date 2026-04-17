package admin

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"vidbot-api/pkg/cache"

	"github.com/gin-gonic/gin"
)

// RedisStats — statistik per koneksi Redis
type RedisStats struct {
	Name        string         `json:"name"`
	Status      string         `json:"status"`
	Latency     string         `json:"latency"`
	MemoryUsed  string         `json:"memory_used"`
	MemoryPeak  string         `json:"memory_peak"`
	KeyCount    int64          `json:"key_count"`
	DBSize      int64          `json:"db_size"`
	ConnClients int64          `json:"connected_clients"`
	Uptime      string         `json:"uptime"`
	Version     string         `json:"version"`
	HitRate     string         `json:"hit_rate,omitempty"`
	Keyspace    map[string]any `json:"keyspace,omitempty"`
	// Upstash free tier limits
	Limits *UpstashLimits `json:"limits,omitempty"`
}

// UpstashLimits — estimasi penggunaan vs free tier limit
// Upstash free: 10.000 req/hari, 256MB storage
type UpstashLimits struct {
	StorageUsedMB  float64 `json:"storage_used_mb"`
	StorageLimitMB float64 `json:"storage_limit_mb"`
	StoragePct     float64 `json:"storage_pct"`
	Warning        bool    `json:"warning"` // true kalau > 80%
}

func (h *Handler) GetRedisStats(c *gin.Context) {
	if !h.validateMasterKey(c) {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	results := []RedisStats{}

	// cek main Redis
	mainStats := probeRedis(ctx, "main", false)
	results = append(results, mainStats)

	// cek cache Redis kalau berbeda
	if cache.HasSeparateCache() {
		cacheStats := probeRedis(ctx, "cache", true)
		results = append(results, cacheStats)
	}

	// hitung key counts per prefix
	keySummary := getKeySummary(ctx)

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"checked_at":  time.Now().UTC().Format(time.RFC3339),
		"connections": results,
		"key_summary": keySummary,
	})
}

// probeRedis — ambil info dari Redis/Upstash via INFO command
func probeRedis(ctx context.Context, name string, useCache bool) RedisStats {
	stats := RedisStats{
		Name:   name,
		Status: "down",
	}

	start := time.Now()
	var err error
	var info string

	if useCache {
		err = cache.PingCache(ctx)
		if err == nil {
			info, _ = cache.InfoCache(ctx)
		}
	} else {
		err = cache.Ping(ctx)
		if err == nil {
			info, _ = cache.Info(ctx)
		}
	}
	latency := time.Since(start)

	if err != nil {
		stats.Latency = "—"
		return stats
	}

	stats.Status = "ok"
	stats.Latency = fmt.Sprintf("%dms", latency.Milliseconds())

	if info == "" {
		return stats
	}

	// parse INFO output
	parsed := parseRedisInfo(info)

	stats.Version = parsed["redis_version"]
	stats.MemoryUsed = formatBytes(parseSize(parsed["used_memory"]))
	stats.MemoryPeak = formatBytes(parseSize(parsed["used_memory_peak"]))
	stats.ConnClients = parseInt(parsed["connected_clients"])

	if uptimeSec := parseInt(parsed["uptime_in_seconds"]); uptimeSec > 0 {
		stats.Uptime = formatUptime(time.Duration(uptimeSec) * time.Second)
	}

	// DB size dari keyspace_hits/misses
	hits := parseInt(parsed["keyspace_hits"])
	misses := parseInt(parsed["keyspace_misses"])
	total := hits + misses
	if total > 0 {
		hitPct := float64(hits) / float64(total) * 100
		stats.HitRate = fmt.Sprintf("%.1f%% (%d hits, %d misses)", hitPct, hits, misses)
	}

	// keyspace
	keyspace := map[string]any{}
	for k, v := range parsed {
		if strings.HasPrefix(k, "db") {
			keyspace[k] = v
		}
	}
	if len(keyspace) > 0 {
		stats.Keyspace = keyspace
	}

	// Upstash limits (estimasi dari memory usage)
	memBytes := parseSize(parsed["used_memory"])
	memMB := float64(memBytes) / 1024 / 1024
	const upstashFreeLimitMB = 256.0
	pct := memMB / upstashFreeLimitMB * 100
	stats.Limits = &UpstashLimits{
		StorageUsedMB:  roundFloat(memMB, 2),
		StorageLimitMB: upstashFreeLimitMB,
		StoragePct:     roundFloat(pct, 1),
		Warning:        pct > 80,
	}

	return stats
}

// getKeySummary — hitung jumlah key per prefix yang dipakai vidbot
func getKeySummary(ctx context.Context) map[string]int64 {
	prefixes := map[string]string{
		"api_keys":      "apikeys:*",
		"rate_limits":   "ratelimit:*",
		"content_cache": "content:*",
		"vidhub_cache":  "vidhub:*",
		"shortlinks":    "sl:*",
		"app_shortlink": "app:sl:*",
		"cdn_cache":     "cdn:app:*",
		"features":      "feature:*",
		"providers":     "content:provider:*",
		"admin_session": "admin:session:*",
	}

	result := map[string]int64{}
	for name, pattern := range prefixes {
		count := cache.CountKeys(ctx, pattern)
		result[name] = count
	}
	return result
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func formatUptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func parseRedisInfo(info string) map[string]string {
	result := map[string]string{}
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return result
}

func parseSize(s string) int64 {
	if s == "" {
		return 0
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}

func parseInt(s string) int64 {
	if s == "" {
		return 0
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}

func formatBytes(b int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.2f GB", float64(b)/GB)
	case b >= MB:
		return fmt.Sprintf("%.2f MB", float64(b)/MB)
	case b >= KB:
		return fmt.Sprintf("%.2f KB", float64(b)/KB)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func roundFloat(f float64, places int) float64 {
	pow := 1.0
	for i := 0; i < places; i++ {
		pow *= 10
	}
	return float64(int(f*pow+0.5)) / pow
}
