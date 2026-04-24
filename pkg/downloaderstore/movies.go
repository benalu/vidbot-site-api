package downloaderstore

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// ─── Schema ───────────────────────────────────────────────────────────────────

func migrateMovies(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS movie_entries (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			tmdb_id     TEXT    NOT NULL UNIQUE,
			title       TEXT    NOT NULL,
			year        TEXT    NOT NULL DEFAULT '',
			duration    TEXT    NOT NULL DEFAULT '',
			rating      TEXT    NOT NULL DEFAULT '',
			genre       TEXT    NOT NULL DEFAULT '',
			poster      TEXT    NOT NULL DEFAULT '',
			backdrop    TEXT    NOT NULL DEFAULT '',
			logo        TEXT    NOT NULL DEFAULT '',
			overview    TEXT    NOT NULL DEFAULT '',
			url_1       TEXT    NOT NULL DEFAULT '',
			url_2       TEXT    NOT NULL DEFAULT '',
			url_3       TEXT    NOT NULL DEFAULT '',
			created_at  TEXT    NOT NULL
		)`,

		`CREATE VIRTUAL TABLE IF NOT EXISTS movie_fts
         USING fts5(title, genre, overview, content='movie_entries', content_rowid='id')`,

		`CREATE TRIGGER IF NOT EXISTS movie_ai AFTER INSERT ON movie_entries BEGIN
            INSERT INTO movie_fts(rowid, title, genre, overview)
            VALUES (new.id, new.title, new.genre, new.overview);
         END`,

		`CREATE TRIGGER IF NOT EXISTS movie_ad AFTER DELETE ON movie_entries BEGIN
            INSERT INTO movie_fts(movie_fts, rowid, title, genre, overview)
            VALUES ('delete', old.id, old.title, old.genre, old.overview);
         END`,

		`CREATE TRIGGER IF NOT EXISTS movie_au AFTER UPDATE ON movie_entries BEGIN
            INSERT INTO movie_fts(movie_fts, rowid, title, genre, overview)
            VALUES ('delete', old.id, old.title, old.genre, old.overview);
            INSERT INTO movie_fts(rowid, title, genre, overview)
            VALUES (new.id, new.title, new.genre, new.overview);
         END`,

		`CREATE INDEX IF NOT EXISTS idx_movie_tmdb_id ON movie_entries(tmdb_id)`,
		`CREATE INDEX IF NOT EXISTS idx_movie_year    ON movie_entries(year)`,
		`CREATE INDEX IF NOT EXISTS idx_movie_genre   ON movie_entries(genre)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("migrate movies: %w (stmt: %.60s...)", err, s)
		}
	}
	return nil
}

// ─── Types ────────────────────────────────────────────────────────────────────

type MovieEntry struct {
	ID        int64
	TmdbID    string
	Title     string
	Year      string
	Duration  string
	Rating    string
	Genre     string
	Poster    string
	Backdrop  string
	Logo      string
	Overview  string
	URL1      string
	URL2      string
	URL3      string
	CreatedAt string
}

type MovieUpsertEntry struct {
	TmdbID   string
	Title    string
	Year     string
	Duration string
	Rating   string
	Genre    string
	Poster   string
	Backdrop string
	Logo     string
	Overview string
	URL1     string
	URL2     string
	URL3     string
}

type MovieUpsertResult struct {
	ID        int64  `json:"id"`
	TmdbID    string `json:"tmdb_id"`
	Title     string `json:"title"`
	Action    string `json:"action"` // "created" | "updated"
	Duplicate bool   `json:"duplicate"`
}

type MovieMetaEntry struct {
	Title    string
	Year     string
	Duration string
	Rating   string
	Genre    string
	Poster   string
	Backdrop string
	Logo     string
	Overview string
}

type MovieLinksEntry struct {
	URL1 string
	URL2 string
	URL3 string
}

type MovieUpdateResult struct {
	ID     int64  `json:"id"`
	TmdbID string `json:"tmdb_id"`
	Title  string `json:"title"`
	Found  bool   `json:"found"`
}

type MovieGenreCount struct {
	Genre string `json:"genre"`
	Count int    `json:"count"`
}

type MovieYearCount struct {
	Year  string `json:"year"`
	Count int    `json:"count"`
}

// ─── Read ─────────────────────────────────────────────────────────────────────

func SearchMovies(keyword string, limit, offset int) ([]MovieEntry, int, error) {
	db, err := getReadDB("movies")
	if err != nil {
		return nil, 0, err
	}

	ftsQuery := sanitizeFTS(keyword) + "*"

	var total int
	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM movie_fts ff
		JOIN movie_entries m ON m.id = ff.rowid
		WHERE movie_fts MATCH ?
	`, ftsQuery).Scan(&total)
	if err != nil {
		kw := "%" + strings.ToLower(keyword) + "%"
		db.QueryRow(`
			SELECT COUNT(*) FROM movie_entries
			WHERE LOWER(title) LIKE ? OR LOWER(genre) LIKE ? OR LOWER(overview) LIKE ?
		`, kw, kw, kw).Scan(&total)
	}

	rows, err := db.Query(`
		SELECT m.id, m.tmdb_id, m.title, m.year, m.duration, m.rating,
		       m.genre, m.poster, m.backdrop, m.logo, m.overview,
		       m.url_1, m.url_2, m.url_3, m.created_at
		FROM movie_fts ff
		JOIN movie_entries m ON m.id = ff.rowid
		WHERE movie_fts MATCH ?
		ORDER BY rank
		LIMIT ? OFFSET ?
	`, ftsQuery, limit, offset)
	if err != nil {
		kw := "%" + strings.ToLower(keyword) + "%"
		rows, err = db.Query(`
			SELECT id, tmdb_id, title, year, duration, rating,
			       genre, poster, backdrop, logo, overview,
			       url_1, url_2, url_3, created_at
			FROM movie_entries
			WHERE LOWER(title) LIKE ? OR LOWER(genre) LIKE ? OR LOWER(overview) LIKE ?
			ORDER BY year DESC, title ASC
			LIMIT ? OFFSET ?
		`, kw, kw, kw, limit, offset)
		if err != nil {
			return nil, 0, err
		}
	}
	defer rows.Close()

	entries, err := scanMovieRows(rows)
	return entries, total, err
}

func GetMovieByTmdbID(tmdbID string) (*MovieEntry, error) {
	db, err := getReadDB("movies")
	if err != nil {
		return nil, err
	}
	var e MovieEntry
	err = db.QueryRow(`
		SELECT id, tmdb_id, title, year, duration, rating,
		       genre, poster, backdrop, logo, overview,
		       url_1, url_2, url_3, created_at
		FROM movie_entries WHERE tmdb_id = ?
	`, tmdbID).Scan(
		&e.ID, &e.TmdbID, &e.Title, &e.Year, &e.Duration, &e.Rating,
		&e.Genre, &e.Poster, &e.Backdrop, &e.Logo, &e.Overview,
		&e.URL1, &e.URL2, &e.URL3, &e.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func GetMovieByID(id int64) (*MovieEntry, error) {
	db, err := getReadDB("movies")
	if err != nil {
		return nil, err
	}
	var e MovieEntry
	err = db.QueryRow(`
		SELECT id, tmdb_id, title, year, duration, rating,
		       genre, poster, backdrop, logo, overview,
		       url_1, url_2, url_3, created_at
		FROM movie_entries WHERE id = ?
	`, id).Scan(
		&e.ID, &e.TmdbID, &e.Title, &e.Year, &e.Duration, &e.Rating,
		&e.Genre, &e.Poster, &e.Backdrop, &e.Logo, &e.Overview,
		&e.URL1, &e.URL2, &e.URL3, &e.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func sanitizeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

func SearchMoviesByGenre(genre string, limit, offset int) ([]MovieEntry, int, error) {
	db, err := getReadDB("movies")
	if err != nil {
		return nil, 0, err
	}
	g := sanitizeLike(strings.ToLower(strings.TrimSpace(genre)))

	var total int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM movie_entries WHERE LOWER(genre) LIKE ?`,
		"%"+g+"%",
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := db.Query(`
		SELECT id, tmdb_id, title, year, duration, rating,
		       genre, poster, backdrop, logo, overview,
		       url_1, url_2, url_3, created_at
		FROM movie_entries
		WHERE LOWER(genre) LIKE ?
		ORDER BY year DESC, title ASC
		LIMIT ? OFFSET ?
	`, "%"+g+"%", limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	entries, err := scanMovieRows(rows)
	return entries, total, err
}

func SearchMoviesByYear(year string, limit, offset int) ([]MovieEntry, int, error) {
	db, err := getReadDB("movies")
	if err != nil {
		return nil, 0, err
	}

	var total int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM movie_entries WHERE year = ?`, year,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := db.Query(`
		SELECT id, tmdb_id, title, year, duration, rating,
		       genre, poster, backdrop, logo, overview,
		       url_1, url_2, url_3, created_at
		FROM movie_entries
		WHERE year = ?
		ORDER BY title ASC
		LIMIT ? OFFSET ?
	`, year, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	entries, err := scanMovieRows(rows)
	return entries, total, err
}

func GetMovieGenres() ([]MovieGenreCount, error) {
	db, err := getReadDB("movies")
	if err != nil {
		return nil, err
	}
	// genre disimpan sebagai "Action, Drama" — kita expand per genre
	rows, err := db.Query(`
		SELECT genre FROM movie_entries WHERE genre != '' ORDER BY title ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var rawGenre string
		if err := rows.Scan(&rawGenre); err != nil {
			continue
		}
		for _, g := range strings.Split(rawGenre, ",") {
			g = strings.TrimSpace(g)
			if g != "" {
				counts[g]++
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]MovieGenreCount, 0, len(counts))
	for genre, count := range counts {
		result = append(result, MovieGenreCount{Genre: genre, Count: count})
	}
	// sort by genre name
	for i := 1; i < len(result); i++ {
		for j := i; j > 0 && result[j].Genre < result[j-1].Genre; j-- {
			result[j], result[j-1] = result[j-1], result[j]
		}
	}
	return result, nil
}

func GetMovieYears() ([]MovieYearCount, error) {
	db, err := getReadDB("movies")
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`
		SELECT year, COUNT(*) as count
		FROM movie_entries
		WHERE year != ''
		GROUP BY year
		ORDER BY year DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]MovieYearCount, 0)
	for rows.Next() {
		var y MovieYearCount
		if err := rows.Scan(&y.Year, &y.Count); err != nil {
			return nil, err
		}
		result = append(result, y)
	}
	return result, rows.Err()
}

func SearchAllMovies(limit, offset int) ([]MovieEntry, int, error) {
	db, err := getReadDB("movies")
	if err != nil {
		return nil, 0, err
	}

	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM movie_entries`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := db.Query(`
		SELECT id, tmdb_id, title, year, duration, rating,
		       genre, poster, backdrop, logo, overview,
		       url_1, url_2, url_3, created_at
		FROM movie_entries
		ORDER BY year DESC, title ASC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	entries, err := scanMovieRows(rows)
	return entries, total, err
}

// ─── Write ────────────────────────────────────────────────────────────────────

func UpsertMovie(e MovieUpsertEntry) (MovieUpsertResult, error) {
	db, err := getWriteDB("movies")
	if err != nil {
		return MovieUpsertResult{}, err
	}

	tx, err := db.Begin()
	if err != nil {
		return MovieUpsertResult{}, fmt.Errorf("upsert movie: begin tx: %w", err)
	}
	defer tx.Rollback()

	var existingID int64
	qErr := tx.QueryRow(
		`SELECT id FROM movie_entries WHERE tmdb_id = ?`, e.TmdbID,
	).Scan(&existingID)

	if qErr == sql.ErrNoRows {
		res, iErr := tx.Exec(`
			INSERT INTO movie_entries
			  (tmdb_id, title, year, duration, rating, genre, poster, backdrop, logo, overview,
			   url_1, url_2, url_3, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			e.TmdbID, e.Title, e.Year, e.Duration, e.Rating, e.Genre,
			e.Poster, e.Backdrop, e.Logo, e.Overview,
			e.URL1, e.URL2, e.URL3,
			time.Now().Format(time.RFC3339),
		)
		if iErr != nil {
			return MovieUpsertResult{}, fmt.Errorf("insert movie: %w", iErr)
		}
		newID, _ := res.LastInsertId()
		if err := tx.Commit(); err != nil {
			return MovieUpsertResult{}, fmt.Errorf("upsert movie: commit: %w", err)
		}
		return MovieUpsertResult{ID: newID, TmdbID: e.TmdbID, Title: e.Title, Action: "created"}, nil
	}
	if qErr != nil {
		return MovieUpsertResult{}, qErr
	}

	// sudah ada — update
	if err := updateMovieTx(tx, existingID, e); err != nil {
		return MovieUpsertResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return MovieUpsertResult{}, fmt.Errorf("upsert movie: commit: %w", err)
	}
	return MovieUpsertResult{ID: existingID, TmdbID: e.TmdbID, Title: e.Title, Action: "updated"}, nil
}

func BulkUpsertMovies(entries []MovieUpsertEntry) ([]MovieUpsertResult, map[int]error) {
	db, err := getWriteDB("movies")
	if err != nil {
		return nil, map[int]error{0: err}
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, map[int]error{0: fmt.Errorf("bulk movies: begin tx: %w", err)}
	}
	defer tx.Rollback()

	results := make([]MovieUpsertResult, 0, len(entries))
	errs := map[int]error{}

	for i, e := range entries {
		var existingID int64
		qErr := tx.QueryRow(
			`SELECT id FROM movie_entries WHERE tmdb_id = ?`, e.TmdbID,
		).Scan(&existingID)

		if qErr == sql.ErrNoRows {
			res, iErr := tx.Exec(`
				INSERT INTO movie_entries
				  (tmdb_id, title, year, duration, rating, genre, poster, backdrop, logo, overview,
				   url_1, url_2, url_3, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				e.TmdbID, e.Title, e.Year, e.Duration, e.Rating, e.Genre,
				e.Poster, e.Backdrop, e.Logo, e.Overview,
				e.URL1, e.URL2, e.URL3,
				time.Now().Format(time.RFC3339),
			)
			if iErr != nil {
				errs[i] = iErr
				continue
			}
			newID, _ := res.LastInsertId()
			results = append(results, MovieUpsertResult{ID: newID, TmdbID: e.TmdbID, Title: e.Title, Action: "created"})
			continue
		}
		if qErr != nil {
			errs[i] = qErr
			continue
		}

		if uErr := updateMovieTx(tx, existingID, e); uErr != nil {
			errs[i] = uErr
			continue
		}
		results = append(results, MovieUpsertResult{ID: existingID, TmdbID: e.TmdbID, Title: e.Title, Action: "updated"})
	}

	if err := tx.Commit(); err != nil {
		return nil, map[int]error{0: fmt.Errorf("bulk movies: commit: %w", err)}
	}
	return results, errs
}

func updateMovieTx(tx *sql.Tx, id int64, e MovieUpsertEntry) error {
	_, err := tx.Exec(`
		UPDATE movie_entries
		SET title=?, year=?, duration=?, rating=?, genre=?,
		    poster=?, backdrop=?, logo=?, overview=?,
		    url_1=?, url_2=?, url_3=?
		WHERE id=?`,
		e.Title, e.Year, e.Duration, e.Rating, e.Genre,
		e.Poster, e.Backdrop, e.Logo, e.Overview,
		e.URL1, e.URL2, e.URL3, id,
	)
	return err
}

func DeleteMovie(id int64) (bool, error) {
	db, err := getWriteDB("movies")
	if err != nil {
		return false, err
	}
	res, err := db.Exec(`DELETE FROM movie_entries WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func UpdateMovieMeta(id int64, e MovieMetaEntry) (MovieUpdateResult, error) {
	db, err := getWriteDB("movies")
	if err != nil {
		return MovieUpdateResult{}, err
	}

	var cur MovieEntry
	err = db.QueryRow(`
		SELECT tmdb_id, title, year, duration, rating, genre, poster, backdrop, logo, overview
		FROM movie_entries WHERE id = ?
	`, id).Scan(
		&cur.TmdbID, &cur.Title, &cur.Year, &cur.Duration, &cur.Rating,
		&cur.Genre, &cur.Poster, &cur.Backdrop, &cur.Logo, &cur.Overview,
	)
	if err == sql.ErrNoRows {
		return MovieUpdateResult{Found: false}, nil
	}
	if err != nil {
		return MovieUpdateResult{}, err
	}

	if e.Title != "" {
		cur.Title = e.Title
	}
	if e.Year != "" {
		cur.Year = e.Year
	}
	if e.Duration != "" {
		cur.Duration = e.Duration
	}
	if e.Rating != "" {
		cur.Rating = e.Rating
	}
	if e.Genre != "" {
		cur.Genre = e.Genre
	}
	if e.Poster != "" {
		cur.Poster = e.Poster
	}
	if e.Backdrop != "" {
		cur.Backdrop = e.Backdrop
	}
	if e.Logo != "" {
		cur.Logo = e.Logo
	}
	if e.Overview != "" {
		cur.Overview = e.Overview
	}

	_, err = db.Exec(`
		UPDATE movie_entries
		SET title=?, year=?, duration=?, rating=?, genre=?,
		    poster=?, backdrop=?, logo=?, overview=?
		WHERE id=?`,
		cur.Title, cur.Year, cur.Duration, cur.Rating, cur.Genre,
		cur.Poster, cur.Backdrop, cur.Logo, cur.Overview, id,
	)
	if err != nil {
		return MovieUpdateResult{}, err
	}
	return MovieUpdateResult{ID: id, TmdbID: cur.TmdbID, Title: cur.Title, Found: true}, nil
}

func UpdateMovieLinks(id int64, e MovieLinksEntry) (MovieUpdateResult, error) {
	db, err := getWriteDB("movies")
	if err != nil {
		return MovieUpdateResult{}, err
	}

	var cur struct {
		TmdbID, Title, URL1, URL2, URL3 string
	}
	err = db.QueryRow(`
		SELECT tmdb_id, title, url_1, url_2, url_3 FROM movie_entries WHERE id = ?
	`, id).Scan(&cur.TmdbID, &cur.Title, &cur.URL1, &cur.URL2, &cur.URL3)
	if err == sql.ErrNoRows {
		return MovieUpdateResult{Found: false}, nil
	}
	if err != nil {
		return MovieUpdateResult{}, err
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
		UPDATE movie_entries SET url_1=?, url_2=?, url_3=? WHERE id=?`,
		cur.URL1, cur.URL2, cur.URL3, id,
	)
	if err != nil {
		return MovieUpdateResult{}, err
	}
	return MovieUpdateResult{ID: id, TmdbID: cur.TmdbID, Title: cur.Title, Found: true}, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func scanMovieRows(rows *sql.Rows) ([]MovieEntry, error) {
	entries := make([]MovieEntry, 0)
	for rows.Next() {
		var e MovieEntry
		if err := rows.Scan(
			&e.ID, &e.TmdbID, &e.Title, &e.Year, &e.Duration, &e.Rating,
			&e.Genre, &e.Poster, &e.Backdrop, &e.Logo, &e.Overview,
			&e.URL1, &e.URL2, &e.URL3, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
