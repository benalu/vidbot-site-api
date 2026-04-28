package downloaderstore

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// ─── Schema ───────────────────────────────────────────────────────────────────

func migrateEbooks(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS ebook_entries (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			title       TEXT    NOT NULL,
			author      TEXT    NOT NULL,
			genres      TEXT    NOT NULL DEFAULT '',
			publisher   TEXT    NOT NULL DEFAULT '',
			published   TEXT    NOT NULL DEFAULT '',
			thumbnail   TEXT    NOT NULL DEFAULT '',
			language    TEXT    NOT NULL DEFAULT '',
			url_1       TEXT    NOT NULL DEFAULT '',
			url_2       TEXT    NOT NULL DEFAULT '',
			url_3       TEXT    NOT NULL DEFAULT '',
			created_at  TEXT    NOT NULL,
			UNIQUE(title, author)
		)`,

		`CREATE VIRTUAL TABLE IF NOT EXISTS ebook_fts
         USING fts5(title, author, genres, publisher, content='ebook_entries', content_rowid='id')`,

		`CREATE TRIGGER IF NOT EXISTS ebook_ai AFTER INSERT ON ebook_entries BEGIN
            INSERT INTO ebook_fts(rowid, title, author, genres, publisher)
            VALUES (new.id, new.title, new.author, new.genres, new.publisher);
         END`,

		`CREATE TRIGGER IF NOT EXISTS ebook_ad AFTER DELETE ON ebook_entries BEGIN
            INSERT INTO ebook_fts(ebook_fts, rowid, title, author, genres, publisher)
            VALUES ('delete', old.id, old.title, old.author, old.genres, old.publisher);
         END`,

		`CREATE TRIGGER IF NOT EXISTS ebook_au AFTER UPDATE ON ebook_entries BEGIN
            INSERT INTO ebook_fts(ebook_fts, rowid, title, author, genres, publisher)
            VALUES ('delete', old.id, old.title, old.author, old.genres, old.publisher);
            INSERT INTO ebook_fts(rowid, title, author, genres, publisher)
            VALUES (new.id, new.title, new.author, new.genres, new.publisher);
         END`,

		`CREATE INDEX IF NOT EXISTS idx_ebook_author   ON ebook_entries(author)`,
		`CREATE INDEX IF NOT EXISTS idx_ebook_language ON ebook_entries(language)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("migrate ebooks: %w (stmt: %.60s...)", err, s)
		}
	}
	return nil
}

// ─── Types ────────────────────────────────────────────────────────────────────

type EbookEntry struct {
	ID        int64
	Title     string
	Author    string
	Genres    string
	Publisher string
	Published string
	Thumbnail string
	Language  string
	URL1      string
	URL2      string
	URL3      string
	CreatedAt string
}

type EbookUpsertEntry struct {
	Title     string
	Author    string
	Genres    string
	Publisher string
	Published string
	Thumbnail string
	Language  string
	URL1      string
	URL2      string
	URL3      string
}

type EbookUpsertResult struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Author    string `json:"author"`
	Action    string `json:"action"` // "created" | "updated"
	Duplicate bool   `json:"duplicate"`
}

type EbookMetaEntry struct {
	Title     string
	Author    string
	Genres    string
	Publisher string
	Published string
	Thumbnail string
	Language  string
}

type EbookLinksEntry struct {
	URL1 string
	URL2 string
	URL3 string
}

type EbookUpdateResult struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Author string `json:"author"`
	Found  bool   `json:"found"`
}

// ─── Read ─────────────────────────────────────────────────────────────────────

func SearchEbooks(keyword string) ([]EbookEntry, error) {
	db, err := getReadDB("ebooks")
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(keyword) == "" {
		return nil, fmt.Errorf("keyword tidak boleh kosong")
	}

	ftsQuery := sanitizeFTS(keyword) + "*"
	rows, err := db.Query(`
		SELECT e.id, e.title, e.author, e.genres, e.publisher, e.published,
		       e.thumbnail, e.language, e.url_1, e.url_2, e.url_3, e.created_at
		FROM ebook_fts ef
		JOIN ebook_entries e ON e.id = ef.rowid
		WHERE ebook_fts MATCH ?
		ORDER BY rank
		LIMIT 50
	`, ftsQuery)

	if err != nil {
		kw := "%" + strings.ToLower(keyword) + "%"
		rows, err = db.Query(`
			SELECT id, title, author, genres, publisher, published,
			       thumbnail, language, url_1, url_2, url_3, created_at
			FROM ebook_entries
			WHERE LOWER(title) LIKE ? OR LOWER(author) LIKE ? OR LOWER(genres) LIKE ?
			ORDER BY title ASC LIMIT 50
		`, kw, kw, kw)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()
	return scanEbookRows(rows)
}

func SearchEbooksPaged(keyword string, limit, offset int) ([]EbookEntry, int, error) {
	db, err := getReadDB("ebooks")
	if err != nil {
		return nil, 0, err
	}

	ftsQuery := sanitizeFTS(keyword) + "*"

	var total int
	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM ebook_fts ef
		JOIN ebook_entries e ON e.id = ef.rowid
		WHERE ebook_fts MATCH ?
	`, ftsQuery).Scan(&total)
	if err != nil {
		kw := "%" + strings.ToLower(keyword) + "%"
		db.QueryRow(`
			SELECT COUNT(*) FROM ebook_entries
			WHERE LOWER(title) LIKE ? OR LOWER(author) LIKE ? OR LOWER(genres) LIKE ?
		`, kw, kw, kw).Scan(&total)
	}

	rows, err := db.Query(`
		SELECT e.id, e.title, e.author, e.genres, e.publisher, e.published,
		       e.thumbnail, e.language, e.url_1, e.url_2, e.url_3, e.created_at
		FROM ebook_fts ef
		JOIN ebook_entries e ON e.id = ef.rowid
		WHERE ebook_fts MATCH ?
		ORDER BY rank
		LIMIT ? OFFSET ?
	`, ftsQuery, limit, offset)
	if err != nil {
		kw := "%" + strings.ToLower(keyword) + "%"
		rows, err = db.Query(`
			SELECT id, title, author, genres, publisher, published,
			       thumbnail, language, url_1, url_2, url_3, created_at
			FROM ebook_entries
			WHERE LOWER(title) LIKE ? OR LOWER(author) LIKE ? OR LOWER(genres) LIKE ?
			ORDER BY title ASC
			LIMIT ? OFFSET ?
		`, kw, kw, kw, limit, offset)
		if err != nil {
			return nil, 0, err
		}
	}
	defer rows.Close()

	entries, err := scanEbookRows(rows)
	return entries, total, err
}

func SearchAllEbooks(limit, offset int) ([]EbookEntry, int, error) {
	db, err := getReadDB("ebooks")
	if err != nil {
		return nil, 0, err
	}

	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ebook_entries`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := db.Query(`
		SELECT id, title, author, genres, publisher, published,
		       thumbnail, language, url_1, url_2, url_3, created_at
		FROM ebook_entries
		ORDER BY title ASC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	entries, err := scanEbookRows(rows)
	return entries, total, err
}

func SearchEbooksByAuthor(author string, limit, offset int) ([]EbookEntry, int, error) {
	db, err := getReadDB("ebooks")
	if err != nil {
		return nil, 0, err
	}
	a := strings.ToLower(strings.TrimSpace(author))

	var total int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM ebook_entries WHERE LOWER(author) = ?`, a,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := db.Query(`
		SELECT id, title, author, genres, publisher, published,
		       thumbnail, language, url_1, url_2, url_3, created_at
		FROM ebook_entries
		WHERE LOWER(author) = ?
		ORDER BY title ASC
		LIMIT ? OFFSET ?
	`, a, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	entries, err := scanEbookRows(rows)
	return entries, total, err
}

func SearchEbooksByGenre(genre string, limit, offset int) ([]EbookEntry, int, error) {
	db, err := getReadDB("ebooks")
	if err != nil {
		return nil, 0, err
	}
	g := strings.ToLower(strings.TrimSpace(genre))

	var total int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM ebook_entries WHERE LOWER(genres) LIKE ?`, "%"+g+"%",
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := db.Query(`
		SELECT id, title, author, genres, publisher, published,
		       thumbnail, language, url_1, url_2, url_3, created_at
		FROM ebook_entries
		WHERE LOWER(genres) LIKE ?
		ORDER BY title ASC
		LIMIT ? OFFSET ?
	`, "%"+g+"%", limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	entries, err := scanEbookRows(rows)
	return entries, total, err
}

type EbookAuthorCount struct {
	Author string `json:"author"`
	Count  int    `json:"count"`
}

func GetEbookAuthors() ([]EbookAuthorCount, error) {
	db, err := getReadDB("ebooks")
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`
		SELECT author, COUNT(*) as count
		FROM ebook_entries
		GROUP BY author
		ORDER BY author ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]EbookAuthorCount, 0)
	for rows.Next() {
		var a EbookAuthorCount
		if err := rows.Scan(&a.Author, &a.Count); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

type EbookGenreCount struct {
	Genre string `json:"genre"`
	Count int    `json:"count"`
}

func GetEbookGenres() ([]EbookGenreCount, error) {
	db, err := getReadDB("ebooks")
	if err != nil {
		return nil, err
	}
	// genres disimpan sebagai "Fiction, Fantasy" — expand per genre
	rows, err := db.Query(`SELECT genres FROM ebook_entries WHERE genres != '' ORDER BY title ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		for _, g := range strings.Split(raw, ",") {
			g = strings.TrimSpace(g)
			if g != "" {
				counts[g]++
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]EbookGenreCount, 0, len(counts))
	for genre, count := range counts {
		result = append(result, EbookGenreCount{Genre: genre, Count: count})
	}
	for i := 1; i < len(result); i++ {
		for j := i; j > 0 && result[j].Genre < result[j-1].Genre; j-- {
			result[j], result[j-1] = result[j-1], result[j]
		}
	}
	return result, nil
}

func GetEbookByID(id int64) (*EbookEntry, error) {
	db, err := getReadDB("ebooks")
	if err != nil {
		return nil, err
	}
	var e EbookEntry
	err = db.QueryRow(`
		SELECT id, title, author, genres, publisher, published,
		       thumbnail, language, url_1, url_2, url_3, created_at
		FROM ebook_entries WHERE id = ?
	`, id).Scan(
		&e.ID, &e.Title, &e.Author, &e.Genres, &e.Publisher, &e.Published,
		&e.Thumbnail, &e.Language, &e.URL1, &e.URL2, &e.URL3, &e.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// ─── Write ────────────────────────────────────────────────────────────────────

func UpsertEbook(e EbookUpsertEntry) (EbookUpsertResult, error) {
	db, err := getWriteDB("ebooks")
	if err != nil {
		return EbookUpsertResult{}, err
	}

	tx, err := db.Begin()
	if err != nil {
		return EbookUpsertResult{}, fmt.Errorf("upsert ebook: begin tx: %w", err)
	}
	defer tx.Rollback()

	var existingID int64
	err = tx.QueryRow(
		`SELECT id FROM ebook_entries WHERE LOWER(title) = LOWER(?) AND LOWER(author) = LOWER(?)`,
		e.Title, e.Author,
	).Scan(&existingID)

	if err == sql.ErrNoRows {
		res, err := tx.Exec(`
			INSERT INTO ebook_entries
			  (title, author, genres, publisher, published, thumbnail, language, url_1, url_2, url_3, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			e.Title, e.Author, e.Genres, e.Publisher, e.Published,
			e.Thumbnail, e.Language, e.URL1, e.URL2, e.URL3,
			time.Now().Format(time.RFC3339),
		)
		if err != nil {
			return EbookUpsertResult{}, fmt.Errorf("insert ebook: %w", err)
		}
		newID, _ := res.LastInsertId()
		if err := tx.Commit(); err != nil {
			return EbookUpsertResult{}, fmt.Errorf("upsert ebook: commit: %w", err)
		}
		return EbookUpsertResult{ID: newID, Title: e.Title, Author: e.Author, Action: "created"}, nil
	}
	if err != nil {
		return EbookUpsertResult{}, err
	}

	// sudah ada — update
	_, err = tx.Exec(`
		UPDATE ebook_entries
		SET genres=?, publisher=?, published=?, thumbnail=?, language=?, url_1=?, url_2=?, url_3=?
		WHERE id=?`,
		e.Genres, e.Publisher, e.Published, e.Thumbnail, e.Language,
		e.URL1, e.URL2, e.URL3, existingID,
	)
	if err != nil {
		return EbookUpsertResult{}, fmt.Errorf("update ebook: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return EbookUpsertResult{}, fmt.Errorf("upsert ebook: commit: %w", err)
	}
	return EbookUpsertResult{ID: existingID, Title: e.Title, Author: e.Author, Action: "updated"}, nil
}

func BulkUpsertEbooks(entries []EbookUpsertEntry) ([]EbookUpsertResult, map[int]error) {
	db, err := getWriteDB("ebooks")
	if err != nil {
		return nil, map[int]error{0: err}
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, map[int]error{0: fmt.Errorf("bulk ebooks: begin tx: %w", err)}
	}
	defer tx.Rollback()

	results := make([]EbookUpsertResult, 0, len(entries))
	errs := map[int]error{}

	for i, e := range entries {
		var existingID int64
		qErr := tx.QueryRow(
			`SELECT id FROM ebook_entries WHERE LOWER(title) = LOWER(?) AND LOWER(author) = LOWER(?)`,
			e.Title, e.Author,
		).Scan(&existingID)

		if qErr == sql.ErrNoRows {
			res, iErr := tx.Exec(`
				INSERT INTO ebook_entries
				  (title, author, genres, publisher, published, thumbnail, language, url_1, url_2, url_3, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				e.Title, e.Author, e.Genres, e.Publisher, e.Published,
				e.Thumbnail, e.Language, e.URL1, e.URL2, e.URL3,
				time.Now().Format(time.RFC3339),
			)
			if iErr != nil {
				errs[i] = iErr
				continue
			}
			newID, _ := res.LastInsertId()
			results = append(results, EbookUpsertResult{ID: newID, Title: e.Title, Author: e.Author, Action: "created"})
			continue
		}
		if qErr != nil {
			errs[i] = qErr
			continue
		}

		if uErr := updateEbookTx(tx, existingID, e); uErr != nil {
			errs[i] = uErr
			continue
		}
		results = append(results, EbookUpsertResult{ID: existingID, Title: e.Title, Author: e.Author, Action: "updated"})
	}

	if err := tx.Commit(); err != nil {
		return nil, map[int]error{0: fmt.Errorf("bulk ebooks: commit: %w", err)}
	}
	return results, errs
}

func updateEbookTx(tx *sql.Tx, id int64, e EbookUpsertEntry) error {
	_, err := tx.Exec(`
		UPDATE ebook_entries
		SET genres=?, publisher=?, published=?, thumbnail=?, language=?, url_1=?, url_2=?, url_3=?
		WHERE id=?`,
		e.Genres, e.Publisher, e.Published, e.Thumbnail, e.Language,
		e.URL1, e.URL2, e.URL3, id,
	)
	return err
}

func DeleteEbook(id int64) (bool, error) {
	db, err := getWriteDB("ebooks")
	if err != nil {
		return false, err
	}
	res, err := db.Exec(`DELETE FROM ebook_entries WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func UpdateEbookMeta(id int64, e EbookMetaEntry) (EbookUpdateResult, error) {
	db, err := getWriteDB("ebooks")
	if err != nil {
		return EbookUpdateResult{}, err
	}

	var cur EbookEntry
	err = db.QueryRow(`
		SELECT title, author, genres, publisher, published, thumbnail, language
		FROM ebook_entries WHERE id = ?
	`, id).Scan(&cur.Title, &cur.Author, &cur.Genres, &cur.Publisher, &cur.Published, &cur.Thumbnail, &cur.Language)
	if err == sql.ErrNoRows {
		return EbookUpdateResult{Found: false}, nil
	}
	if err != nil {
		return EbookUpdateResult{}, err
	}

	if e.Title != "" {
		cur.Title = e.Title
	}
	if e.Author != "" {
		cur.Author = e.Author
	}
	if e.Genres != "" {
		cur.Genres = e.Genres
	}
	if e.Publisher != "" {
		cur.Publisher = e.Publisher
	}
	if e.Published != "" {
		cur.Published = e.Published
	}
	if e.Thumbnail != "" {
		cur.Thumbnail = e.Thumbnail
	}
	if e.Language != "" {
		cur.Language = e.Language
	}

	_, err = db.Exec(`
		UPDATE ebook_entries
		SET title=?, author=?, genres=?, publisher=?, published=?, thumbnail=?, language=?
		WHERE id=?`,
		cur.Title, cur.Author, cur.Genres, cur.Publisher, cur.Published, cur.Thumbnail, cur.Language, id,
	)
	if err != nil {
		return EbookUpdateResult{}, err
	}
	return EbookUpdateResult{ID: id, Title: cur.Title, Author: cur.Author, Found: true}, nil
}

func UpdateEbookLinks(id int64, e EbookLinksEntry) (EbookUpdateResult, error) {
	db, err := getWriteDB("ebooks")
	if err != nil {
		return EbookUpdateResult{}, err
	}

	var cur struct{ Title, Author, URL1, URL2, URL3 string }
	err = db.QueryRow(`
		SELECT title, author, url_1, url_2, url_3 FROM ebook_entries WHERE id = ?
	`, id).Scan(&cur.Title, &cur.Author, &cur.URL1, &cur.URL2, &cur.URL3)
	if err == sql.ErrNoRows {
		return EbookUpdateResult{Found: false}, nil
	}
	if err != nil {
		return EbookUpdateResult{}, err
	}

	if e.URL1 != "" {
		cur.URL1 = e.URL1
	}
	if e.URL2 != "" {
		cur.URL2 = e.URL2
	}
	if e.URL3 != "" {
		cur.URL3 = e.URL3
	}

	_, err = db.Exec(`
		UPDATE ebook_entries SET url_1=?, url_2=?, url_3=? WHERE id=?`,
		cur.URL1, cur.URL2, cur.URL3, id,
	)
	if err != nil {
		return EbookUpdateResult{}, err
	}
	return EbookUpdateResult{ID: id, Title: cur.Title, Author: cur.Author, Found: true}, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func scanEbookRows(rows *sql.Rows) ([]EbookEntry, error) {
	entries := make([]EbookEntry, 0)
	for rows.Next() {
		var e EbookEntry
		if err := rows.Scan(
			&e.ID, &e.Title, &e.Author, &e.Genres, &e.Publisher, &e.Published,
			&e.Thumbnail, &e.Language, &e.URL1, &e.URL2, &e.URL3, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
