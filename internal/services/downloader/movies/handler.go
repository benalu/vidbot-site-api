package movies

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
	"vidbot-api/pkg/tmdb"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	appURL     string
	tmdbClient tmdbClientIface
}

// tmdbClientIface — supaya bisa di-mock saat testing
type tmdbClientIface interface {
	GetMovieMeta(tmdbID string) (*tmdb.MovieMeta, error)
}

func NewHandler(appURL string, tmdbClient *tmdb.Client) *Handler {
	return &Handler{appURL: appURL, tmdbClient: tmdbClient}
}

// ─── Response types ───────────────────────────────────────────────────────────

type movieMeta struct {
	TmdbID   string `json:"tmdb_id"`
	Title    string `json:"title"`
	Year     string `json:"year,omitempty"`
	Duration string `json:"duration,omitempty"`
	Rating   string `json:"rating,omitempty"`
	Genre    string `json:"genre,omitempty"`
	Poster   string `json:"poster,omitempty"`
	Backdrop string `json:"backdrop,omitempty"`
	Logo     string `json:"logo,omitempty"`
	Overview string `json:"overview,omitempty"`
}

type movieDownload struct {
	URL1 string `json:"url_1"`
	URL2 string `json:"url_2,omitempty"`
	URL3 string `json:"url_3,omitempty"`
}

type movieItem struct {
	Data     movieMeta     `json:"data"`
	Download movieDownload `json:"download"`
}

type listResponse struct {
	Success  bool        `json:"success"`
	Services string      `json:"services"`
	Platform string      `json:"platform"`
	Filter   string      `json:"filter,omitempty"`
	Page     int         `json:"page"`
	Limit    int         `json:"limit"`
	Total    int         `json:"total"`
	Data     []movieItem `json:"data"`
}

type genresResponse struct {
	Success  bool                              `json:"success"`
	Services string                            `json:"services"`
	Platform string                            `json:"platform"`
	Total    int                               `json:"total"`
	Data     []downloaderstore.MovieGenreCount `json:"data"`
}

type yearsResponse struct {
	Success  bool                             `json:"success"`
	Services string                           `json:"services"`
	Platform string                           `json:"platform"`
	Total    int                              `json:"total"`
	Data     []downloaderstore.MovieYearCount `json:"data"`
}

// ─── Search ───────────────────────────────────────────────────────────────────

func (h *Handler) Search(c *gin.Context) {
	stats.Platform(c, "downloader", "movies")

	var body map[string]string
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body.")
		return
	}

	// search by tmdb id
	if tmdbID := strings.TrimSpace(body["id"]); tmdbID != "" {
		h.searchByTmdbID(c, tmdbID)
		return
	}

	// search by keyword
	keyword := strings.TrimSpace(body["movies"])
	if keyword == "" {
		keyword = strings.TrimSpace(body["q"])
	}
	if keyword == "" {
		response.WriteMsg(c, response.ErrBadRequest,
			"Search keyword is required. Use 'movies' or 'q' for text search, or 'id' for TMDB ID lookup.")
		return
	}
	if len(keyword) < 2 {
		response.WriteMsg(c, response.ErrBadRequest, "Search keyword must be at least 2 characters.")
		return
	}

	limit, page, offset := parsePagination(body["page"], body["limit"], 20)

	entries, total, err := downloaderstore.SearchMovies(keyword, limit, offset)
	if err != nil {
		slog.Error("movies search failed", "group", "downloader", "platform", "movies", "error", err)
		stats.TrackError(c, "downloader", "movies", "DB_ERROR")
		response.DB(c, "downloader", "movies", err)
		return
	}
	if len(entries) == 0 {
		stats.TrackError(c, "downloader", "movies", "NOT_FOUND")
		response.WriteMsg(c, response.ErrNotFound, fmt.Sprintf("No movies found for '%s'.", keyword))
		return
	}

	items := h.buildMovieItems(entries)
	httputil.WriteJSONOK(c, listResponse{
		Success:  true,
		Services: "downloader",
		Platform: "movies",
		Page:     page,
		Limit:    limit,
		Total:    total,
		Data:     items,
	})
}

func (h *Handler) searchByTmdbID(c *gin.Context, tmdbID string) {
	entry, err := downloaderstore.GetMovieByTmdbID(tmdbID)
	if err != nil {
		slog.Error("movies tmdb lookup failed", "tmdb_id", tmdbID, "error", err)
		stats.TrackError(c, "downloader", "movies", "DB_ERROR")
		response.DB(c, "downloader", "movies", err)
		return
	}
	if entry == nil {
		stats.TrackError(c, "downloader", "movies", "NOT_FOUND")
		response.WriteMsg(c, response.ErrNotFound,
			fmt.Sprintf("Movie with TMDB ID '%s' not found in our library.", tmdbID))
		return
	}

	items := h.buildMovieItems([]downloaderstore.MovieEntry{*entry})
	httputil.WriteJSONOK(c, listResponse{
		Success:  true,
		Services: "downloader",
		Platform: "movies",
		Page:     1,
		Limit:    1,
		Total:    1,
		Data:     items,
	})
}

// ─── Browse by genre ──────────────────────────────────────────────────────────

func (h *Handler) BrowseByGenre(c *gin.Context) {
	stats.Platform(c, "downloader", "movies")

	genre := strings.TrimSpace(c.Param("genre"))
	if genre == "" {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "Genre is required.")
		return
	}

	limit, page, offset := parsePaginationQuery(c, 20)

	entries, total, err := downloaderstore.SearchMoviesByGenre(genre, limit, offset)
	if err != nil {
		slog.Error("movies browse by genre failed", "genre", genre, "error", err)
		stats.TrackError(c, "downloader", "movies", "DB_ERROR")
		response.DB(c, "downloader", "movies", err)
		return
	}
	if total == 0 {
		stats.TrackError(c, "downloader", "movies", "NOT_FOUND")
		response.WriteMsg(c, response.ErrNotFound,
			fmt.Sprintf("Genre '%s' not found. See /downloader/movies/genres for valid genres.", genre))
		return
	}

	items := h.buildMovieItems(entries)
	httputil.WriteJSONOK(c, listResponse{
		Success:  true,
		Services: "downloader",
		Platform: "movies",
		Filter:   genre,
		Page:     page,
		Limit:    limit,
		Total:    total,
		Data:     items,
	})
}

// ─── Browse by year ───────────────────────────────────────────────────────────

func (h *Handler) BrowseByYear(c *gin.Context) {
	stats.Platform(c, "downloader", "movies")

	year := strings.TrimSpace(c.Param("year"))
	if year == "" || len(year) != 4 {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "Year must be a 4-digit value (e.g. 2024).")
		return
	}

	limit, page, offset := parsePaginationQuery(c, 20)

	entries, total, err := downloaderstore.SearchMoviesByYear(year, limit, offset)
	if err != nil {
		slog.Error("movies browse by year failed", "year", year, "error", err)
		stats.TrackError(c, "downloader", "movies", "DB_ERROR")
		response.DB(c, "downloader", "movies", err)
		return
	}
	if total == 0 {
		stats.TrackError(c, "downloader", "movies", "NOT_FOUND")
		response.WriteMsg(c, response.ErrNotFound,
			fmt.Sprintf("No movies found for year '%s'. See /downloader/movies/years for available years.", year))
		return
	}

	items := h.buildMovieItems(entries)
	httputil.WriteJSONOK(c, listResponse{
		Success:  true,
		Services: "downloader",
		Platform: "movies",
		Filter:   year,
		Page:     page,
		Limit:    limit,
		Total:    total,
		Data:     items,
	})
}

// ─── Genres list ──────────────────────────────────────────────────────────────

func (h *Handler) Genres(c *gin.Context) {
	stats.Platform(c, "downloader", "movies")

	genres, err := downloaderstore.GetMovieGenres()
	if err != nil {
		slog.Error("movies genres failed", "error", err)
		stats.TrackError(c, "downloader", "movies", "DB_ERROR")
		response.DB(c, "downloader", "movies", err)
		return
	}

	httputil.WriteJSONOK(c, genresResponse{
		Success:  true,
		Services: "downloader",
		Platform: "movies",
		Total:    len(genres),
		Data:     genres,
	})
}

// ─── Years list ───────────────────────────────────────────────────────────────

func (h *Handler) Years(c *gin.Context) {
	stats.Platform(c, "downloader", "movies")

	years, err := downloaderstore.GetMovieYears()
	if err != nil {
		slog.Error("movies years failed", "error", err)
		stats.TrackError(c, "downloader", "movies", "DB_ERROR")
		response.DB(c, "downloader", "movies", err)
		return
	}

	httputil.WriteJSONOK(c, yearsResponse{
		Success:  true,
		Services: "downloader",
		Platform: "movies",
		Total:    len(years),
		Data:     years,
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
		slog.Warn("movies download shortlink not found", "key", key, "error", err)
		response.WriteMsg(c, response.ErrNotFound, "Download link not found or has expired.")
		return
	}

	c.Redirect(http.StatusFound, rawURL)
}

// ─── Build items dengan URL masking ──────────────────────────────────────────

func (h *Handler) buildMovieItems(entries []downloaderstore.MovieEntry) []movieItem {
	base := strings.TrimRight(h.appURL, "/")

	var allURLs []string
	for _, e := range entries {
		if e.URL1 != "" {
			allURLs = append(allURLs, e.URL1)
		}
	}
	maskedMap := downloaderstore.MaskURLBatch(allURLs)

	items := make([]movieItem, 0, len(entries))
	for _, e := range entries {
		dl := movieDownload{}

		if e.URL1 != "" {
			maskedKey, ok := maskedMap[e.URL1]
			if !ok {
				slog.Warn("failed to mask url_1", "tmdb_id", e.TmdbID, "title", e.Title)
				continue
			}
			dl.URL1 = base + "/downloader/movies/dl?k=" + maskedKey
		}
		if e.URL2 != "" {
			dl.URL2 = e.URL2
		}
		if e.URL3 != "" {
			dl.URL3 = e.URL3
		}

		items = append(items, movieItem{
			Data: movieMeta{
				TmdbID:   e.TmdbID,
				Title:    e.Title,
				Year:     e.Year,
				Duration: e.Duration,
				Rating:   e.Rating,
				Genre:    e.Genre,
				Poster:   e.Poster,
				Backdrop: e.Backdrop,
				Logo:     e.Logo,
				Overview: e.Overview,
			},
			Download: dl,
		})
	}
	return items
}

// ─── Pagination helpers ───────────────────────────────────────────────────────

func parsePagination(pageStr, limitStr string, defaultLimit int) (limit, page, offset int) {
	limit = defaultLimit
	page = 1
	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
		limit = l
	}
	offset = (page - 1) * limit
	return
}

func parsePaginationQuery(c *gin.Context, defaultLimit int) (limit, page, offset int) {
	return parsePagination(c.Query("page"), c.Query("limit"), defaultLimit)
}
