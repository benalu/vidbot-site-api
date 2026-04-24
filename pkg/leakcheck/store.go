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
	db            *sql.DB
	mu            sync.RWMutex
	dir           string
	writeMu       sync.Mutex // serialisasi operasi write: AddDir dan Reload
	cachedCount   int
	cachedCountAt time.Time
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

	// PostgreSQL connection pool — disesuaikan untuk VPS 4GB
	// MaxOpenConns 10 cukup untuk read-heavy workload leakcheck
	// sisakan koneksi untuk stats DB dan proses lain di VPS
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(3 * time.Minute)

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
	db.QueryRow(`SELECT COUNT(1) FROM leakcheck WHERE LOWER(login) = 'warmup_dummy_xyz'`).Scan(new(int))
	slog.Info("leakcheck query planner warmed")

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

	// Cleanup expired search cache setiap 10 menit
	// Tanpa ini, entry expired tetap di memory kalau traffic berhenti
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				now := time.Now()
				sCache.mu.Lock()
				for k, v := range sCache.entries {
					if now.After(v.expiresAt) {
						delete(sCache.entries, k)
					}
				}
				sCache.mu.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stats — info status untuk admin endpoint
type Stats struct {
	EntryCount   int     `json:"entry_count"`
	DBReady      bool    `json:"db_ready"`
	LatencyMs    int64   `json:"latency_ms"`
	CacheSize    int     `json:"cache_size"`
	CacheHitRate float64 `json:"cache_hit_rate"`
}

func (s *Store) Stats() Stats {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	result := Stats{}

	if db == nil {
		return result
	}

	result.DBReady = true
	result.EntryCount = s.CachedCount()

	// ukur latency dengan dummy query
	start := time.Now()
	db.QueryRow(`SELECT 1`).Scan(new(int))
	result.LatencyMs = time.Since(start).Milliseconds()

	// cache size
	sCache.mu.RLock()
	result.CacheSize = len(sCache.entries)
	sCache.mu.RUnlock()

	return result
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

		`CREATE UNIQUE INDEX IF NOT EXISTS idx_leakcheck_unique
         ON leakcheck(login, password)`,

		`CREATE INDEX IF NOT EXISTS idx_leakcheck_login_trgm
         ON leakcheck USING gin (LOWER(login) gin_trgm_ops)`,

		`CREATE INDEX IF NOT EXISTS idx_leakcheck_login
         ON leakcheck(LOWER(login))`,

		`ALTER SYSTEM SET pg_trgm.similarity_threshold = 0.1`,
		`SELECT pg_reload_conf()`,

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

// Search — cari berdasarkan email atau username, keduanya exact match
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

	// Pakai db.QueryContext() langsung — tidak butuh pin ke satu koneksi
	// karena tidak ada SET LOCAL yang perlu dijalankan dulu.
	// Pool otomatis handle ambil dan kembalikan koneksi dengan aman.
	var rows *sql.Rows
	var err error

	rows, err = db.QueryContext(ctx,
		`SELECT source, soft, host, login, password
		 FROM leakcheck
		 WHERE LOWER(login) = $1
		 LIMIT 500`,
		lowerQ,
	)

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
	// Pastikan hanya satu operasi write (AddDir/Reload) berjalan sekaligus
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	// Pre-check: hitung dulu berapa file sebelum mulai insert
	fileCount := 0
	var totalSize int64
	filepath.WalkDir(newDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if strings.HasSuffix(strings.ToLower(path), ".txt") {
			fileCount++
			if info, infoErr := d.Info(); infoErr == nil {
				totalSize += info.Size()
			}
		}
		return nil
	})
	if fileCount == 0 {
		return 0, fmt.Errorf("no .txt files found in dir: %s", newDir)
	}
	const maxFilesPerAddDir = 500
	if fileCount > maxFilesPerAddDir {
		return 0, fmt.Errorf("too many files: %d (max %d), gunakan Reload() untuk dataset besar",
			fileCount, maxFilesPerAddDir)
	}
	slog.Info("leakcheck add-dir pre-check",
		"dir", newDir,
		"files", fileCount,
		"total_size_mb", totalSize/1024/1024,
	)

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
	// writeMu memastikan Reload tidak berjalan bersamaan dengan AddDir
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

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

	// Step 5: atomic swap — bungkus dalam transaksi supaya kedua ALTER TABLE
	// commit atau rollback bersama. Kalau salah satu gagal, tabel lama tetap ada.
	slog.Info("leakcheck swapping tables...")
	swapTx, swapErr := db.Begin()
	if swapErr != nil {
		db.Exec(`DROP TABLE IF EXISTS leakcheck_new`)
		return 0, fmt.Errorf("table swap begin tx failed: %w", swapErr)
	}
	_, swapErr = swapTx.Exec(`ALTER TABLE leakcheck RENAME TO leakcheck_old`)
	if swapErr != nil {
		swapTx.Rollback()
		db.Exec(`DROP TABLE IF EXISTS leakcheck_new`)
		return 0, fmt.Errorf("table swap rename old failed: %w", swapErr)
	}
	_, swapErr = swapTx.Exec(`ALTER TABLE leakcheck_new RENAME TO leakcheck`)
	if swapErr != nil {
		swapTx.Rollback()
		return 0, fmt.Errorf("table swap rename new failed: %w", swapErr)
	}
	if swapErr = swapTx.Commit(); swapErr != nil {
		swapTx.Rollback()
		return 0, fmt.Errorf("table swap commit failed: %w", swapErr)
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
			if prepErr != nil {
				bSources = bSources[:0]
				bSofts = bSofts[:0]
				bHosts = bHosts[:0]
				bLogins = bLogins[:0]
				bPasswords = bPasswords[:0]
				return fmt.Errorf("fallback prepare failed: %w", prepErr)
			}
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
				if fErr := flush(); fErr != nil {
					return fmt.Errorf("flush batch failed: %w", fErr)
				}
			}
		}

		// Progress log per file
		slog.Info("leakcheck file processed",
			"file", filepath.Base(path),
			"file_no", fileCount,
			"entries_in_file", len(entries),
			"inserted_so_far", inserted,
		)

		return nil
	})

	if err != nil {
		return inserted, err
	}

	// Flush sisa data yang belum dikirim
	if fErr := flush(); fErr != nil {
		return inserted, fmt.Errorf("final flush failed: %w", fErr)
	}

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
	if !utf8.ValidString(s) {
		var b strings.Builder
		b.Grow(len(s))
		for i := 0; i < len(s); {
			r, size := utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError && size == 1 {
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

// Count mengembalikan estimasi jumlah entry dari pg_class (cepat).
// Akurat setelah ANALYZE dijalankan — dipanggil otomatis setelah
// loadFromDir() dan AddDir() selesai.
func (s *Store) Count() int {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()
	if db == nil {
		return 0
	}
	var count int
	db.QueryRow(`
        SELECT GREATEST(reltuples::BIGINT, 0)
        FROM pg_class
        WHERE relname = 'leakcheck'
    `).Scan(&count)
	return count
}

// CachedCount mengembalikan Count() dengan cache 1 menit.
// Gunakan ini di response handler supaya tidak ada query DB setiap request.
func (s *Store) CachedCount() int {
	s.mu.RLock()
	if time.Since(s.cachedCountAt) < 1*time.Minute && s.cachedCount > 0 {
		val := s.cachedCount
		s.mu.RUnlock()
		return val
	}
	s.mu.RUnlock()

	count := s.Count()

	s.mu.Lock()
	s.cachedCount = count
	s.cachedCountAt = time.Now()
	s.mu.Unlock()

	return count
}

// ─── Parser ───────────────────────────────────────────────────────────────────

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
