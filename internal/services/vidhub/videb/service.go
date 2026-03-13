package videb

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"vidbot-api/pkg/proxy"
)

type Service struct {
	proxy *proxy.Client
}

type Request struct {
	URL string `json:"url" binding:"required"`
}

type ExtractionResult struct {
	Filecode    string `json:"filecode"`
	Title       string `json:"title"`
	Filename    string `json:"filename"`
	DownloadURL string `json:"download_url"`
	Size        int64  `json:"size"`
	Thumbnail   string `json:"thumbnail"`
}

func NewService(proxyClient *proxy.Client) *Service {
	return &Service{proxy: proxyClient}
}

func (s *Service) Extract(rawURL string) (*ExtractionResult, error) {
	ua := proxy.RandomUA()

	filecode := s.parseFilecode(rawURL)
	if filecode == "" {
		return nil, fmt.Errorf("invalid videb url")
	}

	// Step 1: Follow redirect videb.lol → final domain
	step1, err := s.proxy.Get("https://videb.lol/e/"+filecode, map[string]string{
		"User-Agent": ua,
	})
	if err != nil {
		return nil, fmt.Errorf("step1 redirect: %w", err)
	}
	finalURL := step1.Headers["x-final-url"]
	if finalURL == "" {
		finalURL = "https://videb.lol/e/" + filecode
	}
	u, _ := url.Parse(finalURL)
	finalBase := u.Scheme + "://" + u.Host

	// Step 2: Load player page
	playerURL := finalBase + "/player/" + filecode + "?r=ZGlyZWN0"
	step2, err := s.proxy.Get(playerURL, map[string]string{
		"User-Agent":     ua,
		"Referer":        finalBase + "/e/" + filecode,
		"Accept":         "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Sec-Fetch-Dest": "iframe",
		"Sec-Fetch-Mode": "navigate",
		"Sec-Fetch-Site": "same-origin",
		"Sec-Fetch-User": "?1",
	})
	if err != nil {
		return nil, fmt.Errorf("step2 player: %w", err)
	}
	if step2.Status != 200 {
		return nil, fmt.Errorf("step2 player HTTP %d", step2.Status)
	}

	html := step2.Body

	// Step 3: Extract token, title, thumbnail
	token := s.extractToken(html)
	if token == "" {
		return nil, fmt.Errorf("token not found in player page")
	}
	title := s.extractTitle(html, filecode)
	thumbnail := s.extractThumbnail(html)

	// Step 4: GET stream tanpa redirect → ambil Location header
	streamURL := finalBase + "/stream/" + filecode + "?token=" + token
	step4, err := s.proxy.GetNoRedirect(streamURL, map[string]string{
		"User-Agent":     ua,
		"Referer":        playerURL,
		"Accept":         "*/*",
		"Range":          "bytes=0-",
		"Sec-Fetch-Dest": "video",
		"Sec-Fetch-Mode": "no-cors",
		"Sec-Fetch-Site": "same-origin",
	})
	if err != nil {
		return nil, fmt.Errorf("step4 stream: %w", err)
	}

	st := step4.Status
	if st != 301 && st != 302 && st != 307 && st != 308 {
		return nil, fmt.Errorf("step4 unexpected status %d", st)
	}

	cdnURL := step4.Headers["location"]
	if cdnURL == "" {
		return nil, fmt.Errorf("step4 location header not found")
	}

	// Step 5: HEAD untuk file size
	fileSize := int64(0)
	step5, err := s.proxy.Get(cdnURL, map[string]string{
		"User-Agent": ua,
		"Referer":    playerURL,
	})
	if err == nil {
		if cl, ok := step5.Headers["content-length"]; ok {
			fileSize, _ = strconv.ParseInt(cl, 10, 64)
		}
	}

	filename := sanitizeFilename(title + ".mp4")

	return &ExtractionResult{
		Filecode:    filecode,
		Title:       title,
		Filename:    filename,
		DownloadURL: cdnURL,
		Size:        fileSize,
		Thumbnail:   thumbnail,
	}, nil
}

func (s *Service) parseFilecode(rawURL string) string {
	re := regexp.MustCompile(`/e/([a-zA-Z0-9]+)`)
	if m := re.FindStringSubmatch(rawURL); len(m) >= 2 {
		return m[1]
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

func (s *Service) extractToken(html string) string {
	re := regexp.MustCompile(`token:\s*'([^']+)'`)
	if m := re.FindStringSubmatch(html); len(m) >= 2 {
		return m[1]
	}
	re2 := regexp.MustCompile(`token:\s*"([^"]+)"`)
	if m := re2.FindStringSubmatch(html); len(m) >= 2 {
		return m[1]
	}
	return ""
}

func (s *Service) extractTitle(html, filecode string) string {
	re := regexp.MustCompile(`<title>([^<]+)</title>`)
	if m := re.FindStringSubmatch(html); len(m) >= 2 {
		return strings.TrimSpace(m[1])
	}
	return filecode
}

func (s *Service) extractThumbnail(html string) string {
	re := regexp.MustCompile(`poster="([^"]+)"`)
	if m := re.FindStringSubmatch(html); len(m) >= 2 {
		return m[1]
	}
	return ""
}

func sanitizeFilename(name string) string {
	re := regexp.MustCompile(`[<>:"/\\|?*]`)
	return re.ReplaceAllString(name, "_")
}
