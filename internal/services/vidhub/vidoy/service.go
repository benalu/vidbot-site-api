package vidoy

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"vidbot-api/pkg/proxy"
)

type Service struct {
	proxy *proxy.Client
}

func NewService(proxyClient *proxy.Client) *Service {
	return &Service{
		proxy: proxyClient,
	}
}

func (s *Service) Extract(rawURL string) (*ExtractionResult, error) {
	ua := proxy.RandomUA()
	// Step 1: Follow redirect vidstrm.cloud → vide63.com
	step1, err := s.proxy.Get(rawURL, map[string]string{
		"User-Agent": ua,
	})
	if err != nil {
		return nil, fmt.Errorf("step1 redirect: %w", err)
	}

	finalURL := step1.Headers["x-final-url"]
	if finalURL == "" {
		finalURL = rawURL
	}
	parsedFinal, _ := url.Parse(finalURL)
	baseURL := parsedFinal.Scheme + "://" + parsedFinal.Host

	// ambil filecode dari /d/{filecode}
	filecodeRe := regexp.MustCompile(`/d/([a-zA-Z0-9]+)`)
	fcMatch := filecodeRe.FindStringSubmatch(parsedFinal.Path)
	if len(fcMatch) < 2 {
		return nil, fmt.Errorf("filecode not found after redirect")
	}
	filecode := fcMatch[1]
	html := step1.Body

	// Step 2: Parse HTML — title, iframe path, iframeId
	title := filecode
	titleRe := regexp.MustCompile(`(?i)<h4>(.*?)</h4>`)
	if m := titleRe.FindStringSubmatch(html); len(m) >= 2 {
		raw := strings.TrimSpace(m[1])
		extRe := regexp.MustCompile(`(?i)\.(mp4|mkv|avi|mov|webm)$`)
		title = extRe.ReplaceAllString(raw, "")
	}

	iframePathRe := regexp.MustCompile(`iframe\.src\s*=\s*'(/[^']+)\?id='\s*\+\s*iframeId`)
	iframeIDRe := regexp.MustCompile(`var iframeId\s*=\s*'([a-fA-F0-9]+)'`)

	iframePathMatch := iframePathRe.FindStringSubmatch(html)
	iframeIDMatch := iframeIDRe.FindStringSubmatch(html)
	if len(iframePathMatch) < 2 || len(iframeIDMatch) < 2 {
		return nil, fmt.Errorf("iframe path or iframeId not found")
	}

	iframePath := iframePathMatch[1]
	iframeID := iframeIDMatch[1]
	iframeURL := baseURL + iframePath + "?id=" + iframeID

	// Step 3: Request iframe URL — ambil cookie vf dan embed URL
	step3, err := s.proxy.Get(iframeURL, map[string]string{
		"User-Agent":     ua,
		"Referer":        finalURL,
		"Sec-Fetch-Dest": "iframe",
		"Sec-Fetch-Mode": "navigate",
		"Sec-Fetch-Site": "same-origin",
	})
	if err != nil {
		return nil, fmt.Errorf("step3 iframe: %w", err)
	}

	vfCookie := ""
	vfRe := regexp.MustCompile(`(vf=[^;]+)`)
	if m := vfRe.FindStringSubmatch(step3.Headers["set-cookie"]); len(m) >= 2 {
		vfCookie = m[1]
	}
	if vfCookie == "" {
		return nil, fmt.Errorf("cookie vf not found")
	}

	embedRe := regexp.MustCompile(`(?:playerPath|fullURL)\s*=\s*"(https?://[^"]+embed\.php[^"]+)"`)
	embedMatch := embedRe.FindStringSubmatch(step3.Body)
	if len(embedMatch) < 2 {
		return nil, fmt.Errorf("embed URL not found")
	}
	embedURL := embedMatch[1]

	// Step 4: Request embed URL dengan cookie vf
	step4, err := s.proxy.Get(embedURL, map[string]string{
		"User-Agent":     ua,
		"Referer":        iframeURL,
		"Cookie":         vfCookie,
		"Sec-Fetch-Dest": "iframe",
		"Sec-Fetch-Mode": "navigate",
		"Sec-Fetch-Site": "same-origin",
	})
	if err != nil {
		return nil, fmt.Errorf("step4 embed: %w", err)
	}

	videoRe := regexp.MustCompile(`(?i)<source\s+src="(https?://[^"]+)"`)
	videoMatch := videoRe.FindStringSubmatch(step4.Body)
	if len(videoMatch) < 2 {
		return nil, fmt.Errorf("video URL not found in embed page")
	}
	videoURL := strings.ReplaceAll(videoMatch[1], "&amp;", "&")

	thumbnail := ""
	posterRe := regexp.MustCompile(`(?i)poster="(https?://[^"]+)"`)
	if m := posterRe.FindStringSubmatch(step4.Body); len(m) >= 2 {
		thumbnail = m[1]
	}

	filename := sanitizeFilename(title) + ".mp4"

	return &ExtractionResult{
		Filecode:    filecode,
		Title:       title,
		Filename:    filename,
		DownloadURL: videoURL,
		Size:        0,
		Thumbnail:   thumbnail,
	}, nil
}

func sanitizeFilename(name string) string {
	re := regexp.MustCompile(`[<>:"/\\|?*]`)
	return re.ReplaceAllString(name, "_")
}
