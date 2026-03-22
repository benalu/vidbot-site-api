package iptv

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"vidbot-api/pkg/iptvstore"
	"vidbot-api/pkg/mediaresponse"
	"vidbot-api/pkg/stats"

	"github.com/gin-gonic/gin"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) GetChannels(c *gin.Context) {
	stats.Group(c, "iptv")
	country := c.Query("country")
	category := c.Query("category")
	streamsOnly := c.Query("streams_only") == "true"

	if !iptvstore.Default.IsValidCountry(country) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    "INVALID_COUNTRY",
			"message": fmt.Sprintf("Country code '%s' is not valid.", country),
		})
		return
	}

	if !iptvstore.Default.IsValidCategory(category) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    "INVALID_CATEGORY",
			"message": fmt.Sprintf("Category '%s' is not valid.", category),
		})
		return
	}

	pageStr := c.Query("page")
	limitStr := c.Query("limit")
	usePagination := pageStr != "" || limitStr != ""

	channels := iptvstore.Default.GetChannels(country, category, streamsOnly)
	total := len(channels)

	var page, limit, totalPages int

	if usePagination {
		page, _ = strconv.Atoi(pageStr)
		limit, _ = strconv.Atoi(limitStr)
		if page < 1 {
			page = 1
		}
		if limit < 1 || limit > 100 {
			limit = 50
		}
		totalPages = (total + limit - 1) / limit

		start := (page - 1) * limit
		end := start + limit
		if start >= total {
			channels = []iptvstore.Channel{}
		} else {
			if end > total {
				end = total
			}
			channels = channels[start:end]
		}
	}

	data := make([]mediaresponse.IPTVChannel, 0, len(channels))
	for _, ch := range channels {
		streams := iptvstore.Default.GetStreams(ch.ID)
		iptvStreams := make([]mediaresponse.IPTVStream, 0, len(streams))
		for _, st := range streams {
			iptvStreams = append(iptvStreams, mediaresponse.IPTVStream{
				Title:     st.Title,
				URL:       st.URL,
				Format:    detectStreamFormat(st.URL),
				Quality:   st.Quality,
				UserAgent: st.UserAgent,
				Referrer:  st.Referrer,
			})
		}
		logo := ch.Logo
		if logo == "" {
			logo = iptvstore.Default.GetLogo(ch.ID)
		}
		data = append(data, mediaresponse.IPTVChannel{
			ID:         ch.ID,
			Name:       ch.Name,
			Logo:       logo,
			Country:    ch.Country,
			Categories: ch.Categories,
			Website:    ch.Website,
			Streams:    iptvStreams,
		})
	}

	if usePagination {
		c.JSON(http.StatusOK, mediaresponse.IPTVResponse{
			Success:    true,
			Services:   "iptv",
			Country:    country,
			Category:   category,
			Total:      total,
			Page:       page,
			Limit:      limit,
			TotalPages: totalPages,
			Data:       data,
		})
		return
	}

	c.JSON(http.StatusOK, mediaresponse.IPTVResponse{
		Success:  true,
		Services: "iptv",
		Country:  country,
		Category: category,
		Total:    total,
		Data:     data,
	})
}

func (h *Handler) GetCountries(c *gin.Context) {
	countries := iptvstore.Default.GetCountries()
	data := make([]mediaresponse.IPTVCountry, 0, len(countries))
	for _, ct := range countries {
		data = append(data, mediaresponse.IPTVCountry{
			Name:      ct.Name,
			Code:      ct.Code,
			Languages: ct.Languages,
			Flag:      ct.Flag,
		})
	}
	c.JSON(http.StatusOK, mediaresponse.IPTVCountriesResponse{
		Success:  true,
		Services: "iptv",
		Total:    len(data),
		Data:     data,
	})
}

func (h *Handler) GetCategories(c *gin.Context) {
	categories := iptvstore.Default.GetCategories()
	data := make([]mediaresponse.IPTVCategory, 0, len(categories))
	for _, cat := range categories {
		data = append(data, mediaresponse.IPTVCategory{
			ID:          cat.ID,
			Name:        cat.Name,
			Description: cat.Description,
		})
	}
	c.JSON(http.StatusOK, mediaresponse.IPTVCategoriesResponse{
		Success:  true,
		Services: "iptv",
		Total:    len(data),
		Data:     data,
	})
}

// GetPlaylist — endpoint untuk player seperti VLC dan Tivimate.
// Auth via query param ?key= karena player tidak bisa kirim custom header.
func (h *Handler) GetPlaylist(c *gin.Context) {
	stats.Group(c, "iptv")
	country := c.Query("country")
	category := c.Query("category")
	h.renderPlaylist(c, country, category)
}

// isHLSStream — cek apakah URL adalah HLS atau SMIL yang bisa diplay.
// MPD/DASH di-skip karena tidak semua player support.
func isHLSStream(rawURL string) bool {
	u := strings.ToLower(rawURL)
	if strings.HasSuffix(u, ".mpd") {
		return false
	}
	return strings.HasSuffix(u, ".m3u8") ||
		strings.Contains(u, "/hls/") ||
		strings.Contains(u, "chunklist") ||
		strings.Contains(u, "smil:")
}

// extractBaseURL — ambil scheme + host dari URL.
func extractBaseURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s://%s", u.Scheme, u.Host)
}

func detectStreamFormat(rawURL string) string {
	u := strings.ToLower(rawURL)
	switch {
	case strings.HasSuffix(u, ".mpd"):
		return "dash"
	case strings.Contains(u, "smil:"):
		return "smil"
	case strings.HasSuffix(u, ".m3u8"),
		strings.Contains(u, "/hls/"),
		strings.Contains(u, "chunklist"):
		return "hls"
	default:
		return "direct"
	}
}

// renderPlaylist — generate file M3U.
// SMIL URL dipakai langsung dengan header otomatis — tidak di-resolve ke chunklist
// karena chunklist berisi token dinamis yang expire dalam hitungan detik.
// VLC akan fetch SMIL sendiri dengan header yang benar sehingga token selalu fresh.
func (h *Handler) renderPlaylist(c *gin.Context, country, category string) {
	if !iptvstore.Default.IsValidCountry(country) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    "INVALID_COUNTRY",
			"message": fmt.Sprintf("Country code '%s' is not valid.", country),
		})
		return
	}

	if !iptvstore.Default.IsValidCategory(category) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"code":    "INVALID_CATEGORY",
			"message": fmt.Sprintf("Category '%s' is not valid.", category),
		})
		return
	}

	// playlist selalu streams_only=true
	channels := iptvstore.Default.GetChannels(country, category, true)

	type streamJob struct {
		ch    iptvstore.Channel
		st    iptvstore.Stream
		idx   int
		total int
		logo  string
		group string
	}

	type resolvedStream struct {
		channelID   string
		channelName string
		logo        string
		groupTitle  string
		streamURL   string
		referrer    string
		userAgent   string
	}

	// kumpulkan semua stream yang valid
	var jobs []streamJob

	for _, ch := range channels {
		streams := iptvstore.Default.GetStreams(ch.ID)

		groupTitle := "General"
		if len(ch.Categories) > 0 {
			groupTitle = strings.Title(ch.Categories[0])
		}

		logo := ch.Logo
		if logo == "" {
			logo = iptvstore.Default.GetLogo(ch.ID)
		}

		hlsStreams := []iptvstore.Stream{}
		for _, st := range streams {
			if isHLSStream(st.URL) {
				hlsStreams = append(hlsStreams, st)
			}
		}

		if len(hlsStreams) == 0 {
			continue
		}

		for i, st := range hlsStreams {
			jobs = append(jobs, streamJob{
				ch:    ch,
				st:    st,
				idx:   i,
				total: len(hlsStreams),
				logo:  logo,
				group: groupTitle,
			})
		}
	}

	// build results secara paralel — tidak ada network call,
	// goroutine hanya untuk konsistensi struktur dan future-proof
	results := make([]resolvedStream, len(jobs))
	var wg sync.WaitGroup

	for i, job := range jobs {
		wg.Add(1)
		go func(i int, job streamJob) {
			defer wg.Done()

			name := job.ch.Name
			if job.total > 1 {
				streamName := job.st.Title
				if streamName == "" {
					streamName = fmt.Sprintf("%d", job.idx+1)
				}
				name = fmt.Sprintf("%s (%s)", job.ch.Name, streamName)
			}

			results[i] = resolvedStream{
				channelID:   job.ch.ID,
				channelName: name,
				logo:        job.logo,
				groupTitle:  job.group,
				streamURL:   job.st.URL,
				referrer:    job.st.Referrer,
				userAgent:   job.st.UserAgent,
			}
		}(i, job)
	}

	wg.Wait()

	// tulis hasil ke M3U
	var sb strings.Builder
	sb.WriteString("#EXTM3U\n")

	for _, r := range results {
		sb.WriteString(fmt.Sprintf(
			"#EXTINF:-1 tvg-id=%q tvg-logo=%q group-title=%q,%s\n",
			r.channelID, r.logo, r.groupTitle, r.channelName,
		))

		// header dari data stream (sudah ada di iptv-org)
		if r.referrer != "" {
			sb.WriteString(fmt.Sprintf("#EXTVLCOPT:http-referrer=%s\n", r.referrer))
		}
		if r.userAgent != "" {
			sb.WriteString(fmt.Sprintf("#EXTVLCOPT:http-user-agent=%s\n", r.userAgent))
		}

		// kalau SMIL dan tidak ada referrer — inject otomatis dari base URL
		// supaya VLC kirim Referer yang benar saat fetch SMIL
		if strings.Contains(strings.ToLower(r.streamURL), "smil:") && r.referrer == "" {
			sb.WriteString(fmt.Sprintf(
				"#EXTVLCOPT:http-referrer=%s/\n", extractBaseURL(r.streamURL),
			))
			sb.WriteString("#EXTVLCOPT:http-user-agent=Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36\n")
		}

		sb.WriteString(r.streamURL + "\n\n")
	}

	c.Data(http.StatusOK, "application/x-mpegurl; charset=utf-8", []byte(sb.String()))
}
