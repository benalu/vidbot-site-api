package flac

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

type addRequest struct {
	Artist  string `json:"artist"`
	Album   string `json:"album"`
	Year    string `json:"year"`
	Genre   string `json:"genre"`
	Quality string `json:"quality"`
	URL1    string `json:"url_1"`
	URL2    string `json:"url_2,omitempty"`
	URL3    string `json:"url_3,omitempty"`
}

type bulkRequest struct {
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

	result, err := downloaderstore.UpsertFlac(toUpsertEntry(req))
	if err != nil {
		response.AdminDB(c, "upsert flac", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// ─── Admin: Bulk Add ──────────────────────────────────────────────────────────

func (h *Handler) AdminBulkAdd(c *gin.Context) {
	var req bulkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AdminBadRequest(c, "Request body tidak valid.")
		return
	}
	if len(req.Entries) == 0 {
		response.AdminBadRequest(c, "Entries tidak boleh kosong.")
		return
	}
	if len(req.Entries) > 200 {
		response.AdminBadRequest(c, "Maksimum 200 entries per request.")
		return
	}

	type indexedEntry struct {
		originalIdx int
		entry       downloaderstore.FlacUpsertEntry
	}
	indexed := make([]indexedEntry, 0, len(req.Entries))
	validationErrs := []gin.H{}

	for i, e := range req.Entries {
		if err := validateAddRequest(e); err != nil {
			validationErrs = append(validationErrs, gin.H{
				"index": i, "error": err.Error(),
				"artist": e.Artist, "album": e.Album,
			})
			continue
		}
		indexed = append(indexed, indexedEntry{
			originalIdx: i,
			entry:       toUpsertEntry(e),
		})
	}

	entries := make([]downloaderstore.FlacUpsertEntry, len(indexed))
	for i, ie := range indexed {
		entries[i] = ie.entry
	}

	results, bulkErrs := downloaderstore.BulkUpsertFlac(entries)

	allErrs := validationErrs
	for sliceIdx, e := range bulkErrs {
		origIdx := indexed[sliceIdx].originalIdx
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

// ─── Admin: Delete ────────────────────────────────────────────────────────────

func (h *Handler) AdminDelete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id < 1 {
		response.AdminBadRequest(c, "ID tidak valid.")
		return
	}

	deleted, err := downloaderstore.DeleteFlac(id)
	if err != nil {
		response.AdminDB(c, "delete flac", err)
		return
	}
	if !deleted {
		response.AdminNotFound(c, "Entry tidak ditemukan.")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Entry berhasil dihapus."})
}

// ─── Admin: Edit ──────────────────────────────────────────────────────────────

type editRequest struct {
	Artist  string `json:"artist,omitempty"`
	Album   string `json:"album,omitempty"`
	Year    string `json:"year,omitempty"`
	Genre   string `json:"genre,omitempty"`
	Quality string `json:"quality,omitempty"`
	URL1    string `json:"url_1,omitempty"`
	URL2    string `json:"url_2,omitempty"`
	URL3    string `json:"url_3,omitempty"`
}

func (h *Handler) AdminEdit(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id < 1 {
		response.AdminBadRequest(c, "ID tidak valid.")
		return
	}

	var req editRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.AdminBadRequest(c, "Request body tidak valid.")
		return
	}

	// pastikan ada minimal satu field yang diisi
	if req.Artist == "" && req.Album == "" && req.Year == "" &&
		req.Genre == "" && req.Quality == "" &&
		req.URL1 == "" && req.URL2 == "" && req.URL3 == "" {
		response.AdminBadRequest(c, "Minimal satu field harus diisi.")
		return
	}

	result, err := downloaderstore.UpdateFlac(id, downloaderstore.FlacUpdateEntry{
		Artist:  strings.TrimSpace(req.Artist),
		Album:   strings.TrimSpace(req.Album),
		Year:    strings.TrimSpace(req.Year),
		Genre:   strings.TrimSpace(req.Genre),
		Quality: strings.TrimSpace(req.Quality),
		URL1:    strings.TrimSpace(req.URL1),
		URL2:    strings.TrimSpace(req.URL2),
		URL3:    strings.TrimSpace(req.URL3),
	})
	if err != nil {
		response.AdminDB(c, "update flac", err)
		return
	}
	if !result.Found {
		response.AdminNotFound(c, "Entry tidak ditemukan.")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// ─── Admin: List ──────────────────────────────────────────────────────────────

func (h *Handler) AdminList(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("q"))

	limit := 100
	offset := 0
	if v, _ := strconv.Atoi(c.Query("limit")); v > 0 && v <= 500 {
		limit = v
	}
	if page, _ := strconv.Atoi(c.Query("page")); page > 1 {
		offset = (page - 1) * limit
	}

	var entries []downloaderstore.FlacEntry
	var total int
	var err error

	if keyword == "" {
		entries, total, err = downloaderstore.SearchAllFlac(limit, offset)
	} else {
		entries, err = downloaderstore.SearchFlac(keyword)
		total = len(entries)
	}

	if err != nil {
		response.AdminDB(c, "list flac", err)
		return
	}

	type adminItem struct {
		ID        int64  `json:"id"`
		Artist    string `json:"artist"`
		Album     string `json:"album"`
		Year      string `json:"year"`
		Genre     string `json:"genre"`
		Quality   string `json:"quality"`
		URL1      string `json:"url_1"`
		URL2      string `json:"url_2,omitempty"`
		URL3      string `json:"url_3,omitempty"`
		CreatedAt string `json:"created_at"`
	}

	items := make([]adminItem, 0, len(entries))
	for _, e := range entries {
		items = append(items, adminItem{
			ID:        e.ID,
			Artist:    e.Artist,
			Album:     e.Album,
			Year:      e.Year,
			Genre:     e.Genre,
			Quality:   e.Quality,
			URL1:      e.URL1,
			URL2:      e.URL2,
			URL3:      e.URL3,
			CreatedAt: e.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"platform": "flac",
		"total":    total,
		"page":     offset/limit + 1,
		"limit":    limit,
		"data":     items,
	})
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func validateAddRequest(req addRequest) error {
	if strings.TrimSpace(req.Artist) == "" {
		return fmt.Errorf("artist wajib diisi")
	}
	if strings.TrimSpace(req.Album) == "" {
		return fmt.Errorf("album wajib diisi")
	}
	if strings.TrimSpace(req.URL1) == "" {
		return fmt.Errorf("url_1 wajib diisi")
	}
	return nil
}

func toUpsertEntry(req addRequest) downloaderstore.FlacUpsertEntry {
	return downloaderstore.FlacUpsertEntry{
		Artist:  strings.TrimSpace(req.Artist),
		Album:   strings.TrimSpace(req.Album),
		Year:    strings.TrimSpace(req.Year),
		Genre:   strings.TrimSpace(req.Genre),
		Quality: strings.TrimSpace(req.Quality),
		URL1:    strings.TrimSpace(req.URL1),
		URL2:    strings.TrimSpace(req.URL2),
		URL3:    strings.TrimSpace(req.URL3),
	}
}
