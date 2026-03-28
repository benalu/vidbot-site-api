package vidarato

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
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
	CDNOrigin string
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
		return nil, fmt.Errorf("invalid vidarato URL or missing filecode")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	baseURL := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)

	title := s.fetchTitle(rawURL, baseURL)
	m3u8URL, thumbnail, err := s.fetchStreamURL(baseURL, filecode, rawURL)
	if err != nil {
		return nil, err
	}

	if title == "" {
		title = filecode
	}
	title = regexp.MustCompile(`(?i)\.mp4$`).ReplaceAllString(title, "")
	filename := fileutil.Sanitize(title) + ".mp4"

	return &Result{
		Filecode:  filecode,
		Title:     title,
		Filename:  filename,
		M3U8URL:   m3u8URL,
		Thumbnail: thumbnail,
		CDNOrigin: extractCDNOrigin(m3u8URL),
	}, nil
}

func extractCDNOrigin(m3u8URL string) string {
	parsed, err := url.Parse(m3u8URL)
	if err != nil || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func (s *Service) fetchTitle(pageURL, baseURL string) string {
	// coba worker dulu
	html, err := s.fetchViaWorker(pageURL, baseURL)
	if err != nil {
		// fallback tls-client
		html, err = s.fetchViaTLSClient(pageURL, baseURL)
		if err != nil {
			return ""
		}
	}
	if m := regexp.MustCompile(`<title>(.*?)</title>`).FindStringSubmatch(html); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func (s *Service) fetchStreamURL(baseURL, filecode, referer string) (string, string, error) {
	apiURL := fmt.Sprintf("%s/api/stream", baseURL)

	// coba worker dulu
	body, err := s.fetchViaWorkerPOST(apiURL, baseURL, filecode)
	if err != nil {
		body, err = s.fetchViaTLSClientPOST(apiURL, baseURL, filecode)
		if err != nil {
			return "", "", fmt.Errorf("stream API failed: %w", err)
		}
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		return "", "", fmt.Errorf("parse API response: %w", err)
	}

	m3u8URL, _ := data["streaming_url"].(string)
	thumbnail, _ := data["thumbnail"].(string)

	if m3u8URL == "" {
		return "", "", fmt.Errorf("streaming URL not found")
	}

	return m3u8URL, thumbnail, nil
}

func (s *Service) fetchViaWorker(targetURL, baseURL string) (string, error) {
	headers := map[string]string{
		"Referer":    baseURL + "/",
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
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

func (s *Service) fetchViaWorkerPOST(targetURL, baseURL, filecode string) (string, error) {
	bodyBytes, _ := json.Marshal(map[string]string{"filecode": filecode})
	headers := map[string]string{
		"Content-Type": "application/json",
		"Referer":      baseURL + "/",
		"User-Agent":   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}

	resp, err := s.proxyClient.Do("POST", targetURL, headers, string(bodyBytes), "follow")
	if err != nil {
		return "", err
	}
	if resp.Status != 200 {
		return "", fmt.Errorf("worker HTTP %d", resp.Status)
	}
	return resp.Body, nil
}

func (s *Service) fetchViaTLSClientPOST(targetURL, baseURL, filecode string) (string, error) {
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

	bodyBytes, _ := json.Marshal(map[string]string{"filecode": filecode})
	req, err := http.NewRequest(http.MethodPost, targetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
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

func (s *Service) fetchViaTLSClient(targetURL, baseURL string) (string, error) {
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
	setHeaders(req, baseURL+"/")

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

func setHeaders(req *http.Request, referer string) {
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", referer)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("sec-ch-ua", `"Not(A:Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"Windows"`)
}

func extractFilecode(rawURL string) string {
	if m := regexp.MustCompile(`/[ev]/([a-zA-Z0-9]+)`).FindStringSubmatch(rawURL); m != nil {
		return m[1]
	}
	return ""
}
