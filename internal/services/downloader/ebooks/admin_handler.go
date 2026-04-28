package ebooks

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
	Title     string `json:"title"`
	Author    string `json:"author"`
	Genres    string `json:"genres"`
	Publisher string `json:"publisher,omitempty"`
	Published string `json:"published,omitempty"`
	Thumbnail string `json:"thumbnail,omitempty"`
	Language  string `json:"language,omitempty"`
	URL1      string `json:"url_1"`
	URL2      string `json:"url_2,omitempty"`
	URL3      string `json:"url_3,omitempty"`
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

	result, err := downloaderstore.UpsertEbook(toUpsertEntry(req))
	if err != nil {
		response.AdminDB(c, "upsert ebook", err)
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
		entry       downloaderstore.EbookUpsertEntry
	}
	indexed := make([]indexedEntry, 0, len(req.Entries))
	validationErrs := []gin.H{}

	for i, e := range req.Entries {
		if err := validateAddRequest(e); err != nil {
			validationErrs = append(validationErrs, gin.H{
				"index": i, "error": err.Error(),
				"title": e.Title, "author": e.Author,
			})
			continue
		}
		indexed = append(indexed, indexedEntry{
			originalIdx: i,
			entry:       toUpsertEntry(e),
		})
	}

	entries := make([]downloaderstore.EbookUpsertEntry, len(indexed))
	for i, ie := range indexed {
		entries[i] = ie.entry
	}

	results, bulkErrs := downloaderstore.BulkUpsertEbooks(entries)

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

// ─── Admin: Get ───────────────────────────────────────────────────────────────

func (h *Handler) AdminGet(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id < 1 {
		response.AdminBadRequest(c, "ID tidak valid.")
		return
	}

	entry, err := downloaderstore.GetEbookByID(id)
	if err != nil {
		response.AdminDB(c, "get ebook", err)
		return
	}
	if entry == nil {
		response.AdminNotFound(c, "Entry tidak ditemukan.")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id": entry.ID,
			"meta": gin.H{
				"title": entry.Title, "author": entry.Author,
				"genres": entry.Genres, "publisher": entry.Publisher,
				"published": entry.Published, "thumbnail": entry.Thumbnail,
				"language": entry.Language,
			},
			"links": gin.H{
				"url_1": entry.URL1, "url_2": entry.URL2, "url_3": entry.URL3,
			},
			"created_at": entry.CreatedAt,
		},
	})
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

	var entries []downloaderstore.EbookEntry
	var total int
	var err error

	if keyword == "" {
		entries, total, err = downloaderstore.SearchAllEbooks(limit, offset)
	} else {
		entries, err = downloaderstore.SearchEbooks(keyword)
		total = len(entries)
	}

	if err != nil {
		response.AdminDB(c, "list ebooks", err)
		return
	}

	type adminItem struct {
		ID        int64  `json:"id"`
		Title     string `json:"title"`
		Author    string `json:"author"`
		Genres    string `json:"genres"`
		Publisher string `json:"publisher,omitempty"`
		Published string `json:"published,omitempty"`
		Thumbnail string `json:"thumbnail,omitempty"`
		Language  string `json:"language,omitempty"`
		URL1      string `json:"url_1"`
		URL2      string `json:"url_2,omitempty"`
		URL3      string `json:"url_3,omitempty"`
		CreatedAt string `json:"created_at"`
	}

	items := make([]adminItem, 0, len(entries))
	for _, e := range entries {
		items = append(items, adminItem{
			ID:        e.ID,
			Title:     e.Title,
			Author:    e.Author,
			Genres:    e.Genres,
			Publisher: e.Publisher,
			Published: e.Published,
			Thumbnail: e.Thumbnail,
			Language:  e.Language,
			URL1:      e.URL1,
			URL2:      e.URL2,
			URL3:      e.URL3,
			CreatedAt: e.CreatedAt,
		})
	}

	page := 1
	if offset > 0 {
		page = offset/limit + 1
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"platform": "ebooks",
		"total":    total,
		"page":     page,
		"limit":    limit,
		"data":     items,
	})
}

// ─── Admin: Edit Meta ─────────────────────────────────────────────────────────

type editMetaRequest struct {
	Title     string `json:"title,omitempty"`
	Author    string `json:"author,omitempty"`
	Genres    string `json:"genres,omitempty"`
	Publisher string `json:"publisher,omitempty"`
	Published string `json:"published,omitempty"`
	Thumbnail string `json:"thumbnail,omitempty"`
	Language  string `json:"language,omitempty"`
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
	if req.Title == "" && req.Author == "" && req.Genres == "" &&
		req.Publisher == "" && req.Published == "" &&
		req.Thumbnail == "" && req.Language == "" {
		response.AdminBadRequest(c, "Minimal satu field metadata harus diisi.")
		return
	}

	result, err := downloaderstore.UpdateEbookMeta(id, downloaderstore.EbookMetaEntry{
		Title:     strings.TrimSpace(req.Title),
		Author:    strings.TrimSpace(req.Author),
		Genres:    strings.TrimSpace(req.Genres),
		Publisher: strings.TrimSpace(req.Publisher),
		Published: strings.TrimSpace(req.Published),
		Thumbnail: strings.TrimSpace(req.Thumbnail),
		Language:  strings.TrimSpace(req.Language),
	})
	if err != nil {
		response.AdminDB(c, "update ebook meta", err)
		return
	}
	if !result.Found {
		response.AdminNotFound(c, "Entry tidak ditemukan.")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// ─── Admin: Edit Links ────────────────────────────────────────────────────────

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

	result, err := downloaderstore.UpdateEbookLinks(id, downloaderstore.EbookLinksEntry{
		URL1: strings.TrimSpace(req.URL1),
		URL2: strings.TrimSpace(req.URL2),
		URL3: strings.TrimSpace(req.URL3),
	})
	if err != nil {
		response.AdminDB(c, "update ebook links", err)
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

	deleted, err := downloaderstore.DeleteEbook(id)
	if err != nil {
		response.AdminDB(c, "delete ebook", err)
		return
	}
	if !deleted {
		response.AdminNotFound(c, "Entry tidak ditemukan.")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Entry berhasil dihapus."})
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func validateAddRequest(req addRequest) error {
	if strings.TrimSpace(req.Title) == "" {
		return fmt.Errorf("title wajib diisi")
	}
	if strings.TrimSpace(req.Author) == "" {
		return fmt.Errorf("author wajib diisi")
	}
	if strings.TrimSpace(req.Genres) == "" {
		return fmt.Errorf("genres wajib diisi")
	}
	if strings.TrimSpace(req.URL1) == "" {
		return fmt.Errorf("url_1 wajib diisi")
	}
	return nil
}

func toUpsertEntry(req addRequest) downloaderstore.EbookUpsertEntry {
	return downloaderstore.EbookUpsertEntry{
		Title:     strings.TrimSpace(req.Title),
		Author:    strings.TrimSpace(req.Author),
		Genres:    strings.TrimSpace(req.Genres),
		Publisher: strings.TrimSpace(req.Publisher),
		Published: strings.TrimSpace(req.Published),
		Thumbnail: strings.TrimSpace(req.Thumbnail),
		Language:  strings.TrimSpace(req.Language),
		URL1:      strings.TrimSpace(req.URL1),
		URL2:      strings.TrimSpace(req.URL2),
		URL3:      strings.TrimSpace(req.URL3),
	}
}
