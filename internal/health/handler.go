package health

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"
	"vidbot-api/pkg/cache"
	"vidbot-api/pkg/iptvstore"
	"vidbot-api/pkg/leakcheck"
	"vidbot-api/pkg/limiter"
	"vidbot-api/pkg/stats"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	masterKey       string
	cloudConvertKey string
	convertioKey    string
	workerURLs      []string
	workerSecret    string
	startTime       time.Time
}

func NewHandler(masterKey, cloudConvertKey, convertioKey string, workerURLs []string, workerSecret string) *Handler {
	return &Handler{
		masterKey:       masterKey,
		cloudConvertKey: cloudConvertKey,
		convertioKey:    convertioKey,
		workerURLs:      workerURLs,
		workerSecret:    workerSecret,
		startTime:       time.Now(),
	}
}

func (h *Handler) Check(c *gin.Context) {
	if c.GetHeader("X-Master-Key") != h.masterKey {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"code":    "UNAUTHORIZED",
			"message": "Invalid key.",
		})
		return
	}

	// jalankan semua check secara paralel
	type checkResult struct {
		redis        string
		stats        string
		iptv         gin.H
		cloudconvert string
		convertio    string
		workers      gin.H
		leakcheck    gin.H
	}

	var (
		wg  sync.WaitGroup
		res checkResult
		mu  sync.Mutex
	)

	checks := []struct {
		name string
		fn   func()
	}{
		{"redis", func() {
			v := checkRedis()
			mu.Lock()
			res.redis = v
			mu.Unlock()
		}},
		{"stats", func() {
			v := checkStats()
			mu.Lock()
			res.stats = v
			mu.Unlock()
		}},
		{"iptv", func() {
			v := checkIPTV()
			mu.Lock()
			res.iptv = v
			mu.Unlock()
		}},
		{"cloudconvert", func() {
			v := checkCloudConvert(h.cloudConvertKey)
			mu.Lock()
			res.cloudconvert = v
			mu.Unlock()
		}},
		{"convertio", func() {
			v := checkConvertio(h.convertioKey)
			mu.Lock()
			res.convertio = v
			mu.Unlock()
		}},
		{"leakcheck", func() {
			v := checkLeakcheck()
			mu.Lock()
			res.leakcheck = v
			mu.Unlock()
		}},
		{"workers", func() {
			v := checkWorkers(h.workerURLs, h.workerSecret)
			mu.Lock()
			res.workers = v
			mu.Unlock()
		}},
	}

	wg.Add(len(checks))
	for _, ch := range checks {
		ch := ch
		go func() {
			defer wg.Done()
			ch.fn()
		}()
	}
	wg.Wait()

	// tentukan status overall
	overallStatus := "ok"
	if res.redis == "down" || res.stats == "down" {
		overallStatus = "degraded"
	}

	uptime := time.Since(h.startTime)

	response := gin.H{
		"status":    overallStatus,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"uptime":    formatUptime(uptime),
		"system": gin.H{
			"goroutines":   runtime.NumGoroutine(),
			"hls_slots":    fmt.Sprintf("%d/%d", limiter.HLSDownload.Current(), limiter.HLSDownload.Max()),
			"direct_slots": fmt.Sprintf("%d/%d", limiter.DirectStream.Current(), limiter.DirectStream.Max()),
		},
		"services": gin.H{
			"redis": res.redis,
			"stats": res.stats,
		},
		"leakcheck": res.leakcheck,
		"iptv":      res.iptv,
		"providers": gin.H{
			"cloudconvert": res.cloudconvert,
			"convertio":    res.convertio,
		},
		"workers": res.workers,
	}

	if overallStatus == "degraded" {
		c.JSON(http.StatusServiceUnavailable, response)
		return
	}

	c.JSON(http.StatusOK, response)
}

// =====================
// Individual Checks
// =====================

func checkRedis() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := cache.Ping(ctx); err != nil {
		return "down"
	}
	return "ok"
}

func checkLeakcheck() gin.H {
	if err := leakcheck.Default.Ping(); err != nil {
		return gin.H{
			"status":  "down",
			"entries": 0,
		}
	}
	return gin.H{
		"status":  "ok",
		"entries": leakcheck.Default.Count(),
	}
}

func checkStats() string {
	if stats.DB == nil {
		return "down"
	}
	if err := stats.DB.Ping(); err != nil {
		return "down"
	}
	return "ok"
}

func checkIPTV() gin.H {
	channels := iptvstore.Default.GetChannels("", "", false)
	streams := iptvstore.Default.GetChannels("", "", true)
	countries := iptvstore.Default.GetCountries()
	categories := iptvstore.Default.GetCategories()

	status := "ok"
	if len(channels) == 0 {
		status = "empty"
	}

	return gin.H{
		"status":      status,
		"channels":    len(channels),
		"with_stream": len(streams),
		"countries":   len(countries),
		"categories":  len(categories),
	}
}

func checkCloudConvert(apiKey string) string {
	if apiKey == "" {
		return "no_key"
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", "https://api.cloudconvert.com/v2/users/me", nil)
	if err != nil {
		return "down"
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return "down"
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return "ok"
	}
	if resp.StatusCode == 401 {
		return "invalid_key"
	}
	return fmt.Sprintf("error_%d", resp.StatusCode)
}

func checkConvertio(apiKey string) string {
	if apiKey == "" {
		return "no_key"
	}

	client := &http.Client{Timeout: 5 * time.Second}
	url := fmt.Sprintf("https://api.convertio.co/convert/%s/status", apiKey)
	resp, err := client.Get(url)
	if err != nil {
		return "down"
	}
	defer resp.Body.Close()

	// convertio return 404 untuk invalid job ID tapi koneksi berhasil
	if resp.StatusCode == 404 || resp.StatusCode == 200 {
		return "ok"
	}
	if resp.StatusCode == 401 {
		return "invalid_key"
	}
	return fmt.Sprintf("error_%d", resp.StatusCode)
}

func checkWorkers(workerURLs []string, workerSecret string) gin.H {
	if len(workerURLs) == 0 {
		return gin.H{
			"total":     0,
			"reachable": 0,
			"status":    "no_workers",
		}
	}

	reachable := 0
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, url := range workerURLs {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			client := &http.Client{Timeout: 3 * time.Second}
			req, err := http.NewRequest("GET", u, nil)
			if err != nil {
				return
			}
			req.Header.Set("X-Worker-Secret", workerSecret)
			resp, err := client.Do(req)
			if err != nil {
				return
			}
			resp.Body.Close()
			// worker dianggap reachable kalau dapat response apapun
			mu.Lock()
			reachable++
			mu.Unlock()
		}(url)
	}
	wg.Wait()

	status := "ok"
	if reachable == 0 {
		status = "down"
	} else if reachable < len(workerURLs) {
		status = "partial"
	}

	return gin.H{
		"total":     len(workerURLs),
		"reachable": reachable,
		"status":    status,
	}
}

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
