package movies

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"vidbot-api/pkg/downloaderstore"
	"vidbot-api/pkg/response"

	"github.com/gin-gonic/gin"
)

// ─── Request types ────────────────────────────────────────────────────────────

// addRequest — bisa via TMDB ID saja (auto-fetch) atau manual + TMDB ID
type addRequest struct {
	TmdbID string `json:"id_tmdb"`
	// field manual — opsional, kalau kosong akan di-fetch dari TMDB
	Title    string `json:"title,omitempty"`
	Year     string `json:"year,omitempty"`
	Duration string `json:"duration,omitempty"`
	Rating   string `json:"rating,omitempty"`
	Genre    string `json:"genre,omitempty"`
	Poster   string `json:"poster,omitempty"`
	Backdrop string `json:"backdrop,omitempty"`
	Logo     string `json:"logo_path,omitempty"`
	Overview string `json:"overview,omitempty"`
	URL1     string `json:"url_1"`
	URL2     string `json:"url_2,omitempty"`
	URL3     string `json:"url_3,omitempty"`
}

type bulkAddRequest struct {
	Entries []addRequest `json:"entries"`
}

// ─── Admin: Add / Update ──────────────────────────────────────────────────────

func (h *Handler) AdminAdd(c *gin.Context) {
	var req addRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AdminBadRequest(c, "Request body tidak valid.")
		return
	}
	if err := validateAddRequest(req); err != nil {
		response.AdminBadRequest(c, err.Error())
		return
	}

	entry, err := h.resolveEntry(c, req)
	if err != nil {
		return // error sudah di-handle dalam resolveEntry
	}

	result, err := downloaderstore.UpsertMovie(entry)
	if err != nil {
		response.AdminDB(c, "upsert movie", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// ─── Admin: Bulk Add ──────────────────────────────────────────────────────────

func (h *Handler) AdminBulkAdd(c *gin.Context) {
	var req bulkAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AdminBadRequest(c, "Request body tidak valid.")
		return
	}
	if len(req.Entries) == 0 {
		response.AdminBadRequest(c, "Entries tidak boleh kosong.")
		return
	}
	if len(req.Entries) > 100 {
		response.AdminBadRequest(c, "Maksimum 100 entries per request.")
		return
	}

	type indexed struct {
		origIdx int
		entry   downloaderstore.MovieUpsertEntry
	}

	validEntries := make([]indexed, 0, len(req.Entries))
	validationErrs := []gin.H{}

	for i, r := range req.Entries {
		if err := validateAddRequest(r); err != nil {
			validationErrs = append(validationErrs, gin.H{
				"index": i, "error": err.Error(), "tmdb_id": r.TmdbID,
			})
			continue
		}

		entry, tmdbErr := h.resolveEntryQuiet(r)
		if tmdbErr != nil {
			validationErrs = append(validationErrs, gin.H{
				"index": i, "error": tmdbErr.Error(), "tmdb_id": r.TmdbID,
			})
			continue
		}
		validEntries = append(validEntries, indexed{origIdx: i, entry: entry})
	}

	entries := make([]downloaderstore.MovieUpsertEntry, len(validEntries))
	for i, v := range validEntries {
		entries[i] = v.entry
	}

	results, bulkErrs := downloaderstore.BulkUpsertMovies(entries)

	allErrs := validationErrs
	for sliceIdx, e := range bulkErrs {
		origIdx := validEntries[sliceIdx].origIdx
		allErrs = append(allErrs, gin.H{"index": origIdx, "error": e.Error()})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"processed": len(results),
			"failed":    len(allErrs),
			"results":   results,
			"errors":    allErrs,
		},
	})
}

// ─── Admin: Get ───────────────────────────────────────────────────────────────

func (h *Handler) AdminGet(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id < 1 {
		response.AdminBadRequest(c, "ID tidak valid.")
		return
	}

	entry, err := downloaderstore.GetMovieByID(id)
	if err != nil {
		response.AdminDB(c, "get movie", err)
		return
	}
	if entry == nil {
		response.AdminNotFound(c, "Entry tidak ditemukan.")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    formatAdminEntry(entry),
	})
}

// ─── Admin: List ──────────────────────────────────────────────────────────────

func (h *Handler) AdminList(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("q"))

	limit := 100
	offset := 0
	page := 1
	if v, _ := strconv.Atoi(c.Query("limit")); v > 0 && v <= 500 {
		limit = v
	}
	if p, _ := strconv.Atoi(c.Query("page")); p > 1 {
		page = p
		offset = (page - 1) * limit
	}

	var entries []downloaderstore.MovieEntry
	var total int
	var err error

	if keyword == "" {
		entries, total, err = downloaderstore.SearchAllMovies(limit, offset)
	} else {
		entries, total, err = downloaderstore.SearchMovies(keyword, limit, offset)
	}

	if err != nil {
		response.AdminDB(c, "list movies", err)
		return
	}

	items := make([]gin.H, 0, len(entries))
	for _, e := range entries {
		items = append(items, formatAdminEntry(&e))
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"platform": "movies",
		"total":    total,
		"page":     page,
		"limit":    limit,
		"data":     items,
	})
}

// ─── Admin: Edit meta ─────────────────────────────────────────────────────────

type editMetaRequest struct {
	Title    string `json:"title,omitempty"`
	Year     string `json:"year,omitempty"`
	Duration string `json:"duration,omitempty"`
	Rating   string `json:"rating,omitempty"`
	Genre    string `json:"genre,omitempty"`
	Poster   string `json:"poster,omitempty"`
	Backdrop string `json:"backdrop,omitempty"`
	Logo     string `json:"logo_path,omitempty"`
	Overview string `json:"overview,omitempty"`
}

func (h *Handler) AdminEdit(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id < 1 {
		response.AdminBadRequest(c, "ID tidak valid.")
		return
	}

	var req editMetaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AdminBadRequest(c, "Request body tidak valid.")
		return
	}
	if req.Title == "" && req.Year == "" && req.Duration == "" &&
		req.Rating == "" && req.Genre == "" && req.Poster == "" &&
		req.Backdrop == "" && req.Logo == "" && req.Overview == "" {
		response.AdminBadRequest(c, "Minimal satu field metadata harus diisi.")
		return
	}

	result, err := downloaderstore.UpdateMovieMeta(id, downloaderstore.MovieMetaEntry{
		Title:    strings.TrimSpace(req.Title),
		Year:     strings.TrimSpace(req.Year),
		Duration: strings.TrimSpace(req.Duration),
		Rating:   strings.TrimSpace(req.Rating),
		Genre:    strings.TrimSpace(req.Genre),
		Poster:   strings.TrimSpace(req.Poster),
		Backdrop: strings.TrimSpace(req.Backdrop),
		Logo:     strings.TrimSpace(req.Logo),
		Overview: strings.TrimSpace(req.Overview),
	})
	if err != nil {
		response.AdminDB(c, "update movie meta", err)
		return
	}
	if !result.Found {
		response.AdminNotFound(c, "Entry tidak ditemukan.")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// ─── Admin: Edit links ────────────────────────────────────────────────────────

type editLinksRequest struct {
	URL1 string `json:"url_1,omitempty"`
	URL2 string `json:"url_2,omitempty"`
	URL3 string `json:"url_3,omitempty"`
}

func (h *Handler) AdminEditLinks(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id < 1 {
		response.AdminBadRequest(c, "ID tidak valid.")
		return
	}

	var req editLinksRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AdminBadRequest(c, "Request body tidak valid.")
		return
	}
	if req.URL1 == "" && req.URL2 == "" && req.URL3 == "" {
		response.AdminBadRequest(c, "Minimal satu URL harus diisi.")
		return
	}

	result, err := downloaderstore.UpdateMovieLinks(id, downloaderstore.MovieLinksEntry{
		URL1: strings.TrimSpace(req.URL1),
		URL2: strings.TrimSpace(req.URL2),
		URL3: strings.TrimSpace(req.URL3),
	})
	if err != nil {
		response.AdminDB(c, "update movie links", err)
		return
	}
	if !result.Found {
		response.AdminNotFound(c, "Entry tidak ditemukan.")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// ─── Admin: Delete ────────────────────────────────────────────────────────────

func (h *Handler) AdminDelete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id < 1 {
		response.AdminBadRequest(c, "ID tidak valid.")
		return
	}

	deleted, err := downloaderstore.DeleteMovie(id)
	if err != nil {
		response.AdminDB(c, "delete movie", err)
		return
	}
	if !deleted {
		response.AdminNotFound(c, "Entry tidak ditemukan.")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Entry berhasil dihapus."})
}

// ─── Admin: Refresh from TMDB ─────────────────────────────────────────────────

func (h *Handler) AdminRefreshTmdb(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id < 1 {
		response.AdminBadRequest(c, "ID tidak valid.")
		return
	}

	entry, err := downloaderstore.GetMovieByID(id)
	if err != nil {
		response.AdminDB(c, "get movie", err)
		return
	}
	if entry == nil {
		response.AdminNotFound(c, "Entry tidak ditemukan.")
		return
	}

	if h.tmdbClient == nil {
		response.AdminBadRequest(c, "TMDB API key tidak dikonfigurasi.")
		return
	}

	meta, err := h.tmdbClient.GetMovieMeta(entry.TmdbID)
	if err != nil {
		response.AdminServiceError(c, "fetch tmdb", err)
		return
	}

	result, err := downloaderstore.UpdateMovieMeta(id, downloaderstore.MovieMetaEntry{
		Title:    meta.Title,
		Year:     meta.Year,
		Duration: meta.Duration,
		Rating:   meta.Rating,
		Genre:    meta.Genre,
		Poster:   meta.Poster,
		Backdrop: meta.Backdrop,
		Logo:     meta.Logo,
		Overview: meta.Overview,
	})
	if err != nil {
		response.AdminDB(c, "refresh movie meta", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Metadata '%s' berhasil diperbarui dari TMDB.", meta.Title),
		"data":    result,
	})
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// resolveEntry: fetch dari TMDB kalau field meta kosong, override dengan field manual
func (h *Handler) resolveEntry(c *gin.Context, req addRequest) (downloaderstore.MovieUpsertEntry, error) {
	entry, err := h.resolveEntryQuiet(req)
	if err != nil {
		// bedakan error TMDB vs error lain
		if strings.Contains(err.Error(), "tmdb:") {
			response.AdminServiceError(c, "fetch tmdb metadata", err)
		} else {
			response.AdminBadRequest(c, err.Error())
		}
		return downloaderstore.MovieUpsertEntry{}, err
	}
	return entry, nil
}

// resolveEntryQuiet: sama tapi tidak menulis response (untuk bulk)
func (h *Handler) resolveEntryQuiet(req addRequest) (downloaderstore.MovieUpsertEntry, error) {
	entry := downloaderstore.MovieUpsertEntry{
		TmdbID:   strings.TrimSpace(req.TmdbID),
		Title:    strings.TrimSpace(req.Title),
		Year:     strings.TrimSpace(req.Year),
		Duration: strings.TrimSpace(req.Duration),
		Rating:   strings.TrimSpace(req.Rating),
		Genre:    strings.TrimSpace(req.Genre),
		Poster:   strings.TrimSpace(req.Poster),
		Backdrop: strings.TrimSpace(req.Backdrop),
		Logo:     strings.TrimSpace(req.Logo),
		Overview: strings.TrimSpace(req.Overview),
		URL1:     strings.TrimSpace(req.URL1),
		URL2:     strings.TrimSpace(req.URL2),
		URL3:     strings.TrimSpace(req.URL3),
	}

	// kalau title kosong dan ada tmdb client, fetch dari TMDB
	needsTmdb := entry.Title == "" && h.tmdbClient != nil
	if needsTmdb {
		meta, err := h.tmdbClient.GetMovieMeta(entry.TmdbID)
		if err != nil {
			return entry, err
		}
		// isi hanya field yang masih kosong (manual override tetap dipakai)
		if entry.Title == "" {
			entry.Title = meta.Title
		}
		if entry.Year == "" {
			entry.Year = meta.Year
		}
		if entry.Duration == "" {
			entry.Duration = meta.Duration
		}
		if entry.Rating == "" {
			entry.Rating = meta.Rating
		}
		if entry.Genre == "" {
			entry.Genre = meta.Genre
		}
		if entry.Poster == "" {
			entry.Poster = meta.Poster
		}
		if entry.Backdrop == "" {
			entry.Backdrop = meta.Backdrop
		}
		if entry.Logo == "" {
			entry.Logo = meta.Logo
		}
		if entry.Overview == "" {
			entry.Overview = meta.Overview
		}
	}

	if entry.Title == "" {
		return entry, fmt.Errorf("title wajib diisi atau TMDB API key harus dikonfigurasi untuk auto-fetch")
	}

	return entry, nil
}

func validateAddRequest(req addRequest) error {
	if strings.TrimSpace(req.TmdbID) == "" {
		return fmt.Errorf("id_tmdb wajib diisi")
	}
	if strings.TrimSpace(req.URL1) == "" {
		return fmt.Errorf("url_1 wajib diisi")
	}
	return nil
}

func formatAdminEntry(e *downloaderstore.MovieEntry) gin.H {
	return gin.H{
		"id": e.ID,
		"meta": gin.H{
			"tmdb_id":  e.TmdbID,
			"title":    e.Title,
			"year":     e.Year,
			"duration": e.Duration,
			"rating":   e.Rating,
			"genre":    e.Genre,
			"poster":   e.Poster,
			"backdrop": e.Backdrop,
			"logo":     e.Logo,
			"overview": e.Overview,
		},
		"links": gin.H{
			"url_1": e.URL1,
			"url_2": e.URL2,
			"url_3": e.URL3,
		},
		"created_at": e.CreatedAt,
	}
}
