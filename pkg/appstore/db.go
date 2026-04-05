package appstore

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var validPlatforms = map[string]bool{
	"android": true,
	"windows": true,
}

type platformDB struct {
	write *sql.DB
	read  *sql.DB
}

var dbs = map[string]*platformDB{}

func getWriteDB(platform string) (*sql.DB, error) {
	p, ok := dbs[platform]
	if !ok || p == nil {
		return nil, fmt.Errorf("appstore: platform '%s' not initialized", platform)
	}
	return p.write, nil
}

func getReadDB(platform string) (*sql.DB, error) {
	p, ok := dbs[platform]
	if !ok || p == nil {
		return nil, fmt.Errorf("appstore: platform '%s' not initialized", platform)
	}
	return p.read, nil
}

func IsValidPlatform(platform string) bool {
	return validPlatforms[platform]
}

// Init membuka koneksi SQLite dan membuat tabel jika belum ada.
func Init(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("appstore: mkdir: %w", err)
	}
	for platform := range validPlatforms {
		path := filepath.Join(dir, platform+".db")

		// Write connection
		writeDB, err := sql.Open("sqlite", path)
		if err != nil {
			return fmt.Errorf("appstore: open writeDB %s: %w", platform, err)
		}
		writeDB.SetMaxOpenConns(1)
		writeDB.SetMaxIdleConns(1)
		for _, pragma := range []string{
			`PRAGMA journal_mode=WAL`,
			`PRAGMA synchronous=NORMAL`,
			`PRAGMA foreign_keys=ON`,
			`PRAGMA cache_size=-32000`,
			`PRAGMA temp_store=MEMORY`,
			`PRAGMA mmap_size=268435456`,
		} {
			if _, err := writeDB.Exec(pragma); err != nil {
				return fmt.Errorf("appstore: pragma writeDB %s [%s]: %w", platform, pragma, err)
			}
		}

		// Migrate — buat tabel + index + trigger kalau belum ada
		if err := migrateDB(writeDB); err != nil {
			return fmt.Errorf("appstore: migrate %s: %w", platform, err)
		}

		// Read connection
		readDB, err := sql.Open("sqlite", path)
		if err != nil {
			return fmt.Errorf("appstore: open readDB %s: %w", platform, err)
		}
		readDB.SetMaxOpenConns(10)
		readDB.SetMaxIdleConns(5)
		readDB.SetConnMaxLifetime(30 * time.Minute)
		readDB.SetConnMaxIdleTime(5 * time.Minute)
		for _, pragma := range []string{
			`PRAGMA journal_mode=WAL`,
			`PRAGMA foreign_keys=ON`,
			`PRAGMA mmap_size=268435456`,
		} {
			if _, err := readDB.Exec(pragma); err != nil {
				return fmt.Errorf("appstore: pragma readDB %s [%s]: %w", platform, pragma, err)
			}
		}

		dbs[platform] = &platformDB{write: writeDB, read: readDB}

		var appCount, dlCount int
		readDB.QueryRow(`SELECT COUNT(*) FROM apps`).Scan(&appCount)
		readDB.QueryRow(`SELECT COUNT(*) FROM app_downloads`).Scan(&dlCount)
		wDB := writeDB
		pName := platform
		go func() {
			ticker := time.NewTicker(15 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				if _, err := wDB.Exec(`PRAGMA wal_checkpoint(PASSIVE)`); err != nil {
					log.Printf("[appstore] wal_checkpoint %s: %v", pName, err)
				}
			}
		}()
		log.Printf("[appstore] %s.db ready — %d apps, %d downloads", platform, appCount, dlCount)
	}
	return nil
}

func migrateDB(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS apps (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			slug         TEXT    NOT NULL UNIQUE,
			name         TEXT    NOT NULL UNIQUE,
			category     TEXT    NOT NULL,
			overview     TEXT    NOT NULL DEFAULT '',
			requirements TEXT    NOT NULL DEFAULT '',
			image        TEXT    NOT NULL DEFAULT '',
			created_at   TEXT    NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS app_downloads (
            id      INTEGER PRIMARY KEY AUTOINCREMENT,
            app_id  INTEGER NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
            version TEXT    NOT NULL,
			variant TEXT    NOT NULL DEFAULT '',
            raw_url TEXT    NOT NULL,
            UNIQUE(app_id, version, variant)
        )`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS apps_fts
         USING fts5(name, category, slug, content='apps', content_rowid='id')`,
		`CREATE TRIGGER IF NOT EXISTS apps_ai AFTER INSERT ON apps BEGIN
            INSERT INTO apps_fts(rowid, name, category, slug)
            VALUES (new.id, new.name, new.category, new.slug);
         END`,
		`CREATE TRIGGER IF NOT EXISTS apps_ad AFTER DELETE ON apps BEGIN
            INSERT INTO apps_fts(apps_fts, rowid, name, category, slug)
            VALUES ('delete', old.id, old.name, old.category, old.slug);
         END`,
		`CREATE TRIGGER IF NOT EXISTS apps_au AFTER UPDATE ON apps BEGIN
            INSERT INTO apps_fts(apps_fts, rowid, name, category, slug)
            VALUES ('delete', old.id, old.name, old.category, old.slug);
            INSERT INTO apps_fts(rowid, name, category, slug)
            VALUES (new.id, new.name, new.category, new.slug);
         END`,
		`CREATE INDEX IF NOT EXISTS idx_apps_category ON apps(category)`,
		`CREATE INDEX IF NOT EXISTS idx_apps_slug     ON apps(slug)`,
		`CREATE INDEX IF NOT EXISTS idx_dl_app_id     ON app_downloads(app_id)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("migrate: %w (stmt: %.60s...)", err, s)
		}
	}
	return nil
}

// ─── Types ───────────────────────────────────────────────────────────────────

type App struct {
	ID           int64
	Slug         string
	Name         string
	Category     string
	Overview     string
	Requirements string
	Image        string
	CreatedAt    string
	Downloads    []Download
}

type Download struct {
	ID      int64
	AppID   int64
	Version string
	Variant string
	RawURL  string
}

// ─── Read ─────────────────────────────────────────────────────────────────────

// Search mencari apps berdasarkan platform dan kata kunci (nama / slug / kategori).
// keyword boleh kosong → return semua platform tersebut.
func Search(platform, keyword string) ([]App, error) {
	db, err := getReadDB(platform)
	if err != nil {
		return nil, err
	}

	// keyword wajib diisi — validasi sudah di handler, ini safety net
	if strings.TrimSpace(keyword) == "" {
		return nil, fmt.Errorf("keyword tidak boleh kosong")
	}

	ftsQuery := sanitizeFTS(keyword) + "*"
	rows, err := db.Query(`
    SELECT a.id, a.slug, a.name, a.category, a.overview, a.requirements, a.image, a.created_at
    FROM apps_fts f
    JOIN apps a ON a.id = f.rowid
    WHERE apps_fts MATCH ?
    ORDER BY rank
    LIMIT 50
`, ftsQuery)
	if err != nil {
		// fallback LIKE kalau FTS gagal
		kw := "%" + strings.ToLower(keyword) + "%"
		rows, err = db.Query(`
    SELECT id, slug, name, category, overview, requirements, image, created_at
    FROM apps
    WHERE LOWER(name) LIKE ? OR LOWER(slug) LIKE ? OR LOWER(category) LIKE ?
    ORDER BY name ASC
    LIMIT 50
`, kw, kw, kw)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()

	var apps []App
	for rows.Next() {
		var a App
		if err := rows.Scan(&a.ID, &a.Slug, &a.Name, &a.Category, &a.Overview, &a.Requirements, &a.Image, &a.CreatedAt); err != nil {
			return nil, err
		}
		apps = append(apps, a)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	ids := make([]int64, len(apps))
	for i, a := range apps {
		ids[i] = a.ID
	}
	dlMap, err := batchGetDownloads(db, ids)
	if err != nil {
		return nil, err
	}
	for i := range apps {
		apps[i].Downloads = dlMap[apps[i].ID]
	}
	return apps, nil
}

// SearchAll — khusus admin, tidak butuh keyword
func SearchAll(platform string, limit, offset int) ([]App, int, error) {
	db, err := getReadDB(platform)
	if err != nil {
		return nil, 0, err
	}
	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM apps`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("searchall: count: %w", err)
	}

	rows, err := db.Query(`
        SELECT id, slug, name, category, overview, requirements, image, created_at
        FROM apps ORDER BY name ASC LIMIT ? OFFSET ?
    `, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var apps []App
	for rows.Next() {
		var a App
		if err := rows.Scan(&a.ID, &a.Slug, &a.Name, &a.Category, &a.Overview, &a.Requirements, &a.Image, &a.CreatedAt); err != nil {
			return nil, 0, err
		}
		apps = append(apps, a)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	ids := make([]int64, len(apps))
	for i, a := range apps {
		ids[i] = a.ID
	}
	dlMap, err := batchGetDownloads(db, ids)
	if err != nil {
		return nil, 0, err
	}
	for i := range apps {
		apps[i].Downloads = dlMap[apps[i].ID]
	}
	return apps, total, nil
}

func sanitizeFTS(q string) string {
	replacer := strings.NewReplacer(
		`"`, ``, `*`, ``, `(`, ``, `)`, ``,
		`^`, ``, `{`, ``, `}`, ``, `[`, ``,
		`]`, ``, `:`, ``, `+`, ``,
	)
	return strings.TrimSpace(replacer.Replace(q))
}

func batchGetDownloads(db *sql.DB, appIDs []int64) (map[int64][]Download, error) {
	if len(appIDs) == 0 {
		return map[int64][]Download{}, nil
	}
	placeholders := strings.Repeat("?,", len(appIDs))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(appIDs))
	for i, id := range appIDs {
		args[i] = id
	}
	rows, err := db.Query(
		`SELECT id, app_id, version, variant, raw_url FROM app_downloads
         WHERE app_id IN (`+placeholders+`) ORDER BY app_id, version DESC, id ASC`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[int64][]Download)
	for rows.Next() {
		var d Download
		if err := rows.Scan(&d.ID, &d.AppID, &d.Version, &d.Variant, &d.RawURL); err != nil {
			return nil, err
		}
		result[d.AppID] = append(result[d.AppID], d)
	}
	return result, rows.Err()
}

// ─── Write ────────────────────────────────────────────────────────────────────

type UpsertEntry struct {
	Name         string
	Category     string
	Overview     string
	Requirements string
	Image        string
	Version      string
	Variant      string
	RawURL       string
}

type UpsertResult struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	Action    string `json:"action"` // "created" | "version_added" | "duplicate"
	Version   string `json:"version"`
	Duplicate bool   `json:"duplicate"`
}

// Upsert menyimpan satu entry. Kalau app dengan Name yang sama sudah ada,
// hanya tambah versi baru (skip kalau versi juga sudah ada → duplicate).
func Upsert(platform string, e UpsertEntry) (UpsertResult, error) {
	db, err := getWriteDB(platform)
	if err != nil {
		return UpsertResult{}, err
	}

	tx, err := db.Begin()
	if err != nil {
		return UpsertResult{}, fmt.Errorf("upsert: begin tx: %w", err)
	}
	defer tx.Rollback() // no-op kalau sudah Commit

	baseSlug := toSlug(e.Name)
	slug := baseSlug
	for i := 2; i <= 10; i++ {
		var count int
		tx.QueryRow(`SELECT COUNT(*) FROM apps WHERE slug = ?`, slug).Scan(&count)
		if count == 0 {
			break
		}
		if i == 10 {
			// pakai timestamp sebagai last resort, collision probability ~0
			slug = fmt.Sprintf("%s-%d", baseSlug, time.Now().UnixMilli())
			break
		}
		slug = fmt.Sprintf("%s-%d", baseSlug, i)
	}

	var appID int64
	var existingSlug string
	err = tx.QueryRow(
		`SELECT id, slug FROM apps WHERE LOWER(name) = LOWER(?)`, e.Name,
	).Scan(&appID, &existingSlug)

	if err == sql.ErrNoRows {
		res, err := tx.Exec(
			`INSERT INTO apps (slug, name, category, overview, requirements, image, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			slug, e.Name, normalizeCategory(e.Category),
			e.Overview, e.Requirements, e.Image, time.Now().Format(time.RFC3339),
		)
		if err != nil {
			return UpsertResult{}, fmt.Errorf("insert app: %w", err)
		}
		appID, _ = res.LastInsertId()
		existingSlug = slug
		if err := insertVersionTx(tx, appID, e.Version, e.Variant, e.RawURL); err != nil {
			return UpsertResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return UpsertResult{}, fmt.Errorf("upsert: commit: %w", err)
		}
		return UpsertResult{Slug: existingSlug, Name: e.Name, Action: "created", Version: e.Version}, nil
	}
	if err != nil {
		return UpsertResult{}, err
	}

	var versionExists int
	tx.QueryRow(
		`SELECT COUNT(*) FROM app_downloads WHERE app_id = ? AND LOWER(version) = LOWER(?) AND LOWER(variant) = LOWER(?)`,
		appID, e.Version, e.Variant,
	).Scan(&versionExists)

	if versionExists > 0 {
		// tidak ada write, rollback otomatis dari defer
		return UpsertResult{Slug: existingSlug, Name: e.Name, Action: "duplicate", Version: e.Version, Duplicate: true}, nil
	}

	if err := insertVersionTx(tx, appID, e.Version, e.Variant, e.RawURL); err != nil {
		return UpsertResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return UpsertResult{}, fmt.Errorf("upsert: commit: %w", err)
	}
	return UpsertResult{Slug: existingSlug, Name: e.Name, Action: "version_added", Version: e.Version}, nil
}

func BulkUpsert(platform string, entries []UpsertEntry) ([]UpsertResult, map[int]error) {
	db, err := getWriteDB(platform)
	if err != nil {
		return nil, map[int]error{0: err}
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, map[int]error{0: fmt.Errorf("bulk: begin tx: %w", err)}
	}
	defer tx.Rollback()

	results := make([]UpsertResult, 0, len(entries))
	errs := map[int]error{}

	for i, e := range entries {
		baseSlug := toSlug(e.Name)
		slug := baseSlug
		for attempt := 2; attempt <= 10; attempt++ {
			var count int
			tx.QueryRow(`SELECT COUNT(*) FROM apps WHERE slug = ?`, slug).Scan(&count)
			if count == 0 {
				break
			}
			if attempt == 10 {
				slug = fmt.Sprintf("%s-%d", baseSlug, time.Now().UnixMilli())
				break
			}
			slug = fmt.Sprintf("%s-%d", baseSlug, attempt)
		}
		var appID int64
		var existingSlug string
		qErr := tx.QueryRow(
			`SELECT id, slug FROM apps WHERE LOWER(name) = LOWER(?)`, e.Name,
		).Scan(&appID, &existingSlug)

		if qErr == sql.ErrNoRows {
			res, iErr := tx.Exec(
				`INSERT INTO apps (slug, name, category, overview, requirements, image, created_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?)`,
				slug, e.Name, normalizeCategory(e.Category),
				e.Overview, e.Requirements, e.Image, time.Now().Format(time.RFC3339),
			)
			if iErr != nil {
				errs[i] = iErr
				continue
			}
			appID, _ = res.LastInsertId()
			existingSlug = slug
			if vErr := insertVersionTx(tx, appID, e.Version, e.Variant, e.RawURL); vErr != nil {
				errs[i] = vErr
				continue
			}
			results = append(results, UpsertResult{Slug: existingSlug, Name: e.Name, Action: "created", Version: e.Version})
			continue
		}
		if qErr != nil {
			errs[i] = qErr
			continue
		}

		var versionExists int
		tx.QueryRow(
			`SELECT COUNT(*) FROM app_downloads WHERE app_id = ? AND LOWER(version) = LOWER(?) AND LOWER(variant) = LOWER(?)`,
			appID, e.Version, e.Variant,
		).Scan(&versionExists)
		if versionExists > 0 {
			results = append(results, UpsertResult{Slug: existingSlug, Name: e.Name, Action: "duplicate", Version: e.Version, Duplicate: true})
			continue
		}
		if vErr := insertVersionTx(tx, appID, e.Version, e.Variant, e.RawURL); vErr != nil {
			errs[i] = vErr
			continue
		}
		results = append(results, UpsertResult{Slug: existingSlug, Name: e.Name, Action: "version_added", Version: e.Version})
	}

	if err := tx.Commit(); err != nil {
		return nil, map[int]error{0: fmt.Errorf("bulk: commit: %w", err)}
	}
	return results, errs
}

func insertVersionTx(tx *sql.Tx, appID int64, version, variant, rawURL string) error {
	_, err := tx.Exec(
		`INSERT INTO app_downloads (app_id, version, variant, raw_url) VALUES (?, ?, ?, ?)`,
		appID, version, variant, rawURL,
	)
	return err
}

// Delete menghapus app beserta semua download-nya.
func Delete(platform, slug string) (bool, error) {
	db, err := getWriteDB(platform)
	if err != nil {
		return false, err
	}
	res, err := db.Exec(`DELETE FROM apps WHERE slug = ?`, slug)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func DeleteVersion(platform string, versionID int64) (bool, error) {
	db, err := getWriteDB(platform)
	if err != nil {
		return false, err
	}
	res, err := db.Exec(`DELETE FROM app_downloads WHERE id = ?`, versionID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// toSlug mengubah nama app menjadi URL-friendly slug.
// "Classical Music Radio" → "classical-music-radio"
func toSlug(name string) string {
	// ambil bagian sebelum versi kalau ada pola "Name vX.Y.Z ..."
	// contoh: "Classical Music Radio v6.2.0 GP [Pro]" → "Classical Music Radio"
	lower := strings.ToLower(name)
	var words []string
	for _, w := range strings.Fields(lower) {
		// berhenti kalau mulai dari pola versi
		if len(w) > 1 && w[0] == 'v' && w[1] >= '0' && w[1] <= '9' {
			break
		}
		// buang karakter non-alfanumerik kecuali ampersand
		clean := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				return r
			}
			return -1
		}, w)
		if clean != "" {
			words = append(words, clean)
		}
	}
	if len(words) == 0 {
		return strings.ReplaceAll(lower, " ", "-")
	}
	return strings.Join(words, "-")
}

// normalizeCategory: "music and audio" → "music-and-audio"
func normalizeCategory(cat string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(cat)), " ", "-")
}
