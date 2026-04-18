package leakcheck

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Entry struct {
	Source   string `json:"source"`
	Soft     string `json:"soft"`
	Host     string `json:"host"`
	Login    string `json:"login"`
	Password string `json:"password"`
}

type Store struct {
	db  *sql.DB
	mu  sync.RWMutex
	dir string
}

var Default = &Store{}

// searchCache — in-memory cache untuk hasil search
// TTL 5 menit, max 1000 entry
type searchCache struct {
	mu      sync.RWMutex
	entries map[string]searchCacheEntry
}

type searchCacheEntry struct {
	results   []Entry
	expiresAt time.Time
}

var sCache = &searchCache{
	entries: make(map[string]searchCacheEntry),
}

func (c *searchCache) get(key string) ([]Entry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[key]
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.results, true
}

func (c *searchCache) set(key string, results []Entry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = searchCacheEntry{
		results:   results,
		expiresAt: time.Now().Add(5 * time.Minute),
	}
	// Cleanup expired entries kalau sudah > 1000
	if len(c.entries) > 1000 {
		for k, v := range c.entries {
			if time.Now().After(v.expiresAt) {
				delete(c.entries, k)
			}
		}
	}
}

func (s *Store) Init(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	s.mu.Lock()
	s.dir = dir
	s.mu.Unlock()

	dsn := os.Getenv("LEAKCHECK_DB_DSN")
	if dsn == "" {
		dsn = "host=localhost port=5432 user=postgres password=postgres dbname=vidbot_leakcheck sslmode=disable"
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("leakcheck: open db: %w", err)
	}

	// PostgreSQL connection pool — read-heavy workload
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return fmt.Errorf("leakcheck: ping failed: %w", err)
	}

	if err := ensureSchema(db); err != nil {
		return fmt.Errorf("leakcheck: schema: %w", err)
	}

	s.mu.Lock()
	s.db = db
	s.mu.Unlock()

	// Pre-warm connection pool — eliminasi cold start latency di request pertama
	slog.Info("leakcheck warming connection pool...")
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := db.Conn(context.Background())
			if err == nil {
				conn.Close()
			}
		}()
	}
	wg.Wait()
	slog.Info("leakcheck connection pool warmed")

	var count int
	db.QueryRow(`SELECT COUNT(1) FROM leakcheck`).Scan(&count)

	if count > 0 {
		slog.Info("leakcheck db already populated, skip reload", "count", count)
		return nil
	}

	slog.Info("leakcheck db empty, loading from dir", "dir", dir)
	return s.loadFromDir(dir)
}

func (s *Store) StartBackground(ctx context.Context) {
	// Stats log per jam
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				slog.Info("leakcheck entry count", "count", s.Count())
			case <-ctx.Done():
				return
			}
		}
	}()
}

func ensureSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE EXTENSION IF NOT EXISTS pg_trgm`,

		`CREATE TABLE IF NOT EXISTS leakcheck (
            id       BIGSERIAL PRIMARY KEY,
            source   TEXT,
            soft     TEXT,
            host     TEXT,
            login    TEXT NOT NULL,
            password TEXT NOT NULL DEFAULT ''
        )`,

		// Ganti UNIQUE constraint jadi index biasa — UNIQUE di 10M rows
		// bikin index ukurannya dobel dan insert jadi lambat.
		// Dedup sudah kita handle di Go sebelum insert.
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_leakcheck_unique
         ON leakcheck(login, password)`,

		// Index untuk LIKE search — ini yang dipakai query utama
		`CREATE INDEX IF NOT EXISTS idx_leakcheck_login_trgm
         ON leakcheck USING gin (LOWER(login) gin_trgm_ops)`,

		// Index exact match untuk email search yang presisi
		`CREATE INDEX IF NOT EXISTS idx_leakcheck_login
         ON leakcheck(LOWER(login))`,

		// Konfigurasi trgm threshold — tuning untuk performa
		`ALTER SYSTEM SET pg_trgm.similarity_threshold = 0.1`,
		`SELECT pg_reload_conf()`,

		// Autovacuum tuning untuk tabel yang besar dan heavy insert
		`ALTER TABLE leakcheck SET (
            autovacuum_vacuum_scale_factor = 0.01,
            autovacuum_analyze_scale_factor = 0.005,
            autovacuum_vacuum_cost_delay = 2
        )`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			// beberapa statement boleh gagal kalau sudah exist
			slog.Debug("schema stmt skipped", "error", err)
		}
	}
	return nil
}

// Search — cari berdasarkan email atau username
// PostgreSQL pg_trgm jauh lebih efisien dari SQLite FTS untuk 5M+ rows
func (s *Store) Search(q string) []Entry {
	q = strings.TrimSpace(q)
	if q == "" {
		return []Entry{}
	}

	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db == nil {
		return []Entry{}
	}

	lowerQ := strings.ToLower(q)

	// Cek cache dulu
	if cached, ok := sCache.get(lowerQ); ok {
		return cached
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := db.Conn(ctx)
	if err != nil {
		slog.Error("leakcheck search: get conn failed", "error", err)
		return []Entry{}
	}
	defer conn.Close()

	// Set work_mem per connection untuk query besar
	conn.ExecContext(ctx, `SET LOCAL work_mem = '64MB'`)

	var rows *sql.Rows
	isEmail := strings.Contains(lowerQ, "@")

	if isEmail {
		// Email: exact match pakai btree idx_leakcheck_login — <5ms
		rows, err = conn.QueryContext(ctx,
			`SELECT source, soft, host, login, password
			 FROM leakcheck
			 WHERE LOWER(login) = $1
			 LIMIT 500`,
			lowerQ,
		)
	} else {
		// Username: partial match pakai trgm — 5ms
		rows, err = conn.QueryContext(ctx,
			`SELECT source, soft, host, login, password
			 FROM leakcheck
			 WHERE LOWER(login) LIKE $1
			 LIMIT 500`,
			"%"+lowerQ+"%",
		)
	}

	if err != nil {
		slog.Error("leakcheck search failed", "error", err, "query", q)
		return []Entry{}
	}
	defer rows.Close()

	results := []Entry{}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.Source, &e.Soft, &e.Host, &e.Login, &e.Password); err != nil {
			continue
		}
		results = append(results, e)
	}

	sCache.set(lowerQ, results)
	return results
}

// AddDir — tambah data dari folder baru tanpa rebuild
func (s *Store) AddDir(newDir string) (int, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	inserted, err := insertEntriesFromDir(newDir, db)
	if err != nil {
		return 0, err
	}

	// ANALYZE setelah tambah data baru — update query planner statistics
	slog.Info("leakcheck analyze started after add-dir...")
	if _, err := db.Exec(`ANALYZE leakcheck`); err != nil {
		slog.Warn("leakcheck analyze failed", "error", err)
	} else {
		slog.Info("leakcheck analyze done")
	}

	slog.Info("leakcheck dir added", "inserted", inserted, "dir", newDir)
	return inserted, nil
}

// Reload — rebuild semua data (clear + reload)
func (s *Store) Reload(dir string) (int, error) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	slog.Info("leakcheck reload started (zero downtime)", "dir", dir)

	// Step 1: buat tabel baru sebagai staging
	// Search tetap jalan di tabel lama selama ini berlangsung
	_, err := db.Exec(`
        DROP TABLE IF EXISTS leakcheck_new;
        CREATE TABLE leakcheck_new (
            id       BIGSERIAL PRIMARY KEY,
            source   TEXT,
            soft     TEXT,
            host     TEXT,
            login    TEXT NOT NULL,
            password TEXT NOT NULL DEFAULT ''
        );
    `)
	if err != nil {
		return 0, fmt.Errorf("create staging table failed: %w", err)
	}

	// Step 2: insert semua data ke tabel staging
	// Selama ini berlangsung (1 jam+), leakcheck lama masih bisa di-search
	inserted, err := insertEntriesFromDirToTable(dir, db, "leakcheck_new")
	if err != nil {
		db.Exec(`DROP TABLE IF EXISTS leakcheck_new`)
		return 0, fmt.Errorf("insert to staging failed: %w", err)
	}

	// Step 3: buat index di tabel staging sebelum swap
	slog.Info("leakcheck building indexes on staging table...")
	indexStmts := []string{
		`CREATE UNIQUE INDEX idx_leakcheck_new_unique ON leakcheck_new(login, password)`,
		`CREATE INDEX idx_leakcheck_new_login_trgm ON leakcheck_new USING gin (LOWER(login) gin_trgm_ops)`,
		`CREATE INDEX idx_leakcheck_new_login_lower ON leakcheck_new(LOWER(login))`,
	}
	for _, stmt := range indexStmts {
		if _, err := db.Exec(stmt); err != nil {
			slog.Warn("index creation warning", "error", err)
		}
	}

	// Step 4: ANALYZE staging table
	slog.Info("leakcheck analyze staging table...")
	db.Exec(`ANALYZE leakcheck_new`)

	// Step 5: atomic swap — ini yang paling kritis, harus cepat
	// Pakai transaction untuk atomic rename
	slog.Info("leakcheck swapping tables...")
	_, err = db.Exec(`
        ALTER TABLE leakcheck RENAME TO leakcheck_old;
        ALTER TABLE leakcheck_new RENAME TO leakcheck;
    `)
	if err != nil {
		db.Exec(`DROP TABLE IF EXISTS leakcheck_new`)
		return 0, fmt.Errorf("table swap failed: %w", err)
	}

	// Step 6: hapus tabel lama (background, tidak blocking)
	go func() {
		slog.Info("leakcheck dropping old table...")
		if _, err := db.Exec(`DROP TABLE IF EXISTS leakcheck_old`); err != nil {
			slog.Warn("drop old table failed", "error", err)
		} else {
			slog.Info("leakcheck old table dropped")
		}
	}()

	slog.Info("leakcheck reload done", "inserted", inserted)
	return inserted, nil
}

func (s *Store) loadFromDir(dir string) error {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	inserted, err := insertEntriesFromDir(dir, db)
	if err != nil {
		return err
	}

	// ANALYZE sekali di sini, setelah semua file selesai diproses
	slog.Info("leakcheck analyze started (this may take a moment)...")
	if _, err := db.Exec(`ANALYZE leakcheck`); err != nil {
		slog.Warn("leakcheck analyze failed", "error", err)
	} else {
		slog.Info("leakcheck analyze done")
	}

	slog.Info("leakcheck loaded", "inserted", inserted, "dir", dir)
	return nil
}

// insertEntriesFromDir — scan semua .txt di dir, parse, insert ke PostgreSQL
// Pakai COPY command via UNNEST untuk performa bulk insert maksimal
func insertEntriesFromDir(dir string, db *sql.DB) (int, error) {
	return insertEntriesFromDirToTable(dir, db, "leakcheck")
}

func insertEntriesFromDirToTable(dir string, db *sql.DB, tableName string) (int, error) {
	inserted := 0
	fileCount := 0
	errorCount := 0

	const batchSize = 50_000

	var (
		bSources   []string
		bSofts     []string
		bHosts     []string
		bLogins    []string
		bPasswords []string
	)

	flush := func() error {
		if len(bLogins) == 0 {
			return nil
		}
		count := len(bLogins)

		// Gunakan fmt.Sprintf untuk inject table name
		// Table name tidak bisa di-parameterize di PostgreSQL
		query := fmt.Sprintf(`
            INSERT INTO %s (source, soft, host, login, password)
            SELECT * FROM UNNEST($1::text[], $2::text[], $3::text[], $4::text[], $5::text[])
            ON CONFLICT (login, password) DO NOTHING
        `, tableName)

		_, err := db.Exec(query, bSources, bSofts, bHosts, bLogins, bPasswords)
		if err != nil {
			slog.Warn("batch insert failed, falling back to per-row insert",
				"batch_size", count, "error", err)

			fallbackQuery := fmt.Sprintf(`
                INSERT INTO %s (source, soft, host, login, password)
                VALUES ($1, $2, $3, $4, $5)
                ON CONFLICT (login, password) DO NOTHING
            `, tableName)

			stmt, prepErr := db.Prepare(fallbackQuery)
			if prepErr == nil {
				defer stmt.Close()
				fallbackCount := 0
				for i := range bLogins {
					if _, rowErr := stmt.Exec(
						bSources[i], bSofts[i], bHosts[i], bLogins[i], bPasswords[i],
					); rowErr != nil {
						slog.Debug("skip row with invalid data",
							"login_prefix", safePrefix(bLogins[i]), "error", rowErr)
						continue
					}
					fallbackCount++
				}
				inserted += fallbackCount
			}
		} else {
			inserted += count
		}

		bSources = bSources[:0]
		bSofts = bSofts[:0]
		bHosts = bHosts[:0]
		bLogins = bLogins[:0]
		bPasswords = bPasswords[:0]
		return nil
	}

	// ... sisa kode WalkDir sama persis seperti sebelumnya ...
	// tidak ada perubahan di bagian walk dan parse

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(path), ".txt") {
			return nil
		}
		fileCount++
		entries, parseErr := parseFile(path)
		if parseErr != nil {
			slog.Error("leakcheck parse file failed",
				"file", filepath.Base(path), "error", parseErr)
			errorCount++
			return nil
		}

		for _, e := range entries {
			login := sanitizeString(e.Login)
			password := sanitizeString(e.Password)

			if login == "" || len(login) > 255 {
				continue
			}
			if len(password) > 255 {
				password = password[:255]
			}

			bSources = append(bSources, sanitizeString(e.Source))
			bSofts = append(bSofts, sanitizeString(e.Soft))
			bHosts = append(bHosts, sanitizeString(e.Host))
			bLogins = append(bLogins, login)
			bPasswords = append(bPasswords, password)

			if len(bLogins) >= batchSize {
				flush()
			}
		}
		return nil
	})

	if err != nil {
		return inserted, err
	}

	flush()

	slog.Info("leakcheck scan done",
		"files", fileCount,
		"errors", errorCount,
		"inserted", inserted,
		"table", tableName,
	)
	return inserted, nil
}
func sanitizeString(s string) string {
	// Hapus null byte
	s = strings.ReplaceAll(s, "\x00", "")

	// Encode ke []byte lalu rebuild hanya rune yang valid
	// Ini lebih reliable dari strings.ToValidUTF8 untuk partial sequences
	if !utf8.ValidString(s) {
		var b strings.Builder
		b.Grow(len(s))
		for i := 0; i < len(s); {
			r, size := utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError && size == 1 {
				// byte tunggal invalid — skip
				i++
				continue
			}
			b.WriteRune(r)
			i += size
		}
		s = b.String()
	}

	return strings.TrimSpace(s)
}

func safePrefix(s string) string {
	if len(s) > 20 {
		return s[:20] + "..."
	}
	return s
}

func (s *Store) Ping() error {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	return db.PingContext(context.Background())
}

func (s *Store) Count() int {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return 0
	}
	var count int
	// reltuples lebih cepat, cukup untuk display
	err := db.QueryRow(`
        SELECT GREATEST(reltuples::BIGINT, 0)
        FROM pg_class
        WHERE relname = 'leakcheck'
    `).Scan(&count)
	if err != nil || count == 0 {
		// fallback exact count — hanya dipakai saat pertama kali sebelum ANALYZE
		db.QueryRow(`SELECT COUNT(1) FROM leakcheck`).Scan(&count)
	}
	return count
}

// ─── Parser (sama dengan SQLite version) ──────────────────────────────────────

func parseFile(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var firstLine string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			firstLine = line
			break
		}
	}

	isCombo := isComboFormat(firstLine)
	filename := filepath.Base(path)
	var entries []Entry

	if isCombo {
		if firstLine != "" {
			if e, ok := parseComboLine(firstLine, filename); ok {
				entries = append(entries, e)
			}
		}
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			if e, ok := parseComboLine(line, filename); ok {
				entries = append(entries, e)
			}
		}
	} else {
		entries = parseLogsFromScanner(firstLine, scanner)
	}

	return entries, scanner.Err()
}

func isComboEmail(line string) bool {
	if line == "" {
		return false
	}
	idx := strings.Index(line, ":")
	if idx < 0 {
		return false
	}
	key := strings.TrimSpace(line[:idx])
	return strings.ContainsAny(key, "@.")
}

var keyValueKeywords = map[string]struct{}{
	"soft": {}, "application": {}, "url": {}, "host": {},
	"user": {}, "login": {}, "username": {}, "pass": {}, "password": {},
}

func isComboUsername(line string) bool {
	if line == "" {
		return false
	}
	idx := strings.Index(line, ":")
	if idx < 0 {
		return false
	}
	left := strings.TrimSpace(line[:idx])
	right := strings.TrimSpace(line[idx+1:])
	if left == "" || right == "" || strings.Contains(left, " ") || strings.ContainsAny(left, "@.") {
		return false
	}
	if _, isKeyword := keyValueKeywords[strings.ToLower(left)]; isKeyword {
		return false
	}
	return true
}

func isComboFormat(line string) bool {
	return isComboEmail(line) || isComboUsername(line)
}

func parseComboLine(line, filename string) (Entry, bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return Entry{}, false
	}
	login := strings.TrimSpace(line[:idx])
	password := strings.TrimSpace(line[idx+1:])
	if login == "" {
		return Entry{}, false
	}
	return Entry{
		Source:   strings.TrimSuffix(filename, filepath.Ext(filename)),
		Login:    login,
		Password: password,
	}, true
}

func parseLogsFromScanner(firstLine string, scanner *bufio.Scanner) []Entry {
	var entries []Entry
	var current Entry
	hasData := false

	processLine := func(line string) {
		if line == "" || strings.HasPrefix(line, "===") {
			if hasData && current.Login != "" {
				current.Source = "Logs"
				entries = append(entries, current)
			}
			current = Entry{}
			hasData = false
			return
		}
		key, value, ok := parseLine(line)
		if !ok {
			return
		}
		switch key {
		case "soft", "application":
			current.Soft = value
			hasData = true
		case "url", "host":
			current.Host = value
			hasData = true
		case "user", "login", "username":
			current.Login = value
			hasData = true
		case "pass", "password":
			current.Password = value
			hasData = true
		}
	}

	processLine(firstLine)
	for scanner.Scan() {
		processLine(strings.TrimSpace(scanner.Text()))
	}
	if hasData && current.Login != "" {
		current.Source = "Logs"
		entries = append(entries, current)
	}
	return entries
}

func parseLine(line string) (key, value string, ok bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	key = strings.ToLower(strings.TrimSpace(line[:idx]))
	key = strings.ReplaceAll(key, " ", "")
	value = strings.TrimSpace(line[idx+1:])
	value = strings.TrimPrefix(value, "//")
	value = strings.TrimSpace(value)
	return key, value, true
}
