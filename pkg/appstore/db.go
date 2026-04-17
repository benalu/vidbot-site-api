package appstore

import (
	"database/sql"
	"fmt"
	"log/slog"
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

func Init(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("appstore: mkdir: %w", err)
	}
	for platform := range validPlatforms {
		path := filepath.Join(dir, platform+".db")

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

		if err := migrateDB(writeDB); err != nil {
			return fmt.Errorf("appstore: migrate %s: %w", platform, err)
		}

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

		var appCount int
		readDB.QueryRow(`SELECT COUNT(*) FROM apps`).Scan(&appCount)
		wDB := writeDB
		pName := platform
		go func() {
			ticker := time.NewTicker(15 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				if _, err := wDB.Exec(`PRAGMA wal_checkpoint(PASSIVE)`); err != nil {
					slog.Warn("wal checkpoint failed", "platform", pName, "error", err)
				}
			}
		}()
		slog.Info("appstore db ready", "platform", platform, "apps", appCount)
	}
	return nil
}

func migrateDB(db *sql.DB) error {
	stmts := []string{
		// ─── Apps table — metadata only, no URL ──────────────────────────────
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
		// ─── Versions table — hanya version string, tidak ada URL ─────────────
		// version: string versi (misal "4.3.1")
		// cdn_query: keyword yang dipakai saat search ke CDN (default = nama app)
		//            berguna kalau nama file di CDN berbeda dari nama app
		`CREATE TABLE IF NOT EXISTS app_versions (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			app_id     INTEGER NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
			version    TEXT    NOT NULL,
			cdn_query  TEXT    NOT NULL DEFAULT '',
			created_at TEXT    NOT NULL,
			UNIQUE(app_id, version)
		)`,
		// ─── FTS ─────────────────────────────────────────────────────────────
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
		`CREATE INDEX IF NOT EXISTS idx_apps_category   ON apps(category)`,
		`CREATE INDEX IF NOT EXISTS idx_apps_slug        ON apps(slug)`,
		`CREATE INDEX IF NOT EXISTS idx_appver_app_id    ON app_versions(app_id)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("migrate: %w (stmt: %.60s...)", err, s)
		}
	}
	return nil
}

// ─── Types ────────────────────────────────────────────────────────────────────

type App struct {
	ID           int64
	Slug         string
	Name         string
	Category     string
	Overview     string
	Requirements string
	Image        string
	CreatedAt    string
	Versions     []AppVersion
}

// AppVersion — hanya metadata versi, URL datang dari CDN resolver
type AppVersion struct {
	ID        int64
	AppID     int64
	Version   string
	CDNQuery  string // override keyword pencarian CDN, kosong = pakai nama app
	CreatedAt string
}

// ─── Read ─────────────────────────────────────────────────────────────────────

func Search(platform, keyword string) ([]App, error) {
	db, err := getReadDB(platform)
	if err != nil {
		return nil, err
	}
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
		kw := "%" + strings.ToLower(keyword) + "%"
		rows, err = db.Query(`
			SELECT id, slug, name, category, overview, requirements, image, created_at
			FROM apps
			WHERE LOWER(name) LIKE ? OR LOWER(slug) LIKE ? OR LOWER(category) LIKE ?
			ORDER BY name ASC LIMIT 50
		`, kw, kw, kw)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()

	apps := make([]App, 0)
	for rows.Next() {
		var a App
		if err := rows.Scan(&a.ID, &a.Slug, &a.Name, &a.Category,
			&a.Overview, &a.Requirements, &a.Image, &a.CreatedAt); err != nil {
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
	versionMap, err := batchGetVersions(db, ids)
	if err != nil {
		return nil, err
	}
	for i := range apps {
		apps[i].Versions = versionMap[apps[i].ID]
	}
	return apps, nil
}

func SearchAll(platform string, limit, offset int) ([]App, int, error) {
	db, err := getReadDB(platform)
	if err != nil {
		return nil, 0, err
	}
	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM apps`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := db.Query(`
		SELECT id, slug, name, category, overview, requirements, image, created_at
		FROM apps ORDER BY name ASC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	apps := make([]App, 0)
	for rows.Next() {
		var a App
		if err := rows.Scan(&a.ID, &a.Slug, &a.Name, &a.Category,
			&a.Overview, &a.Requirements, &a.Image, &a.CreatedAt); err != nil {
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
	versionMap, err := batchGetVersions(db, ids)
	if err != nil {
		return nil, 0, err
	}
	for i := range apps {
		apps[i].Versions = versionMap[apps[i].ID]
	}
	return apps, total, nil
}

func SearchByCategory(platform, category string, limit, offset int) ([]App, int, error) {
	db, err := getReadDB(platform)
	if err != nil {
		return nil, 0, err
	}
	cat := normalizeCategory(category)

	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM apps WHERE category = ?`, cat).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := db.Query(`
		SELECT id, slug, name, category, overview, requirements, image, created_at
		FROM apps WHERE category = ?
		ORDER BY name ASC LIMIT ? OFFSET ?
	`, cat, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	apps := make([]App, 0)
	for rows.Next() {
		var a App
		if err := rows.Scan(&a.ID, &a.Slug, &a.Name, &a.Category,
			&a.Overview, &a.Requirements, &a.Image, &a.CreatedAt); err != nil {
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
	versionMap, err := batchGetVersions(db, ids)
	if err != nil {
		return nil, 0, err
	}
	for i := range apps {
		apps[i].Versions = versionMap[apps[i].ID]
	}
	return apps, total, nil
}

func GetCategories(platform string) ([]CategoryCount, error) {
	db, err := getReadDB(platform)
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`
		SELECT category, COUNT(*) as count
		FROM apps GROUP BY category ORDER BY category ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]CategoryCount, 0)
	for rows.Next() {
		var c CategoryCount
		if err := rows.Scan(&c.Category, &c.Count); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

type CategoryCount struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}

func batchGetVersions(db *sql.DB, appIDs []int64) (map[int64][]AppVersion, error) {
	if len(appIDs) == 0 {
		return map[int64][]AppVersion{}, nil
	}
	placeholders := strings.Repeat("?,", len(appIDs))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(appIDs))
	for i, id := range appIDs {
		args[i] = id
	}
	rows, err := db.Query(
		`SELECT id, app_id, version, cdn_query, created_at FROM (
			SELECT id, app_id, version, cdn_query, created_at,
					ROW_NUMBER() OVER (PARTITION BY app_id ORDER BY created_at DESC) AS rn
			FROM app_versions
			WHERE app_id IN (`+placeholders+`)
		) WHERE rn <= 5`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64][]AppVersion)
	for rows.Next() {
		var v AppVersion
		if err := rows.Scan(&v.ID, &v.AppID, &v.Version, &v.CDNQuery, &v.CreatedAt); err != nil {
			return nil, err
		}
		result[v.AppID] = append(result[v.AppID], v)
	}
	return result, rows.Err()
}

// ─── Write ────────────────────────────────────────────────────────────────────

// UpsertEntry — input dari admin, tanpa URL
type UpsertEntry struct {
	Name         string
	Category     string
	Overview     string
	Requirements string
	Image        string
	Version      string
	// CDNQuery opsional — override keyword pencarian CDN
	// kosong = pakai nama app
	CDNQuery string
}

type UpsertResult struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	Action    string `json:"action"` // "created" | "version_added" | "duplicate"
	Version   string `json:"version"`
	Duplicate bool   `json:"duplicate"`
}

func Upsert(platform string, e UpsertEntry) (UpsertResult, error) {
	db, err := getWriteDB(platform)
	if err != nil {
		return UpsertResult{}, err
	}

	tx, err := db.Begin()
	if err != nil {
		return UpsertResult{}, fmt.Errorf("upsert: begin tx: %w", err)
	}
	defer tx.Rollback()

	baseSlug := toSlug(e.Name)
	slug := baseSlug
	for i := 2; i <= 10; i++ {
		var count int
		tx.QueryRow(`SELECT COUNT(*) FROM apps WHERE slug = ?`, slug).Scan(&count)
		if count == 0 {
			break
		}
		if i == 10 {
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

		if err := insertVersionTx(tx, appID, e.Version, e.CDNQuery); err != nil {
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

	// app sudah ada — cek duplikat versi
	var versionExists int
	tx.QueryRow(
		`SELECT COUNT(*) FROM app_versions WHERE app_id = ? AND LOWER(version) = LOWER(?)`,
		appID, e.Version,
	).Scan(&versionExists)

	if versionExists > 0 {
		return UpsertResult{Slug: existingSlug, Name: e.Name, Action: "duplicate", Version: e.Version, Duplicate: true}, nil
	}

	// update image kalau ada yang baru
	if e.Image != "" {
		if _, err := tx.Exec(`UPDATE apps SET image = ? WHERE id = ?`, e.Image, appID); err != nil {
			return UpsertResult{}, fmt.Errorf("update image: %w", err)
		}
	}
	if err := insertVersionTx(tx, appID, e.Version, e.CDNQuery); err != nil {
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
			if vErr := insertVersionTx(tx, appID, e.Version, e.CDNQuery); vErr != nil {
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
			`SELECT COUNT(*) FROM app_versions WHERE app_id = ? AND LOWER(version) = LOWER(?)`,
			appID, e.Version,
		).Scan(&versionExists)
		if versionExists > 0 {
			results = append(results, UpsertResult{Slug: existingSlug, Name: e.Name, Action: "duplicate", Version: e.Version, Duplicate: true})
			continue
		}
		if e.Image != "" {
			if _, uErr := tx.Exec(`UPDATE apps SET image = ? WHERE id = ?`, e.Image, appID); uErr != nil {
				errs[i] = fmt.Errorf("update image: %w", uErr)
				continue
			}
		}
		if vErr := insertVersionTx(tx, appID, e.Version, e.CDNQuery); vErr != nil {
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

func insertVersionTx(tx *sql.Tx, appID int64, version, cdnQuery string) error {
	_, err := tx.Exec(
		`INSERT INTO app_versions (app_id, version, cdn_query, created_at) VALUES (?, ?, ?, ?)`,
		appID, version, cdnQuery, time.Now().Format(time.RFC3339),
	)
	return err
}

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
	res, err := db.Exec(`DELETE FROM app_versions WHERE id = ?`, versionID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func sanitizeFTS(q string) string {
	replacer := strings.NewReplacer(
		`"`, ``, `*`, ``, `(`, ``, `)`, ``,
		`^`, ``, `{`, ``, `}`, ``, `[`, ``,
		`]`, ``, `:`, ``, `+`, ``,
	)
	return strings.TrimSpace(replacer.Replace(q))
}

func toSlug(name string) string {
	lower := strings.ToLower(name)
	var words []string
	for _, w := range strings.Fields(lower) {
		if len(w) > 1 && w[0] == 'v' && w[1] >= '0' && w[1] <= '9' {
			break
		}
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

func normalizeCategory(cat string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(cat)), " ", "-")
}
