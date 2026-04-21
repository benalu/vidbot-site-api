package flac

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"vidbot-api/pkg/downloaderstore"
	"vidbot-api/pkg/httputil"
	"vidbot-api/pkg/response"
	"vidbot-api/pkg/stats"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	appURL string
}

func NewHandler(appURL string) *Handler {
	return &Handler{appURL: appURL}
}

// ─── Response types ───────────────────────────────────────────────────────────

type flacData struct {
	Artist  string `json:"artist"`
	Album   string `json:"album"`
	Year    string `json:"year,omitempty"`
	Genre   string `json:"genre,omitempty"`
	Quality string `json:"quality,omitempty"`
}

type flacDownload struct {
	URL1 string `json:"url_1"`
	URL2 string `json:"url_2,omitempty"`
	URL3 string `json:"url_3,omitempty"`
}

type flacItem struct {
	Data     flacData     `json:"data"`
	Download flacDownload `json:"download"`
}

type searchResponse struct {
	Success  bool       `json:"success"`
	Services string     `json:"services"`
	Platform string     `json:"platform"`
	Page     int        `json:"page"`
	Limit    int        `json:"limit"`
	Total    int        `json:"total"`
	Data     []flacItem `json:"data"`
}

type browseResponse struct {
	Success  bool       `json:"success"`
	Services string     `json:"services"`
	Platform string     `json:"platform"`
	Filter   string     `json:"filter"`
	Page     int        `json:"page"`
	Limit    int        `json:"limit"`
	Total    int        `json:"total"`
	Data     []flacItem `json:"data"`
}

// ─── Search ───────────────────────────────────────────────────────────────────

func (h *Handler) Search(c *gin.Context) {
	stats.Platform(c, "downloader", "flac")

	var body map[string]string
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body.")
		return
	}

	keyword := strings.TrimSpace(body["q"])
	if keyword == "" {
		keyword = strings.TrimSpace(body["artist"])
	}
	if keyword == "" {
		keyword = strings.TrimSpace(body["album"])
	}
	if keyword == "" {
		response.WriteMsg(c, response.ErrBadRequest, "Search keyword is required. Use the 'q', 'artist', or 'album' field.")
		return
	}
	if len(keyword) < 2 {
		response.WriteMsg(c, response.ErrBadRequest, "Search keyword must be at least 2 characters.")
		return
	}

	limit := 20
	page := 1
	if p, err := strconv.Atoi(body["page"]); err == nil && p > 0 {
		page = p
	}
	offset := (page - 1) * limit

	entries, total, err := downloaderstore.SearchFlacPaged(keyword, limit, offset)
	if err != nil {
		slog.Error("flac search failed", "group", "downloader", "platform", "flac", "error", err)
		stats.TrackError(c, "downloader", "flac", "DB_ERROR")
		response.DB(c, "downloader", "flac", err)
		return
	}

	items := h.buildFlacItems(entries)
	if len(items) == 0 {
		stats.TrackError(c, "downloader", "flac", "NOT_FOUND")
		response.WriteMsg(c, response.ErrNotFound, "No results found.")
		return
	}

	httputil.WriteJSONOK(c, searchResponse{
		Success:  true,
		Services: "downloader",
		Platform: "flac",
		Page:     page,
		Limit:    limit,
		Total:    total,
		Data:     items,
	})
}

// ─── Browse by genre ──────────────────────────────────────────────────────────

func (h *Handler) BrowseByGenre(c *gin.Context) {
	stats.Platform(c, "downloader", "flac")

	genre := strings.TrimSpace(c.Param("genre"))
	if genre == "" {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "Genre is required.")
		return
	}

	limit := 20
	page := 1
	if p, err := strconv.Atoi(c.Query("page")); err == nil && p > 0 {
		page = p
	}
	offset := (page - 1) * limit

	entries, total, err := downloaderstore.SearchFlacByGenre(genre, limit, offset)
	if err != nil {
		slog.Error("flac browse failed", "group", "downloader", "platform", "flac", "genre", genre, "error", err)
		stats.TrackError(c, "downloader", "flac", "DB_ERROR")
		response.DB(c, "downloader", "flac", err)
		return
	}
	if total == 0 {
		stats.TrackError(c, "downloader", "flac", "NOT_FOUND")
		response.WriteMsg(c, response.ErrNotFound,
			fmt.Sprintf("Genre '%s' not found. See /downloader/flac/genre for valid genres.", genre))
		return
	}

	items := h.buildFlacItems(entries)

	httputil.WriteJSONOK(c, browseResponse{
		Success:  true,
		Services: "downloader",
		Platform: "flac",
		Filter:   genre,
		Page:     page,
		Limit:    limit,
		Total:    total,
		Data:     items,
	})
}

// ─── Browse by artist ──────────────────────────────────────────────────────────
func (h *Handler) Artists(c *gin.Context) {
	stats.Platform(c, "downloader", "flac")

	artists, err := downloaderstore.GetFlacArtists()
	if err != nil {
		slog.Error("flac artists failed", "group", "downloader", "platform", "flac", "error", err)
		stats.TrackError(c, "downloader", "flac", "DB_ERROR")
		response.DB(c, "downloader", "flac", err)
		return
	}

	type artistsResponse struct {
		Success  bool                          `json:"success"`
		Services string                        `json:"services"`
		Platform string                        `json:"platform"`
		Total    int                           `json:"total"`
		Data     []downloaderstore.ArtistCount `json:"data"`
	}

	httputil.WriteJSONOK(c, artistsResponse{
		Success:  true,
		Services: "downloader",
		Platform: "flac",
		Total:    len(artists),
		Data:     artists,
	})
}

func (h *Handler) BrowseByArtist(c *gin.Context) {
	stats.Platform(c, "downloader", "flac")

	artist := strings.TrimSpace(c.Param("artist"))
	if artist == "" {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "Artist is required.")
		return
	}

	limit := 20
	page := 1
	if p, err := strconv.Atoi(c.Query("page")); err == nil && p > 0 {
		page = p
	}
	offset := (page - 1) * limit

	entries, total, err := downloaderstore.SearchFlacByArtist(artist, limit, offset)
	if err != nil {
		slog.Error("flac browse by artist failed", "group", "downloader", "platform", "flac", "artist", artist, "error", err)
		stats.TrackError(c, "downloader", "flac", "DB_ERROR")
		response.DB(c, "downloader", "flac", err)
		return
	}
	if total == 0 {
		stats.TrackError(c, "downloader", "flac", "NOT_FOUND")
		response.WriteMsg(c, response.ErrNotFound,
			fmt.Sprintf("Artist '%s' not found. See /downloader/flac/artist for valid artists.", artist))
		return
	}

	items := h.buildFlacItems(entries)

	httputil.WriteJSONOK(c, browseResponse{
		Success:  true,
		Services: "downloader",
		Platform: "flac",
		Filter:   artist,
		Page:     page,
		Limit:    limit,
		Total:    total,
		Data:     items,
	})
}

// ─── Genres list ──────────────────────────────────────────────────────────────

func (h *Handler) Genres(c *gin.Context) {
	stats.Platform(c, "downloader", "flac")

	genres, err := downloaderstore.GetFlacGenres()
	if err != nil {
		slog.Error("flac genres failed", "group", "downloader", "platform", "flac", "error", err)
		stats.TrackError(c, "downloader", "flac", "DB_ERROR")
		response.DB(c, "downloader", "flac", err)
		return
	}

	type genresResponse struct {
		Success  bool                         `json:"success"`
		Services string                       `json:"services"`
		Platform string                       `json:"platform"`
		Total    int                          `json:"total"`
		Data     []downloaderstore.GenreCount `json:"data"`
	}

	httputil.WriteJSONOK(c, genresResponse{
		Success:  true,
		Services: "downloader",
		Platform: "flac",
		Total:    len(genres),
		Data:     genres,
	})
}

// ─── Download redirect ────────────────────────────────────────────────────────

func (h *Handler) Download(c *gin.Context) {
	key := strings.TrimSpace(c.Query("k"))
	if key == "" {
		response.WriteMsg(c, response.ErrBadRequest, "Download key is required.")
		return
	}

	rawURL, err := downloaderstore.ResolveURL(key)
	if err != nil {
		slog.Warn("flac download shortlink not found or expired",
			"group", "downloader", "platform", "flac", "key", key, "error", err)
		response.WriteMsg(c, response.ErrNotFound, "Download link not found or has expired.")
		return
	}

	c.Redirect(http.StatusFound, rawURL)
}

// ─── Build items dengan URL masking ──────────────────────────────────────────

func (h *Handler) buildFlacItems(entries []downloaderstore.FlacEntry) []flacItem {
	base := strings.TrimRight(h.appURL, "/")

	// kumpulkan semua url_1 untuk batch masking
	var allURLs []string
	for _, e := range entries {
		if e.URL1 != "" {
			allURLs = append(allURLs, e.URL1)
		}
	}
	maskedMap := downloaderstore.MaskURLBatch(allURLs)

	items := make([]flacItem, 0, len(entries))
	for _, e := range entries {
		dl := flacDownload{}

		if e.URL1 != "" {
			maskedKey, ok := maskedMap[e.URL1]
			if !ok {
				slog.Warn("failed to mask url_1", "artist", e.Artist, "album", e.Album)
				continue
			}
			dl.URL1 = base + "/downloader/dl?k=" + maskedKey
		}
		if e.URL2 != "" {
			dl.URL2 = e.URL2
		}
		if e.URL3 != "" {
			dl.URL3 = e.URL3
		}

		items = append(items, flacItem{
			Data: flacData{
				Artist:  e.Artist,
				Album:   e.Album,
				Year:    e.Year,
				Genre:   e.Genre,
				Quality: e.Quality,
			},
			Download: dl,
		})
	}
	return items
}
