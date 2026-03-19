package vidbos

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"
	"vidbot-api/pkg/fileutil"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

type Result struct {
	Filecode    string
	Title       string
	Filename    string
	DownloadURL string
}

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Extract(url string) (*Result, error) {
	filecode := extractFilecode(url)
	if filecode == "" {
		return nil, fmt.Errorf("invalid vidbos URL or missing filecode")
	}

	videoURL, title, err := s.getVideoURLWithChallenge(url)
	if err != nil {
		return nil, fmt.Errorf("extraction failed: %w", err)
	}

	if videoURL == "" {
		return nil, fmt.Errorf("no video URL found")
	}

	if title == "" {
		title = filecode
	} else {
		title = regexp.MustCompile(`(?i)\.mp4$`).ReplaceAllString(title, "")
	}

	filename := fileutil.Sanitize(title) + ".mp4"

	return &Result{
		Filecode:    filecode,
		Title:       title,
		Filename:    filename,
		DownloadURL: videoURL,
	}, nil
}

func (s *Service) getVideoURLWithChallenge(watchURL string) (string, string, error) {
	jar := tls_client.NewCookieJar()
	options := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithClientProfile(profiles.Chrome_120),
		tls_client.WithCookieJar(jar),
		tls_client.WithNotFollowRedirects(),
	}

	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		return "", "", fmt.Errorf("tls client init: %w", err)
	}

	// Step 1: simulate referrer dari Google
	refReq, _ := http.NewRequest(http.MethodGet, "https://vidbos.com/", nil)
	setHeaders(refReq, "https://www.google.com/")
	client.Do(refReq)
	time.Sleep(time.Duration(rand.Intn(1000)+1000) * time.Millisecond)

	// Step 2: fetch halaman video
	req, err := http.NewRequest(http.MethodGet, watchURL, nil)
	if err != nil {
		return "", "", err
	}
	setHeaders(req, "https://vidbos.com/")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("fetch page: %w", err)
	}
	defer resp.Body.Close()

	html := readBody(resp)

	// Step 3: check CF challenge
	if strings.Contains(html, "One moment, please") || strings.Contains(html[:min(1000, len(html))], "setTimeout") {
		params := extractChallengeParams(html)
		if params == nil {
			return "", "", fmt.Errorf("failed to extract challenge params")
		}

		time.Sleep(time.Duration(rand.Intn(1000)+500) * time.Millisecond)

		// build challenge URL
		challengeURL := fmt.Sprintf("https://vidbos.com%s?wsidchk=%s&pdata=%s&id=%s&ts=%s",
			params["path"], params["wsidchk"], params["pdata"], params["id"], params["ts"],
		)

		chalReq, _ := http.NewRequest(http.MethodGet, challengeURL, nil)
		setHeaders(chalReq, "https://vidbos.com/")
		chalResp, err := client.Do(chalReq)
		if err != nil {
			return "", "", fmt.Errorf("challenge request: %w", err)
		}
		chalResp.Body.Close()

		if chalResp.StatusCode != 302 {
			return "", "", fmt.Errorf("challenge failed: HTTP %d", chalResp.StatusCode)
		}

		time.Sleep(time.Duration(rand.Intn(1000)+500) * time.Millisecond)

		// fetch ulang dengan cookie baru
		req2, _ := http.NewRequest(http.MethodGet, watchURL, nil)
		setHeaders(req2, "https://vidbos.com/")
		resp2, err := client.Do(req2)
		if err != nil {
			return "", "", fmt.Errorf("refetch after challenge: %w", err)
		}
		defer resp2.Body.Close()
		html = readBody(resp2)
	}

	videoURL := extractVideoURL(html)
	title := extractTitle(html)

	return videoURL, title, nil
}

func setHeaders(req *http.Request, referer string) {
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,id;q=0.8")
	req.Header.Set("Cache-Control", "max-age=0")
	req.Header.Set("DNT", "1")
	req.Header.Set("Referer", referer)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("sec-ch-ua", `"Not(A:Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"Windows"`)
}

func extractChallengeParams(html string) map[string]string {
	sMatch := regexp.MustCompile(`S\s*=\s*\+\(\((.*?)\)\)(?:,|;)`).FindStringSubmatch(html)
	iMatch := regexp.MustCompile(`i\s*=\s*\+\(\((.*?)\)\)(?:,|;)`).FindStringSubmatch(html)
	if sMatch == nil || iMatch == nil {
		return nil
	}

	sVal := decodeJSFuck(sMatch[1])
	iVal := decodeJSFuck(iMatch[1])
	if sVal == "" || iVal == "" {
		return nil
	}

	wsidchk := fmt.Sprintf("%d", mustInt(sVal)+mustInt(iVal))

	idMatch := regexp.MustCompile(`'id',\s*'([0-9a-f]{10})'`).FindStringSubmatch(html)
	lastMatch := regexp.MustCompile(`\+L\([^)]+\)\+'([0-9a-f]{2})'`).FindStringSubmatch(html)
	if idMatch == nil || lastMatch == nil {
		return nil
	}

	part1 := idMatch[1]
	part4 := lastMatch[1]

	allHex := regexp.MustCompile(`'([0-9a-f]{10})'`).FindAllStringSubmatch(html, -1)
	unique := []string{}
	for _, h := range allHex {
		if h[1] != part1 && !contains(unique, h[1]) {
			unique = append(unique, h[1])
		}
	}
	if len(unique) < 2 {
		return nil
	}
	fullID := part1 + unique[0] + unique[1] + part4

	tsMatch := regexp.MustCompile(`'ts',\s*'(\d+)'`).FindStringSubmatch(html)
	pdataMatch := regexp.MustCompile(`o\s*=\s*'([^']+)'`).FindStringSubmatch(html)
	pathMatch := regexp.MustCompile(`I\s*=\s*'(/[^']+)'`).FindStringSubmatch(html)

	if tsMatch == nil || pdataMatch == nil || pathMatch == nil {
		return nil
	}

	return map[string]string{
		"wsidchk": wsidchk,
		"id":      fullID,
		"ts":      tsMatch[1],
		"pdata":   pdataMatch[1],
		"path":    pathMatch[1],
	}
}

func extractVideoURL(html string) string {
	patterns := []string{
		`data-link="([^"]+\.mp4[^"]*)"`,
		`<video[^>]+data-link=["']([^"']+\.mp4)["']`,
		`src=["']([^"']+\.mp4)["']`,
	}
	for _, p := range patterns {
		if m := regexp.MustCompile(p).FindStringSubmatch(html); m != nil {
			return m[1]
		}
	}
	return ""
}

func extractTitle(html string) string {
	patterns := []string{
		`const\s+videoTitle\s*=\s*["']([^"']+)["']`,
		`videoTitle\s*=\s*["']([^"']+)["']`,
		`<title>([^<]+)</title>`,
	}
	for _, p := range patterns {
		if m := regexp.MustCompile(p).FindStringSubmatch(html); m != nil {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

func extractFilecode(url string) string {
	patterns := []string{
		`/([a-zA-Z0-9]+)/watch`,
		`/([a-zA-Z0-9]{15,})`,
	}
	for _, p := range patterns {
		if m := regexp.MustCompile(p).FindStringSubmatch(url); m != nil {
			return m[1]
		}
	}
	return ""
}

func decodeJSFuck(expr string) string {
	expr = strings.Trim(expr, "()")
	parts := strings.Split(expr, ")+(")
	result := ""
	for _, p := range parts {
		count := strings.Count(p, "!![")
		result += fmt.Sprintf("%d", count)
	}
	return result
}

func mustInt(s string) int {
	n := 0
	fmt.Sscanf(s, "%d", &n)
	return n
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func readBody(resp *http.Response) string {
	buf := new(strings.Builder)
	b := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(b)
		if n > 0 {
			buf.Write(b[:n])
		}
		if err != nil {
			break
		}
	}
	return buf.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
