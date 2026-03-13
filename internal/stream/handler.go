package stream

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"vidbot-api/pkg/downloader"
	"vidbot-api/pkg/limiter"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	client *http.Client
}

func NewHandler() *Handler {
	return &Handler{
		client: &http.Client{Timeout: 0},
	}
}

func (h *Handler) Stream(c *gin.Context, streamSecret, toolsDir string) {
	encodedURL := c.Query("url")
	if encodedURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url is required"})
		return
	}

	payload, err := downloader.DecodePayload(encodedURL)
	if err != nil {
		h.serveHoneypot(c)
		return
	}

	if payload.Secret != streamSecret {
		h.serveHoneypot(c)
		return
	}

	mediaType := downloader.DetectMediaType(payload.URL)

	switch mediaType {
	case downloader.TypeM3U8, downloader.TypeHLS, downloader.TypeTS:
		h.streamViaYTDLP(c, payload, toolsDir)
	default:
		h.streamDirect(c, payload)
	}
}

func (h *Handler) streamDirect(c *gin.Context, payload *downloader.Payload) {
	if !limiter.DirectStream.Acquire() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"code":    "SERVER_BUSY",
			"message": fmt.Sprintf("Server busy, only %d concurrent downloads allowed. Please try again later.", limiter.DirectStream.Max()),
		})
		return
	}
	defer limiter.DirectStream.Release()

	ext := payload.Ext
	if ext == "" {
		ext = "mp4"
	}
	filename := downloader.ResolveFilename(payload.Title, payload.Filename, payload.Filecode, ext)

	req, err := http.NewRequestWithContext(c.Request.Context(), "GET", payload.URL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	if rangeHeader := c.GetHeader("Range"); rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		if c.Request.Context().Err() != nil {
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 206 {
		c.JSON(resp.StatusCode, gin.H{"error": fmt.Sprintf("upstream HTTP %d", resp.StatusCode)})
		return
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("Content-Type", contentType)
	c.Header("Accept-Ranges", "bytes")
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		c.Header("Content-Length", cl)
	}
	if cr := resp.Header.Get("Content-Range"); cr != "" {
		c.Header("Content-Range", cr)
	}
	c.Status(resp.StatusCode)

	buf := make([]byte, 32*1024)
	c.Stream(func(w io.Writer) bool {
		select {
		case <-c.Request.Context().Done():
			return false
		default:
		}
		n, err := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
		}
		return err == nil
	})
}

func (h *Handler) streamViaYTDLP(c *gin.Context, payload *downloader.Payload, toolsDir string) {
	if !limiter.HLSDownload.Acquire() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"code":    "SERVER_BUSY",
			"message": fmt.Sprintf("Server busy, only %d concurrent HLS downloads allowed. Please try again later.", limiter.HLSDownload.Max()),
		})
		return
	}
	defer limiter.HLSDownload.Release()

	filename := downloader.ResolveFilename(payload.Title, payload.Filename, payload.Filecode, "mp4")

	ytdlpPath := "yt-dlp"
	if toolsDir != "" {
		ytdlpPath = toolsDir + "/yt-dlp"
	}

	parsed, _ := url.Parse(payload.URL)
	referer := fmt.Sprintf("%s://%s/", parsed.Scheme, parsed.Host)

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	cmd := exec.CommandContext(ctx,
		ytdlpPath,
		"-f", "best[ext=mp4]/best",
		"-o", "-",
		"--no-playlist",
		"--no-check-certificate",
		"--no-part",
		"--concurrent-fragments", "1",
		"--hls-prefer-native",
		"--quiet",
		"--no-warnings",
		"--user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
		"--referer", referer,
		"--add-header", "Accept: */*",
		"--add-header", "Accept-Language: en-US,en;q=0.9,id;q=0.8",
		"--add-header", "Sec-Fetch-Dest: video",
		"--add-header", "Sec-Fetch-Mode: cors",
		"--add-header", "Sec-Fetch-Site: same-origin",
		"--socket-timeout", "30",
		"--retries", "10",
		"--fragment-retries", "10",
		payload.URL,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to initialize stream"})
		return
	}

	stderr, _ := cmd.StderrPipe()
	go func() {
		b := make([]byte, 512)
		for {
			n, err := stderr.Read(b)
			if n > 0 {
				log.Printf("[yt-dlp] %s", string(b[:n]))
			}
			if err != nil {
				break
			}
		}
	}()

	if err := cmd.Start(); err != nil {
		log.Printf("[stream] yt-dlp start error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start stream processor"})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("Content-Type", "video/mp4")
	c.Header("Transfer-Encoding", "chunked")
	c.Status(http.StatusOK)

	buf := make([]byte, 64*1024)
	c.Stream(func(w io.Writer) bool {
		select {
		case <-ctx.Done():
			cmd.Process.Kill()
			return false
		default:
		}
		n, err := stdout.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
		}
		return err == nil
	})
	cmd.Process.Kill()
	cmd.Wait()
}

func (h *Handler) serveHoneypot(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"status":  "ready",
		"url":     "https://cdn-media-delivery.storage.googleapis.com/secure/stream/v2/eyJhbGciOiJSUzI1NiJ9/media.mp4",
		"token":   "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJtZWRpYSJ9",
		"expires": 1999999999,
	})
}
