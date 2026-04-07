package downr

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"sort"
	"strings"
	"time"
	"vidbot-api/internal/services/content/provider"
	"vidbot-api/pkg/proxy"
)

type Downr struct {
	client *proxy.Client
}

func New(client *proxy.Client) *Downr {
	return &Downr{client: client}
}

func (d *Downr) Name() string {
	return "downr"
}

func (d *Downr) Extract(url string) (*provider.MediaResult, error) {
	// pilih satu worker, pakai konsisten untuk seluruh sesi
	workerURL := d.client.PickWorker()

	sessionCookie, err := d.getSessionCookieFromWorker(workerURL)
	if err != nil {
		return nil, fmt.Errorf("session cookie: %w", err)
	}

	time.Sleep(time.Duration(rand.Intn(700)+500) * time.Millisecond)

	data, err := d.requestVideoInfoFromWorker(workerURL, url, sessionCookie)
	if err != nil {
		return nil, fmt.Errorf("video info: %w", err)
	}

	return d.parseResult(data), nil
}

func (d *Downr) getSessionCookieFromWorker(workerURL string) (string, error) {
	analyticsURL := "https://downr.cc/.netlify/functions/analytics"
	ua := proxy.RandomUA()
	headers := getDownrHeaders(ua, "")

	resp, err := d.client.GetFromWorker(workerURL, analyticsURL, headers)
	if err != nil {
		return "", err
	}

	setCookie := resp.Headers["set-cookie"]
	if setCookie == "" {
		return "", fmt.Errorf("no set-cookie header")
	}

	re := regexp.MustCompile(`session_id=[^;]+`)
	if m := re.FindString(setCookie); m != "" {
		return m, nil
	}

	if idx := strings.Index(setCookie, ";"); idx > 0 {
		return setCookie[:idx], nil
	}
	return setCookie, nil
}

func (d *Downr) requestVideoInfoFromWorker(workerURL, url, sessionCookie string) (map[string]interface{}, error) {
	nytURL := "https://downr.cc/.netlify/functions/nyt"
	ua := proxy.RandomUA()
	headers := getDownrHeaders(ua, sessionCookie)

	bodyBytes, _ := json.Marshal(map[string]string{"url": url})

	resp, err := d.client.DoFromWorker(workerURL, "POST", nytURL, headers, string(bodyBytes), "follow")
	if err != nil {
		return nil, err
	}

	// retry dengan worker yang SAMA
	if resp.Status == 403 && strings.Contains(resp.Body, "user_retry_required") {
		time.Sleep(time.Duration(rand.Intn(1000)+1000) * time.Millisecond)
		newCookie, err := d.getSessionCookieFromWorker(workerURL)
		if err != nil {
			return nil, err
		}
		headers = getDownrHeaders(proxy.RandomUA(), newCookie)
		resp, err = d.client.DoFromWorker(workerURL, "POST", nytURL, headers, string(bodyBytes), "follow")
		if err != nil {
			return nil, err
		}
	}

	if resp.Status != 200 {
		preview := resp.Body
		if len(preview) > 200 {
			preview = preview[:200]
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.Status, preview)
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(resp.Body), &data); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if errMsg, ok := data["error"].(string); ok && errMsg != "" {
		return nil, fmt.Errorf("downr error: %s", errMsg)
	}

	return data, nil
}

func (d *Downr) parseResult(data map[string]interface{}) *provider.MediaResult {
	result := &provider.MediaResult{
		Title:     getString(data, "title"),
		Thumbnail: getString(data, "thumbnail"),
		Duration:  getString(data, "duration"),
		URL:       getString(data, "url"),
		TrackID:   getString(data, "track_id"),
		Author: provider.Author{
			Name:     getString(data, "author"),
			Username: getOwnerUsername(data),
		},
	}

	// view_count dan like_count
	if v, ok := data["view_count"].(float64); ok {
		result.ViewCount = int64(v)
	}
	if v, ok := data["like_count"].(float64); ok {
		result.LikeCount = int64(v)
	}

	// duration dari float kalau string kosong
	if result.Duration == "" {
		if v, ok := data["duration"].(float64); ok && v > 0 {
			result.Duration = fmt.Sprintf("%.0f", v)
		}
	}

	source := getString(data, "source")

	qualityPriority := map[string]int{
		"hd_no_watermark": 4,
		"no_watermark":    3,
		"hd":              2,
		"watermark":       1,
		"sd":              0,
	}

	medias, _ := data["medias"].([]interface{})

	// threads — pakai MediaItems
	if source == "threads" {
		for i, m := range medias {
			media, ok := m.(map[string]interface{})
			if !ok {
				continue
			}
			mediaType := getString(media, "type")
			ext := getString(media, "extension")
			if ext == "" {
				if mediaType == "video" {
					ext = "mp4"
				} else {
					ext = "jpg"
				}
			}
			result.MediaItems = append(result.MediaItems, provider.MediaItem{
				Index:     i,
				Type:      mediaType,
				URL:       getString(media, "url"),
				Thumbnail: getString(media, "thumbnail"),
				Extension: ext,
			})
		}
		return result
	}

	// platform lain — pakai Videos/Audio
	for _, m := range medias {
		media, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		switch getString(media, "type") {
		case "video":
			result.Videos = append(result.Videos, provider.Video{
				Quality:   getString(media, "quality"),
				URL:       getString(media, "url"),
				Extension: getString(media, "extension"),
			})
		case "audio":
			result.Audio = &provider.Audio{
				URL:       getString(media, "url"),
				Quality:   getString(media, "quality"),
				Extension: getString(media, "extension"),
			}
		}
	}

	sort.Slice(result.Videos, func(i, j int) bool {
		return qualityPriority[result.Videos[i].Quality] > qualityPriority[result.Videos[j].Quality]
	})

	return result
}

// helper untuk ambil username dari owner object
func getOwnerUsername(data map[string]interface{}) string {
	// cek unique_id dulu (TikTok)
	if v := getString(data, "unique_id"); v != "" {
		return v
	}
	// cek owner.username (Instagram)
	if owner, ok := data["owner"].(map[string]interface{}); ok {
		return getString(owner, "username")
	}
	return ""
}
func getDownrHeaders(ua, sessionCookie string) map[string]string {
	headers := map[string]string{
		"User-Agent":      ua,
		"Accept":          "*/*",
		"Accept-Language": "en-US,en;q=0.9",
		"Origin":          "https://downr.cc",
		"Referer":         "https://downr.cc/",
		"Sec-Fetch-Dest":  "empty",
		"Sec-Fetch-Mode":  "cors",
		"Sec-Fetch-Site":  "same-origin",
	}

	re := regexp.MustCompile(`Chrome/(\d+)`)
	if m := re.FindStringSubmatch(ua); len(m) >= 2 && !strings.Contains(ua, "Edg") {
		v := m[1]
		headers["sec-ch-ua"] = fmt.Sprintf(`"Not(A:Brand";v="8", "Chromium";v="%s", "Google Chrome";v="%s"`, v, v)
		headers["sec-ch-ua-mobile"] = "?0"
		switch {
		case strings.Contains(ua, "Windows"):
			headers["sec-ch-ua-platform"] = `"Windows"`
		case strings.Contains(ua, "Macintosh"):
			headers["sec-ch-ua-platform"] = `"macOS"`
		default:
			headers["sec-ch-ua-platform"] = `"Linux"`
		}
	}

	if sessionCookie != "" {
		headers["Cookie"] = sessionCookie
		headers["Content-Type"] = "application/json"
	}

	return headers
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
