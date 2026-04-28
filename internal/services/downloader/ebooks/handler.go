package ebooks

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

type ebookData struct {
	Title     string `json:"title"`
	Author    string `json:"author"`
	Genres    string `json:"genres"`
	Publisher string `json:"publisher,omitempty"`
	Published string `json:"published,omitempty"`
	Thumbnail string `json:"thumbnail,omitempty"`
	Language  string `json:"language,omitempty"`
}

type ebookDownload struct {
	URL1 string `json:"url_1"`
	URL2 string `json:"url_2,omitempty"`
	URL3 string `json:"url_3,omitempty"`
}

type ebookItem struct {
	Data     ebookData     `json:"data"`
	Download ebookDownload `json:"download"`
}

type searchResponse struct {
	Success  bool        `json:"success"`
	Services string      `json:"services"`
	Platform string      `json:"platform"`
	Page     int         `json:"page"`
	Limit    int         `json:"limit"`
	Total    int         `json:"total"`
	Data     []ebookItem `json:"data"`
}

type browseResponse struct {
	Success  bool        `json:"success"`
	Services string      `json:"services"`
	Platform string      `json:"platform"`
	Filter   string      `json:"filter"`
	Page     int         `json:"page"`
	Limit    int         `json:"limit"`
	Total    int         `json:"total"`
	Data     []ebookItem `json:"data"`
}

// ─── Search ───────────────────────────────────────────────────────────────────

func (h *Handler) Search(c *gin.Context) {
	stats.Platform(c, "downloader", "ebooks")

	var body map[string]string
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body.")
		return
	}

	keyword := strings.TrimSpace(body["q"])
	if keyword == "" {
		keyword = strings.TrimSpace(body["title"])
	}
	if keyword == "" {
		keyword = strings.TrimSpace(body["author"])
	}
	if keyword == "" {
		response.WriteMsg(c, response.ErrBadRequest, "Search keyword is required. Use the 'q', 'title', or 'author' field.")
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

	entries, total, err := downloaderstore.SearchEbooksPaged(keyword, limit, offset)
	if err != nil {
		slog.Error("ebooks search failed", "group", "downloader", "platform", "ebooks", "error", err)
		stats.TrackError(c, "downloader", "ebooks", "DB_ERROR")
		response.DB(c, "downloader", "ebooks", err)
		return
	}

	items := h.buildEbookItems(entries)
	if len(items) == 0 {
		stats.TrackError(c, "downloader", "ebooks", "NOT_FOUND")
		response.WriteMsg(c, response.ErrNotFound, "No results found.")
		return
	}

	httputil.WriteJSONOK(c, searchResponse{
		Success:  true,
		Services: "downloader",
		Platform: "ebooks",
		Page:     page,
		Limit:    limit,
		Total:    total,
		Data:     items,
	})
}

// ─── Browse by genre ──────────────────────────────────────────────────────────

func (h *Handler) BrowseByGenre(c *gin.Context) {
	stats.Platform(c, "downloader", "ebooks")

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

	entries, total, err := downloaderstore.SearchEbooksByGenre(genre, limit, offset)
	if err != nil {
		slog.Error("ebooks browse by genre failed", "group", "downloader", "platform", "ebooks", "genre", genre, "error", err)
		stats.TrackError(c, "downloader", "ebooks", "DB_ERROR")
		response.DB(c, "downloader", "ebooks", err)
		return
	}
	if total == 0 {
		stats.TrackError(c, "downloader", "ebooks", "NOT_FOUND")
		response.WriteMsg(c, response.ErrNotFound,
			fmt.Sprintf("Genre '%s' not found. See /downloader/ebooks/genre for valid genres.", genre))
		return
	}

	items := h.buildEbookItems(entries)

	httputil.WriteJSONOK(c, browseResponse{
		Success:  true,
		Services: "downloader",
		Platform: "ebooks",
		Filter:   genre,
		Page:     page,
		Limit:    limit,
		Total:    total,
		Data:     items,
	})
}

// ─── Browse by author ─────────────────────────────────────────────────────────

func (h *Handler) BrowseByAuthor(c *gin.Context) {
	stats.Platform(c, "downloader", "ebooks")

	author := strings.TrimSpace(c.Param("author"))
	if author == "" {
		response.ErrorWithCode(c, http.StatusBadRequest, "BAD_REQUEST", "Author is required.")
		return
	}

	limit := 20
	page := 1
	if p, err := strconv.Atoi(c.Query("page")); err == nil && p > 0 {
		page = p
	}
	offset := (page - 1) * limit

	entries, total, err := downloaderstore.SearchEbooksByAuthor(author, limit, offset)
	if err != nil {
		slog.Error("ebooks browse by author failed", "group", "downloader", "platform", "ebooks", "author", author, "error", err)
		stats.TrackError(c, "downloader", "ebooks", "DB_ERROR")
		response.DB(c, "downloader", "ebooks", err)
		return
	}
	if total == 0 {
		stats.TrackError(c, "downloader", "ebooks", "NOT_FOUND")
		response.WriteMsg(c, response.ErrNotFound,
			fmt.Sprintf("Author '%s' not found. See /downloader/ebooks/author for valid authors.", author))
		return
	}

	items := h.buildEbookItems(entries)

	httputil.WriteJSONOK(c, browseResponse{
		Success:  true,
		Services: "downloader",
		Platform: "ebooks",
		Filter:   author,
		Page:     page,
		Limit:    limit,
		Total:    total,
		Data:     items,
	})
}

// ─── Genre list ───────────────────────────────────────────────────────────────

func (h *Handler) Genres(c *gin.Context) {
	stats.Platform(c, "downloader", "ebooks")

	genres, err := downloaderstore.GetEbookGenres()
	if err != nil {
		slog.Error("ebooks genres failed", "group", "downloader", "platform", "ebooks", "error", err)
		stats.TrackError(c, "downloader", "ebooks", "DB_ERROR")
		response.DB(c, "downloader", "ebooks", err)
		return
	}

	type genresResponse struct {
		Success  bool                              `json:"success"`
		Services string                            `json:"services"`
		Platform string                            `json:"platform"`
		Total    int                               `json:"total"`
		Data     []downloaderstore.EbookGenreCount `json:"data"`
	}

	httputil.WriteJSONOK(c, genresResponse{
		Success:  true,
		Services: "downloader",
		Platform: "ebooks",
		Total:    len(genres),
		Data:     genres,
	})
}

// ─── Author list ──────────────────────────────────────────────────────────────

func (h *Handler) Authors(c *gin.Context) {
	stats.Platform(c, "downloader", "ebooks")

	authors, err := downloaderstore.GetEbookAuthors()
	if err != nil {
		slog.Error("ebooks authors failed", "group", "downloader", "platform", "ebooks", "error", err)
		stats.TrackError(c, "downloader", "ebooks", "DB_ERROR")
		response.DB(c, "downloader", "ebooks", err)
		return
	}

	type authorsResponse struct {
		Success  bool                               `json:"success"`
		Services string                             `json:"services"`
		Platform string                             `json:"platform"`
		Total    int                                `json:"total"`
		Data     []downloaderstore.EbookAuthorCount `json:"data"`
	}

	httputil.WriteJSONOK(c, authorsResponse{
		Success:  true,
		Services: "downloader",
		Platform: "ebooks",
		Total:    len(authors),
		Data:     authors,
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
		slog.Warn("ebooks download shortlink not found or expired",
			"group", "downloader", "platform", "ebooks", "key", key, "error", err)
		response.WriteMsg(c, response.ErrNotFound, "Download link not found or has expired.")
		return
	}

	c.Redirect(http.StatusFound, rawURL)
}

// ─── Build items dengan URL masking ──────────────────────────────────────────

func (h *Handler) buildEbookItems(entries []downloaderstore.EbookEntry) []ebookItem {
	base := strings.TrimRight(h.appURL, "/")

	var allURLs []string
	for _, e := range entries {
		if e.URL1 != "" {
			allURLs = append(allURLs, e.URL1)
		}
	}
	maskedMap := downloaderstore.MaskURLBatch(allURLs)

	items := make([]ebookItem, 0, len(entries))
	for _, e := range entries {
		dl := ebookDownload{}

		if e.URL1 != "" {
			maskedKey, ok := maskedMap[e.URL1]
			if !ok {
				slog.Warn("failed to mask url_1", "title", e.Title, "author", e.Author)
				continue
			}
			dl.URL1 = base + "/downloader/ebooks/dl?k=" + maskedKey
		}
		if e.URL2 != "" {
			dl.URL2 = e.URL2
		}
		if e.URL3 != "" {
			dl.URL3 = e.URL3
		}

		items = append(items, ebookItem{
			Data: ebookData{
				Title:     e.Title,
				Author:    e.Author,
				Genres:    e.Genres,
				Publisher: e.Publisher,
				Published: e.Published,
				Thumbnail: e.Thumbnail,
				Language:  e.Language,
			},
			Download: dl,
		})
	}
	return items
}
