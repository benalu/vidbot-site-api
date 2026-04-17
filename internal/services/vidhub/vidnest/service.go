package vidnest

import (
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"vidbot-api/pkg/fileutil"
	"vidbot-api/pkg/proxy"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

type Result struct {
	Filecode    string
	Title       string
	Filename    string
	DownloadURL string
	Thumbnail   string
	Duration    int
	Size        int64
}

type Service struct {
	proxyClient *proxy.Client
}

func NewService(proxyClient *proxy.Client) *Service {
	return &Service{proxyClient: proxyClient}
}

func (s *Service) Extract(rawURL string) (*Result, error) {
	filecode := extractFilecode(rawURL)
	if filecode == "" {
		return nil, fmt.Errorf("invalid vidnest URL")
	}

	html, err := s.fetchViaWorker(rawURL)
	if err != nil {
		slog.Debug("worker fetch failed, trying tls client", "platform", "vidnest", "error", err)
		html, err = s.fetchViaTLSClient(rawURL)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch page: %w", err)
		}
	}

	downloadURL := extractDownloadURL(html)
	if downloadURL == "" {
		return nil, fmt.Errorf("failed to extract download URL")
	}

	title := extractTitle(html, filecode)
	thumbnail := extractThumbnail(html)
	duration := extractDuration(html)
	size := extractSize(html, duration)
	filename := fileutil.Sanitize(title) + ".mp4"

	return &Result{
		Filecode:    filecode,
		Title:       title,
		Filename:    filename,
		DownloadURL: downloadURL,
		Thumbnail:   thumbnail,
		Duration:    duration,
		Size:        size,
	}, nil
}

func (s *Service) fetchViaWorker(targetURL string) (string, error) {
	headers := map[string]string{
		"Referer":    "https://vidnest.io/",
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Accept":     "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
	}

	resp, err := s.proxyClient.Get(targetURL, headers)
	if err != nil {
		return "", err
	}
	if resp.Status != 200 {
		return "", fmt.Errorf("worker HTTP %d", resp.Status)
	}
	return resp.Body, nil
}

func (s *Service) fetchViaTLSClient(targetURL string) (string, error) {
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return "", err
	}
	baseURL := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)

	jar := tls_client.NewCookieJar()
	options := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(15),
		tls_client.WithClientProfile(profiles.Chrome_120),
		tls_client.WithCookieJar(jar),
	}

	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", baseURL+"/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func extractFilecode(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) > 0 && segments[0] != "" {
		return segments[0]
	}
	return ""
}

func extractTitle(html, fallback string) string {
	if m := regexp.MustCompile(`<title>(.+?)</title>`).FindStringSubmatch(html); m != nil {
		title := strings.TrimSpace(m[1])
		if strings.HasPrefix(strings.ToLower(title), "watch ") {
			title = title[6:]
		}
		if title != "" {
			return strings.TrimSpace(title)
		}
	}
	return fallback
}

func extractDownloadURL(html string) string {
	patterns := []string{
		`sources:\s*\[\s*\{\s*file:\s*["']([^"']+)["']`,
		`file:\s*["']([^"']+\.mp4[^"']*)["']`,
	}
	for _, p := range patterns {
		if m := regexp.MustCompile(p).FindStringSubmatch(html); m != nil {
			return m[1]
		}
	}
	return ""
}

func extractThumbnail(html string) string {
	if m := regexp.MustCompile(`image:\s*["']([^"']+)["']`).FindStringSubmatch(html); m != nil {
		return m[1]
	}
	return ""
}

func extractDuration(html string) int {
	if m := regexp.MustCompile(`duration:\s*["']?(\d+(?:\.\d+)?)["']?`).FindStringSubmatch(html); m != nil {
		if f, err := strconv.ParseFloat(m[1], 64); err == nil {
			return int(f)
		}
	}
	return 0
}

func extractSize(html string, duration int) int64 {
	if m := regexp.MustCompile(`Content-Length:\s*(\d+)`).FindStringSubmatch(html); m != nil {
		if n, err := strconv.ParseInt(m[1], 10, 64); err == nil {
			return n
		}
	}
	if duration > 0 {
		const bitrateKbps = 767
		return int64(bitrateKbps * 1024 * duration / 8)
	}
	return 0
}
