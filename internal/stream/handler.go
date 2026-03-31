package stream

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"vidbot-api/pkg/downloader"
	"vidbot-api/pkg/fileutil"
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

const (
	chunkSize   = 256 * 1024
	maxChunks   = 2048
	bufferAhead = 8
	sessionTTL  = 10 * time.Minute
)

type streamChunk struct {
	data []byte
	seq  int
}

type hlsSession struct {
	mu       sync.RWMutex
	chunks   []streamChunk
	done     bool
	err      error
	readers  int
	lastRead time.Time
	notify   chan struct{}
	cancel   context.CancelFunc
}

var (
	hlsSessions   = make(map[string]*hlsSession)
	hlsSessionsMu sync.Mutex
)

func (s *hlsSession) freeChunks() {
	s.mu.Lock()
	s.chunks = nil
	s.mu.Unlock()
}

// CancelAllSessions — dipanggil saat server shutdown dari main.go
func CancelAllSessions() {
	hlsSessionsMu.Lock()
	defer hlsSessionsMu.Unlock()
	for _, sess := range hlsSessions {
		if sess.cancel != nil {
			sess.cancel()
		}
	}
	hlsSessions = make(map[string]*hlsSession)
}

func getOrCreateSession(cacheKey, m3u8URL, toolsDir string) *hlsSession {
	hlsSessionsMu.Lock()
	defer hlsSessionsMu.Unlock()

	if sess, ok := hlsSessions[cacheKey]; ok {
		return sess
	}

	ctx, cancel := context.WithCancel(context.Background())

	sess := &hlsSession{
		notify:   make(chan struct{}, 1),
		lastRead: time.Now(),
		cancel:   cancel,
	}
	hlsSessions[cacheKey] = sess

	go func() {
		defer cancel()

		// coba direct HLS dulu (lebih aman dari throttle)
		err := runDirectHLS(ctx, m3u8URL, toolsDir, sess)
		if err != nil && ctx.Err() == nil {
			// fallback ke yt-dlp kalau direct gagal
			log.Printf("[progressive] direct HLS failed (%v), falling back to yt-dlp", err)
			sess.mu.Lock()
			sess.chunks = nil // reset chunks
			sess.done = false
			sess.err = nil
			sess.mu.Unlock()
			err = runYTDLP(ctx, m3u8URL, toolsDir, sess)
		}

		if err != nil && ctx.Err() == nil {
			log.Printf("[progressive] all methods failed: %v", err)
			sess.mu.Lock()
			sess.err = err
			sess.done = true
			sess.mu.Unlock()
			sess.signalNew()
		}

		time.AfterFunc(sessionTTL, func() {
			hlsSessionsMu.Lock()
			delete(hlsSessions, cacheKey)
			hlsSessionsMu.Unlock()
			sess.freeChunks() // ← tambah ini
			log.Printf("[progressive] session expired: %s", cacheKey)
		})
	}()

	return sess
}

func (s *hlsSession) signalNew() {
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

func (s *hlsSession) appendChunk(data []byte) {
	s.mu.Lock()
	if len(s.chunks) >= maxChunks {
		if !s.done {
			s.err = fmt.Errorf("buffer overflow: video too large (>%d chunks)", maxChunks)
			s.done = true
		}
		s.mu.Unlock()
		s.signalNew()
		return
	}
	chunk := make([]byte, len(data))
	copy(chunk, data)
	seq := len(s.chunks)
	s.chunks = append(s.chunks, streamChunk{data: chunk, seq: seq})
	s.mu.Unlock()
	s.signalNew()
}

func (s *hlsSession) waitForChunk(idx int, ctx context.Context) bool {
	for {
		s.mu.RLock()
		available := len(s.chunks)
		done := s.done
		err := s.err
		s.mu.RUnlock()

		if idx < available {
			return true
		}
		if done || err != nil {
			return false
		}

		select {
		case <-s.notify:
			continue
		case <-ctx.Done():
			return false
		case <-time.After(60 * time.Second):
			return false
		}
	}
}

func (s *hlsSession) totalChunks() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.chunks)
}

func (s *hlsSession) isDone() (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.done, s.err
}

// ============================================================
// Direct HLS — download segment .ts langsung dengan header browser
// lalu pipe ke ffmpeg untuk mux jadi mp4
//
// Kenapa lebih aman:
// - Header identik dengan browser streaming biasa
// - Download sequential (bukan paralel) = pola manusia
// - Cloudflare cache HIT = tidak hit origin server
// - Tidak ada pola "download manager" yang suspicious
// ============================================================

// hlsHeaders — header yang identik dengan browser streaming video
// Ambil dari inspect element langsung
func hlsHeaders(m3u8URL string) map[string]string {
	parsed, _ := url.Parse(m3u8URL)
	origin, referer := resolveOriginReferer(parsed.Host)

	return map[string]string{
		"Origin":             origin,
		"Referer":            referer,
		"User-Agent":         randomUA(), // ← rotate UA
		"Accept":             "*/*",
		"Accept-Language":    "id,en-US;q=0.9,en;q=0.8",
		"sec-ch-ua":          randomSecChUA(), // ← rotate sec-ch-ua
		"sec-ch-ua-mobile":   "?1",
		"sec-ch-ua-platform": `"Android"`,
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "cross-site",
		"Connection":         "keep-alive",
	}
}

func resolveOriginReferer(cdnHost string) (string, string) {
	// exact match dulu
	exactMappings := map[string]string{
		"stream.kingbokep.video": "https://kingbokep.tv",
		"cdn.kingbokep.video":    "https://kingbokep.tv",
	}
	if origin, ok := exactMappings[cdnHost]; ok {
		return origin, origin + "/"
	}

	// suffix match — untuk CDN dengan subdomain random
	// misal: 112b80.s1q2105.com → match "s1q2105.com"
	suffixMappings := map[string]string{
		"s1q2105.com": "https://vidara.so",
		// tambah di sini kalau ada CDN baru dengan pola subdomain random
	}
	for suffix, origin := range suffixMappings {
		if strings.HasSuffix(cdnHost, suffix) {
			return origin, origin + "/"
		}
	}

	// fallback: strip subdomain
	parts := strings.Split(cdnHost, ".")
	if len(parts) >= 2 {
		domain := "https://" + strings.Join(parts[len(parts)-2:], ".")
		return domain, domain + "/"
	}
	return "https://" + cdnHost, "https://" + cdnHost + "/"
}

var mobileUAs = []string{
	"Mozilla/5.0 (Linux; Android 13; SM-G991B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Mobile Safari/537.36 Edg/146.0.0.0",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (Linux; Android 12; Pixel 6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
}

var mobileSecChUAs = []string{
	`"Not(A:Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`,
	`"Not(A:Brand";v="8", "Chromium";v="118", "Google Chrome";v="118"`,
	`"Not(A:Brand";v="8", "Chromium";v="119", "Google Chrome";v="119"`,
}

func randomUA() string {
	return mobileUAs[rand.Intn(len(mobileUAs))]
}

func randomSecChUA() string {
	return mobileSecChUAs[rand.Intn(len(mobileSecChUAs))]
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func fetchPlaylist(ctx context.Context, client *http.Client, m3u8URL string, headers map[string]string) ([]string, string, error) {
	body, baseURL, err := fetchM3U8Body(ctx, client, m3u8URL, headers)
	if err != nil {
		return nil, "", err
	}

	if isMasterPlaylist(body) {
		subURL := extractFirstSubPlaylist(body, baseURL)
		if subURL == "" {
			return nil, "", fmt.Errorf("master playlist: no sub-playlist found")
		}
		body, baseURL, err = fetchM3U8Body(ctx, client, subURL, headers)
		if err != nil {
			return nil, "", fmt.Errorf("fetch sub-playlist: %w", err)
		}
	}

	segments := extractSegments(body, baseURL)
	if len(segments) == 0 {
		return nil, "", fmt.Errorf("no segments found in playlist")
	}

	return segments, body, nil
}

func fetchM3U8Body(ctx context.Context, client *http.Client, m3u8URL string, headers map[string]string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", m3u8URL, nil)
	if err != nil {
		return "", "", err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("playlist HTTP %d", resp.StatusCode)
	}

	b, _ := io.ReadAll(resp.Body)
	parsed, _ := url.Parse(m3u8URL)
	basePath := parsed.Scheme + "://" + parsed.Host + filepath.ToSlash(filepath.Dir(parsed.Path))

	return string(b), basePath, nil
}

func isMasterPlaylist(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lower := strings.ToLower(line)
		return strings.HasSuffix(lower, ".m3u8") || strings.Contains(lower, ".m3u8?")
	}
	return false
}

func extractFirstSubPlaylist(body, baseURL string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "http") {
			return line
		}
		return baseURL + "/" + line
	}
	return ""
}

func extractSegments(body, baseURL string) []string {
	var segments []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "http") {
			segments = append(segments, line)
		} else {
			segments = append(segments, baseURL+"/"+line)
		}
	}
	return segments
}

// runDirectHLS — download .ts segments langsung → pipe ke ffmpeg → chunks
//
// Flow:
// 1. Fetch playlist.m3u8 → dapat list segment URLs
// 2. Spawn ffmpeg dengan input dari stdin (pipe)
// 3. Download tiap .ts segment → kirim ke ffmpeg stdin
// 4. ffmpeg output mp4 → baca ke chunks → client mulai terima data
//
// Kenapa ffmpeg perlu:
// - Concat .ts langsung tidak valid mp4 (tidak ada moov atom)
// - ffmpeg mux .ts stream jadi mp4 yang proper dengan seekable header
// - Client (browser/IDM) butuh valid mp4 container
func runDirectHLS(ctx context.Context, m3u8URL, toolsDir string, sess *hlsSession) error {
	parsed, _ := url.Parse(m3u8URL)
	cdnHost := parsed.Host

	if !limiter.AcquireCDN(cdnHost) {
		return fmt.Errorf("CDN rate limit: too many concurrent downloads to %s", cdnHost)
	}
	defer limiter.ReleaseCDN(cdnHost)
	headers := hlsHeaders(m3u8URL)
	httpClient := &http.Client{
		Timeout: 90 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 10, // ← semua segment ke host yang sama
			IdleConnTimeout:     90 * time.Second,
			DisableCompression:  false,
			// TLS fingerprint — default Go sudah cukup untuk Cloudflare
		},
	}

	// fetch playlist
	segments, _, err := fetchPlaylist(ctx, httpClient, m3u8URL, headers)
	if err != nil {
		return fmt.Errorf("fetch playlist: %w", err)
	}
	if len(segments) == 0 {
		return fmt.Errorf("no segments found in playlist")
	}

	// spawn ffmpeg: baca dari stdin (ts stream), output mp4 ke stdout
	ffmpegPath := "ffmpeg"
	if toolsDir != "" {
		ffmpegPath = filepath.Join(toolsDir, "ffmpeg")
	}

	cmd := exec.CommandContext(ctx,
		ffmpegPath,
		"-i", "pipe:0",
		"-c", "copy",
		"-bsf:a", "aac_adtstoasc", // ← fix AAC bitstream dari .ts
		"-movflags", "frag_keyframe+empty_moov+faststart",
		"-f", "mp4",
		"pipe:1",
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg stdout pipe: %w", err)
	}

	// tangkap stderr ffmpeg untuk logging
	stderr, _ := cmd.StderrPipe()
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// hanya log kalau mengandung kata error/fatal/warning
			lower := strings.ToLower(line)
			if strings.Contains(lower, "error") ||
				strings.Contains(lower, "fatal") ||
				strings.Contains(lower, "invalid") ||
				strings.Contains(lower, "failed") {
				log.Printf("[ffmpeg] %s", line)
			}
		}
	}()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start: %w", err)
	}

	// goroutine 1: download segments → kirim ke ffmpeg stdin
	dlDone := make(chan error, 1)
	go func() {
		defer stdin.Close()
		for i, segURL := range segments {
			select {
			case <-ctx.Done():
				dlDone <- fmt.Errorf("cancelled")
				return
			default:
			}

			// download segment dengan retry
			var segData []byte
			var segErr error
			for attempt := 0; attempt < 3; attempt++ {
				segData, segErr = fetchSegmentData(ctx, httpClient, segURL, headers)
				if segErr == nil {
					break
				}
				log.Printf("[direct_hls] segment %d attempt %d failed: %v", i, attempt+1, segErr)
				// jeda sebentar sebelum retry — lebih natural
				select {
				case <-ctx.Done():
					dlDone <- fmt.Errorf("cancelled")
					return
				case <-time.After(time.Duration(attempt+1) * time.Second):
				}
			}

			if segErr != nil {
				dlDone <- fmt.Errorf("segment %d failed after retries: %w", i, segErr)
				return
			}

			// kirim ke ffmpeg stdin
			if _, err := stdin.Write(segData); err != nil {
				dlDone <- fmt.Errorf("write segment %d to ffmpeg: %w", i, err)
				return
			}

			// jeda natural antar segment — simulasi browser buffering
			// 300-800ms, cukup untuk tidak kelihatan seperti bot
			// tapi tidak terlalu lama supaya stream tetap smooth
			jeda := time.Duration(300+rand.Intn(500)) * time.Millisecond
			select {
			case <-ctx.Done():
				dlDone <- fmt.Errorf("cancelled")
				return
			case <-time.After(jeda):
			}
		}
		dlDone <- nil
	}()

	// goroutine 2: baca output ffmpeg → masukkan ke session chunks
	readDone := make(chan error, 1)
	go func() {
		buf := make([]byte, chunkSize)
		for {
			n, readErr := io.ReadFull(stdout, buf)
			if n > 0 {
				sess.appendChunk(buf[:n])

			}
			if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
				readDone <- nil
				return
			}
			if readErr != nil {
				readDone <- readErr
				return
			}
		}
	}()

	// tunggu download selesai dulu
	dlErr := <-dlDone
	if dlErr != nil {
		cmd.Process.Kill()
		cmd.Wait()
		return dlErr
	}

	// tunggu ffmpeg selesai proses
	readErr := <-readDone
	cmd.Wait()

	if readErr != nil {
		return fmt.Errorf("read ffmpeg output: %w", readErr)
	}

	sess.mu.Lock()
	sess.done = true
	sess.mu.Unlock()
	sess.signalNew()

	return nil
}

func fetchSegmentData(ctx context.Context, client *http.Client, segURL string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", segURL, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// runYTDLP — fallback kalau direct HLS gagal
func runYTDLP(ctx context.Context, m3u8URL, toolsDir string, sess *hlsSession) error {
	ytdlpPath := "yt-dlp"
	if toolsDir != "" {
		ytdlpPath = filepath.Join(toolsDir, "yt-dlp")
	}

	parsed, _ := url.Parse(m3u8URL)
	referer := fmt.Sprintf("%s://%s/", parsed.Scheme, parsed.Host)

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
		"--user-agent", "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Mobile Safari/537.36 Edg/146.0.0.0",
		"--referer", referer,
		"--add-header", fmt.Sprintf("Origin: %s", func() string {
			p, _ := url.Parse(m3u8URL)
			origin, _ := resolveOriginReferer(p.Host)
			return origin
		}()),
		"--add-header", "Accept: */*",
		"--add-header", "Accept-Language: id,en-US;q=0.9,en;q=0.8",
		"--add-header", "sec-fetch-dest: empty",
		"--add-header", "sec-fetch-mode: cors",
		"--add-header", "sec-fetch-site: cross-site",
		"--socket-timeout", "30",
		"--retries", "10",
		"--fragment-retries", "10",
		m3u8URL,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	var stderrBuf strings.Builder
	var stderrMu sync.Mutex
	stderr, _ := cmd.StderrPipe()
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			stderrMu.Lock()
			stderrBuf.WriteString(line + "\n")
			stderrMu.Unlock()
			lower := strings.ToLower(line)
			if strings.Contains(lower, "error") ||
				strings.Contains(lower, "fatal") ||
				strings.Contains(lower, "invalid") ||
				strings.Contains(lower, "failed") {
				log.Printf("[yt-dlp] %s", line)
			}
		}
	}()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("cmd start: %w", err)
	}

	buf := make([]byte, chunkSize)
	for {
		n, readErr := io.ReadFull(stdout, buf)
		if n > 0 {
			sess.appendChunk(buf[:n])
		}
		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}
		if readErr != nil {
			cmd.Process.Kill()
			return fmt.Errorf("read stdout: %w", readErr)
		}
	}

	if err := cmd.Wait(); err != nil {
		if sess.totalChunks() == 0 {
			return fmt.Errorf("yt-dlp exit with no output: %w", err)
		}
		stderrMu.Lock()
		errOutput := stderrBuf.String()
		stderrMu.Unlock()
		isFragmentErr := strings.Contains(errOutput, "fragment") ||
			strings.Contains(errOutput, "Unable to download") ||
			strings.Contains(errOutput, "HTTP Error")
		if isFragmentErr {
			return fmt.Errorf("yt-dlp fragment error: %w", err)
		}

	}

	sess.mu.Lock()
	sess.done = true
	sess.mu.Unlock()
	sess.signalNew()

	return nil
}

// ============================================================
// Stream handler
// ============================================================

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
		h.streamProgressive(c, payload, toolsDir)
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
	rawName := downloader.ResolveFilename(payload.Title, payload.Filename, payload.Filecode, ext)
	filename := fileutil.SanitizeWithExt(rawName, ext)

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
	c.Header("Cache-Control", "no-cache")
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		c.Header("Content-Length", cl)
	}
	if cr := resp.Header.Get("Content-Range"); cr != "" {
		c.Header("Content-Range", cr)
	}
	if lm := resp.Header.Get("Last-Modified"); lm != "" {
		c.Header("Last-Modified", lm)
	}
	c.Status(resp.StatusCode)

	buf := make([]byte, 256*1024)
	done := make(chan struct{})
	go func() {
		defer close(done)
		io.CopyBuffer(c.Writer, resp.Body, buf)
	}()

	select {
	case <-c.Request.Context().Done():
		return
	case <-done:
	}
}

func (h *Handler) streamProgressive(c *gin.Context, payload *downloader.Payload, toolsDir string) {
	cacheKey := c.Query("url")

	rawName := downloader.ResolveFilename(payload.Title, payload.Filename, payload.Filecode, "mp4")
	filename := fileutil.SanitizeWithExt(rawName, "mp4")

	hlsSessionsMu.Lock()
	_, sessionExists := hlsSessions[cacheKey]
	hlsSessionsMu.Unlock()

	if !sessionExists {
		if !limiter.HLSDownload.Acquire() {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"success": false,
				"code":    "SERVER_BUSY",
				"message": fmt.Sprintf("Server busy, only %d concurrent HLS downloads allowed.", limiter.HLSDownload.Max()),
			})
			return
		}
	}

	sess := getOrCreateSession(cacheKey, payload.URL, toolsDir)

	if !sessionExists {
		// release setelah session selesai, bukan setelah handler return
		go func() {
			for {
				time.Sleep(2 * time.Second)
				d, _ := sess.isDone()
				if d {
					limiter.HLSDownload.Release()
					return
				}
			}
		}()
	}

	sess.mu.Lock()
	sess.readers++
	sess.lastRead = time.Now()
	sess.mu.Unlock()

	defer func() {
		sess.mu.Lock()
		sess.readers--
		readerCount := sess.readers
		done := sess.done
		age := time.Since(sess.lastRead)
		sess.mu.Unlock()

		if readerCount == 0 {
			if done {
				// semua reader selesai dan download done — free chunks sekarang
				// tidak perlu tunggu sessionTTL
				sess.freeChunks()
				hlsSessionsMu.Lock()
				delete(hlsSessions, cacheKey)
				hlsSessionsMu.Unlock()
				return
			}

			// download masih jalan tapi semua reader pergi
			if age < 2*time.Minute {
				// grace period — biarkan hidup kalau session masih muda
				return
			}
			log.Printf("[progressive] all readers gone after %v, cancelling: %s", age, cacheKey)
			if sess.cancel != nil {
				sess.cancel()
			}
			hlsSessionsMu.Lock()
			delete(hlsSessions, cacheKey)
			hlsSessionsMu.Unlock()
			sess.freeChunks()
		}
	}()

	// tunggu buffer awal — pakai background context supaya CDN yang lambat
	// tidak trigger cancel
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer waitCancel()

	if !sess.waitForChunk(bufferAhead-1, waitCtx) {
		done, err := sess.isDone()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "download failed"})
			return
		}
		if !done && sess.totalChunks() == 0 {
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "download timeout"})
			return
		}
	}

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("Content-Type", "video/mp4")
	c.Header("Transfer-Encoding", "chunked")
	c.Status(http.StatusOK)

	idx := 0
	ctx := c.Request.Context()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[progressive] client disconnected at chunk %d", idx)
			return
		default:
		}

		sess.mu.RLock()
		available := len(sess.chunks)
		done := sess.done
		sessErr := sess.err
		sess.mu.RUnlock()

		if idx < available {
			chunk := sess.chunks[idx]
			if _, writeErr := c.Writer.Write(chunk.data); writeErr != nil {
				log.Printf("[progressive] write error at chunk %d: %v", idx, writeErr)
				return
			}
			c.Writer.Flush()
			idx++
			continue
		}

		if done || sessErr != nil {
			if sessErr != nil {
				log.Printf("[progressive] stream ended with error: %v", sessErr)
			} else {

			}
			return
		}

		if !sess.waitForChunk(idx, ctx) {
			log.Printf("[progressive] wait timeout or client gone at chunk %d", idx)
			return
		}
	}
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
