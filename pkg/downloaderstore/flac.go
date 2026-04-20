package downloaderstore

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// ─── Schema ───────────────────────────────────────────────────────────────────

func migrateFlac(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS flac_entries (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			artist     TEXT    NOT NULL,
			album      TEXT    NOT NULL,
			year       TEXT    NOT NULL DEFAULT '',
			genre      TEXT    NOT NULL DEFAULT '',
			quality    TEXT    NOT NULL DEFAULT '',
			url_1      TEXT    NOT NULL DEFAULT '',
			url_2      TEXT    NOT NULL DEFAULT '',
			url_3      TEXT    NOT NULL DEFAULT '',
			created_at TEXT    NOT NULL,
			UNIQUE(artist, album)
		)`,

		`CREATE VIRTUAL TABLE IF NOT EXISTS flac_fts
         USING fts5(artist, album, genre, content='flac_entries', content_rowid='id')`,

		`CREATE TRIGGER IF NOT EXISTS flac_ai AFTER INSERT ON flac_entries BEGIN
            INSERT INTO flac_fts(rowid, artist, album, genre)
            VALUES (new.id, new.artist, new.album, new.genre);
         END`,

		`CREATE TRIGGER IF NOT EXISTS flac_ad AFTER DELETE ON flac_entries BEGIN
            INSERT INTO flac_fts(flac_fts, rowid, artist, album, genre)
            VALUES ('delete', old.id, old.artist, old.album, old.genre);
         END`,

		`CREATE TRIGGER IF NOT EXISTS flac_au AFTER UPDATE ON flac_entries BEGIN
            INSERT INTO flac_fts(flac_fts, rowid, artist, album, genre)
            VALUES ('delete', old.id, old.artist, old.album, old.genre);
            INSERT INTO flac_fts(rowid, artist, album, genre)
            VALUES (new.id, new.artist, new.album, new.genre);
         END`,

		`CREATE INDEX IF NOT EXISTS idx_flac_artist ON flac_entries(artist)`,
		`CREATE INDEX IF NOT EXISTS idx_flac_genre  ON flac_entries(genre)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("migrate flac: %w (stmt: %.60s...)", err, s)
		}
	}
	return nil
}

// ─── Types ────────────────────────────────────────────────────────────────────

type FlacEntry struct {
	ID        int64
	Artist    string
	Album     string
	Year      string
	Genre     string
	Quality   string
	URL1      string
	URL2      string
	URL3      string
	CreatedAt string
}

type FlacUpsertEntry struct {
	Artist  string
	Album   string
	Year    string
	Genre   string
	Quality string
	URL1    string
	URL2    string
	URL3    string
}

type FlacUpsertResult struct {
	ID        int64  `json:"id"`
	Artist    string `json:"artist"`
	Album     string `json:"album"`
	Action    string `json:"action"` // "created" | "updated" | "duplicate"
	Duplicate bool   `json:"duplicate"`
}

// ─── Read ─────────────────────────────────────────────────────────────────────

func SearchFlac(keyword string) ([]FlacEntry, error) {
	db, err := getReadDB("flac")
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(keyword) == "" {
		return nil, fmt.Errorf("keyword tidak boleh kosong")
	}

	ftsQuery := sanitizeFTS(keyword) + "*"
	rows, err := db.Query(`
		SELECT f.id, f.artist, f.album, f.year, f.genre, f.quality,
		       f.url_1, f.url_2, f.url_3, f.created_at
		FROM flac_fts ff
		JOIN flac_entries f ON f.id = ff.rowid
		WHERE flac_fts MATCH ?
		ORDER BY rank
		LIMIT 50
	`, ftsQuery)

	// fallback ke LIKE kalau FTS gagal
	if err != nil {
		kw := "%" + strings.ToLower(keyword) + "%"
		rows, err = db.Query(`
			SELECT id, artist, album, year, genre, quality,
			       url_1, url_2, url_3, created_at
			FROM flac_entries
			WHERE LOWER(artist) LIKE ? OR LOWER(album) LIKE ? OR LOWER(genre) LIKE ?
			ORDER BY artist ASC LIMIT 50
		`, kw, kw, kw)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()

	return scanFlacRows(rows)
}

func SearchAllFlac(limit, offset int) ([]FlacEntry, int, error) {
	db, err := getReadDB("flac")
	if err != nil {
		return nil, 0, err
	}

	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM flac_entries`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := db.Query(`
		SELECT id, artist, album, year, genre, quality,
		       url_1, url_2, url_3, created_at
		FROM flac_entries
		ORDER BY artist ASC, album ASC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	entries, err := scanFlacRows(rows)
	return entries, total, err
}

func SearchFlacByGenre(genre string, limit, offset int) ([]FlacEntry, int, error) {
	db, err := getReadDB("flac")
	if err != nil {
		return nil, 0, err
	}

	g := strings.ToLower(strings.TrimSpace(genre))

	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM flac_entries WHERE LOWER(genre) = ?`, g).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := db.Query(`
		SELECT id, artist, album, year, genre, quality,
		       url_1, url_2, url_3, created_at
		FROM flac_entries
		WHERE LOWER(genre) = ?
		ORDER BY artist ASC, album ASC
		LIMIT ? OFFSET ?
	`, g, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	entries, err := scanFlacRows(rows)
	return entries, total, err
}

func GetFlacGenres() ([]GenreCount, error) {
	db, err := getReadDB("flac")
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`
		SELECT genre, COUNT(*) as count
		FROM flac_entries
		WHERE genre != ''
		GROUP BY genre
		ORDER BY genre ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]GenreCount, 0)
	for rows.Next() {
		var g GenreCount
		if err := rows.Scan(&g.Genre, &g.Count); err != nil {
			return nil, err
		}
		result = append(result, g)
	}
	return result, rows.Err()
}

type GenreCount struct {
	Genre string `json:"genre"`
	Count int    `json:"count"`
}

// ─── Write ────────────────────────────────────────────────────────────────────

func UpsertFlac(e FlacUpsertEntry) (FlacUpsertResult, error) {
	db, err := getWriteDB("flac")
	if err != nil {
		return FlacUpsertResult{}, err
	}

	tx, err := db.Begin()
	if err != nil {
		return FlacUpsertResult{}, fmt.Errorf("upsert flac: begin tx: %w", err)
	}
	defer tx.Rollback()

	var existingID int64
	err = tx.QueryRow(
		`SELECT id FROM flac_entries WHERE LOWER(artist) = LOWER(?) AND LOWER(album) = LOWER(?)`,
		e.Artist, e.Album,
	).Scan(&existingID)

	if err == sql.ErrNoRows {
		// insert baru
		res, err := tx.Exec(`
			INSERT INTO flac_entries (artist, album, year, genre, quality, url_1, url_2, url_3, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			e.Artist, e.Album, e.Year, e.Genre, e.Quality,
			e.URL1, e.URL2, e.URL3,
			time.Now().Format(time.RFC3339),
		)
		if err != nil {
			return FlacUpsertResult{}, fmt.Errorf("insert flac: %w", err)
		}
		newID, _ := res.LastInsertId()
		if err := tx.Commit(); err != nil {
			return FlacUpsertResult{}, fmt.Errorf("upsert flac: commit: %w", err)
		}
		return FlacUpsertResult{ID: newID, Artist: e.Artist, Album: e.Album, Action: "created"}, nil
	}
	if err != nil {
		return FlacUpsertResult{}, err
	}

	// sudah ada — update (beda dari app yang "duplicate", flac update datanya)
	_, err = tx.Exec(`
		UPDATE flac_entries
		SET year=?, genre=?, quality=?, url_1=?, url_2=?, url_3=?
		WHERE id=?`,
		e.Year, e.Genre, e.Quality, e.URL1, e.URL2, e.URL3, existingID,
	)
	if err != nil {
		return FlacUpsertResult{}, fmt.Errorf("update flac: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return FlacUpsertResult{}, fmt.Errorf("upsert flac: commit: %w", err)
	}
	return FlacUpsertResult{ID: existingID, Artist: e.Artist, Album: e.Album, Action: "updated"}, nil
}

func BulkUpsertFlac(entries []FlacUpsertEntry) ([]FlacUpsertResult, map[int]error) {
	db, err := getWriteDB("flac")
	if err != nil {
		return nil, map[int]error{0: err}
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, map[int]error{0: fmt.Errorf("bulk flac: begin tx: %w", err)}
	}
	defer tx.Rollback()

	results := make([]FlacUpsertResult, 0, len(entries))
	errs := map[int]error{}

	for i, e := range entries {
		var existingID int64
		qErr := tx.QueryRow(
			`SELECT id FROM flac_entries WHERE LOWER(artist) = LOWER(?) AND LOWER(album) = LOWER(?)`,
			e.Artist, e.Album,
		).Scan(&existingID)

		if qErr == sql.ErrNoRows {
			res, iErr := tx.Exec(`
				INSERT INTO flac_entries (artist, album, year, genre, quality, url_1, url_2, url_3, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				e.Artist, e.Album, e.Year, e.Genre, e.Quality,
				e.URL1, e.URL2, e.URL3,
				time.Now().Format(time.RFC3339),
			)
			if iErr != nil {
				errs[i] = iErr
				continue
			}
			newID, _ := res.LastInsertId()
			results = append(results, FlacUpsertResult{ID: newID, Artist: e.Artist, Album: e.Album, Action: "created"})
			continue
		}
		if qErr != nil {
			errs[i] = qErr
			continue
		}

		// update existing
		if uErr := updateFlacTx(tx, existingID, e); uErr != nil {
			errs[i] = uErr
			continue
		}
		results = append(results, FlacUpsertResult{ID: existingID, Artist: e.Artist, Album: e.Album, Action: "updated"})
	}

	if err := tx.Commit(); err != nil {
		return nil, map[int]error{0: fmt.Errorf("bulk flac: commit: %w", err)}
	}
	return results, errs
}

func updateFlacTx(tx *sql.Tx, id int64, e FlacUpsertEntry) error {
	_, err := tx.Exec(`
		UPDATE flac_entries
		SET year=?, genre=?, quality=?, url_1=?, url_2=?, url_3=?
		WHERE id=?`,
		e.Year, e.Genre, e.Quality, e.URL1, e.URL2, e.URL3, id,
	)
	return err
}

func DeleteFlac(id int64) (bool, error) {
	db, err := getWriteDB("flac")
	if err != nil {
		return false, err
	}
	res, err := db.Exec(`DELETE FROM flac_entries WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func scanFlacRows(rows *sql.Rows) ([]FlacEntry, error) {
	entries := make([]FlacEntry, 0)
	for rows.Next() {
		var e FlacEntry
		if err := rows.Scan(
			&e.ID, &e.Artist, &e.Album, &e.Year, &e.Genre, &e.Quality,
			&e.URL1, &e.URL2, &e.URL3, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
