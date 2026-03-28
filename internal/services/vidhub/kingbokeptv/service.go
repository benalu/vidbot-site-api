package kingbokeptv

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"vidbot-api/pkg/fileutil"
	"vidbot-api/pkg/proxy"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

type Result struct {
	Filecode  string
	Title     string
	Filename  string
	M3U8URL   string
	Thumbnail string
	Duration  string
}

type Service struct {
	proxyClient *proxy.Client
}

func NewService(proxyClient *proxy.Client) *Service {
	return &Service{proxyClient: proxyClient}
}

func (s *Service) Extract(rawURL string) (*Result, error) {
	// coba via worker dulu
	html, err := s.fetchViaWorker(rawURL)
	if err != nil {
		// fallback tls-client
		html, err = s.fetchViaTLSClient(rawURL)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch page: %w", err)
		}
	}

	m3u8URL := extractM3U8(html)
	if m3u8URL == "" {
		return nil, fmt.Errorf("m3u8 URL not found")
	}

	title := extractTitle(html)
	thumbnail := extractThumbnail(html)
	duration := extractDuration(html)
	filecode := extractFilecode(rawURL)

	if title == "" {
		title = filecode
	}
	filename := fileutil.Sanitize(title) + ".mp4"

	return &Result{
		Filecode:  filecode,
		Title:     title,
		Filename:  filename,
		M3U8URL:   m3u8URL,
		Thumbnail: thumbnail,
		Duration:  duration,
	}, nil
}

func (s *Service) fetchViaWorker(targetURL string) (string, error) {
	headers := map[string]string{
		"User-Agent":                "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Mobile Safari/537.36 Edg/146.0.0.0",
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		"Accept-Language":           "id,en-US;q=0.9,en;q=0.8",
		"Referer":                   "https://kingbokep.tv/",
		"Sec-Fetch-Dest":            "document",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-Site":            "same-origin",
		"Sec-Fetch-User":            "?1",
		"Upgrade-Insecure-Requests": "1",
		"sec-ch-ua":                 `"Chromium";v="146", "Not-A.Brand";v="24", "Microsoft Edge";v="146"`,
		"sec-ch-ua-mobile":          "?1",
		"sec-ch-ua-platform":        `"Android"`,
	}

	resp, err := s.proxyClient.Get(targetURL, headers)
	if err != nil {
		return "", fmt.Errorf("worker GET error: %w", err)
	}
	if resp.Status != 200 {
		return "", fmt.Errorf("worker HTTP %d", resp.Status)
	}
	return resp.Body, nil
}

func (s *Service) fetchViaTLSClient(targetURL string) (string, error) {
	jar := tls_client.NewCookieJar()
	options := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(15),
		tls_client.WithClientProfile(profiles.Chrome_120),
		tls_client.WithCookieJar(jar),
	}

	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		return "", fmt.Errorf("tls client init: %w", err)
	}

	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Mobile Safari/537.36 Edg/146.0.0.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "id,en-US;q=0.9,en;q=0.8")
	req.Header.Set("Referer", "https://kingbokep.tv/")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("tls client do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("tls HTTP %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// extractM3U8 — ambil dari atribut data-playlist pada #bokep-player
func extractM3U8(html string) string {
	// data-playlist="https://stream.kingbokep.video/.../playlist.m3u8"
	re := regexp.MustCompile(`data-playlist=["']([^"']+\.m3u8[^"']*)["']`)
	if m := re.FindStringSubmatch(html); m != nil {
		return m[1]
	}
	return ""
}

// extractTitle — dari <title>...</title>, hilangkan suffix " | KingBokep"
func extractTitle(html string) string {
	re := regexp.MustCompile(`(?i)<title>([^<]+)</title>`)
	if m := re.FindStringSubmatch(html); m != nil {
		title := strings.TrimSpace(m[1])
		// buang suffix " | KingBokep" atau " | KingBokep.tv | ..."
		if idx := strings.LastIndex(title, " | KingBokep"); idx != -1 {
			title = strings.TrimSpace(title[:idx])
		}
		return title
	}
	return ""
}

// extractThumbnail — dari og:image
func extractThumbnail(html string) string {
	re := regexp.MustCompile(`<meta\s+property=["']og:image["']\s+content=["']([^"']+)["']`)
	if m := re.FindStringSubmatch(html); m != nil {
		return m[1]
	}
	// fallback: poster attr di video tag
	re2 := regexp.MustCompile(`poster=["']([^"']+)["']`)
	if m := re2.FindStringSubmatch(html); m != nil {
		return m[1]
	}
	return ""
}

// extractDuration — dari <span data-pagefind-meta="duration">MM:SS</span>
func extractDuration(html string) string {
	re := regexp.MustCompile(`data-pagefind-meta=["']duration["'][^>]*>([^<]+)<`)
	if m := re.FindStringSubmatch(html); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// extractFilecode — slug dari URL path
func extractFilecode(rawURL string) string {
	// https://kingbokep.tv/view/slug-judul/
	re := regexp.MustCompile(`/view/([^/?#]+)`)
	if m := re.FindStringSubmatch(rawURL); m != nil {
		return m[1]
	}
	return "unknown"
}
