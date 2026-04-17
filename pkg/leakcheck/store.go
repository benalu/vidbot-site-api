package leakcheck

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Entry struct {
	Source   string `json:"source"`
	Soft     string `json:"soft"`
	Host     string `json:"host"`
	Login    string `json:"login"`
	Password string `json:"password"`
}

type Store struct {
	db        *sql.DB
	readDB    *sql.DB
	mu        sync.RWMutex
	reloading bool
	dir       string // simpan dir supaya AddDir & background bisa akses
}

var Default = &Store{}

func (s *Store) Init(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	s.mu.Lock()
	s.dir = dir
	s.mu.Unlock()

	// cleanup sisa file temp dari reload yang gagal sebelumnya
	tmpPath := filepath.Join(dir, "leakcheck_new.db")
	for _, f := range []string{tmpPath, tmpPath + "-shm", tmpPath + "-wal"} {
		os.Remove(f)
	}

	dbPath := filepath.Join(dir, "leakcheck.db")
	db, err := openWriteDB(dbPath)
	if err != nil {
		return err
	}

	readDB, err := openReadDB(dbPath)
	if err != nil {
		db.Close()
		return err
	}

	if err := ensureSchema(db); err != nil {
		db.Close()
		readDB.Close()
		return err
	}

	s.mu.Lock()
	s.db = db
	s.readDB = readDB
	s.mu.Unlock()

	count := 0
	if err := db.QueryRow(`SELECT COUNT(1) FROM leakcheck`).Scan(&count); err != nil {
		return err
	}

	if count > 0 {
		log.Printf("[leakcheck] database already has %d entries (%s), skip reload", count, dbPath)
		return nil
	}

	return s.loadFromDir(dir)
}

// StartBackground menjalankan goroutine pemeliharaan.
// Panggil setelah Init() di main.go:
//
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//	leakcheck.Default.StartBackground(ctx)
func (s *Store) StartBackground(ctx context.Context) {
	// WAL checkpoint — cegah WAL file membengkak
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.mu.RLock()
				db := s.db
				s.mu.RUnlock()
				if db != nil {
					if _, err := db.Exec(`PRAGMA wal_checkpoint(PASSIVE)`); err != nil {
						log.Printf("[leakcheck] wal_checkpoint error: %v", err)
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Stats log per jam
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				log.Printf("[leakcheck] entries: %d", s.Count())
			case <-ctx.Done():
				return
			}
		}
	}()
}

// AddDir menambah data dari direktori baru tanpa full rebuild.
// Cocok untuk insert data tambahan saat sudah production.
// FTS diupdate otomatis via trigger per-row — tidak ada downtime.
func (s *Store) AddDir(newDir string) (int, error) {
	s.mu.Lock()
	if s.reloading {
		s.mu.Unlock()
		return 0, fmt.Errorf("reload already in progress")
	}
	s.reloading = true
	db := s.db
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.reloading = false
		s.mu.Unlock()
	}()

	if db == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	// Untuk AddDir kita pakai INSERT OR IGNORE karena DB sudah punya UNIQUE constraint
	// dan kita tidak mau rebuild seluruh seen map untuk data yang sudah ada
	inserted, err := insertEntriesFromDirIncremental(newDir, db)
	if err != nil {
		return 0, err
	}

	log.Printf("[leakcheck] AddDir: added %d new entries from %s", inserted, newDir)
	return inserted, nil
}

func (s *Store) Search(q string) []Entry {
	q = strings.TrimSpace(q)
	if q == "" {
		return []Entry{}
	}

	s.mu.RLock()
	db := s.readDB
	s.mu.RUnlock()

	if db == nil {
		return []Entry{}
	}

	if strings.Contains(q, "@") {
		return s.searchEmail(db, q)
	}
	return s.searchUsername(db, q)
}

func (s *Store) searchEmail(db *sql.DB, q string) []Entry {
	tokens := tokenizeEmail(q)
	if len(tokens) == 0 {
		return s.searchFallback(q)
	}

	ftsTokens := make([]string, len(tokens))
	for i, t := range tokens {
		ftsTokens[i] = sanitizeFTSQuery(strings.ToLower(t))
	}
	clean := ftsTokens[:0]
	for _, t := range ftsTokens {
		if t != "" {
			clean = append(clean, t)
		}
	}
	ftsTokens = clean
	if len(ftsTokens) == 0 {
		return s.searchFallback(q)
	}

	ftsQuery := strings.Join(ftsTokens, " AND ")
	lowerQ := strings.ToLower(q)

	rows, err := db.Query(`
		SELECT l.source, l.soft, l.host, l.login, l.password
		FROM leakcheck_fts f
		JOIN leakcheck l ON l.id = f.rowid
		WHERE leakcheck_fts MATCH ?
		LIMIT 500
	`, ftsQuery)
	if err != nil {
		log.Printf("[leakcheck] fts email search failed, fallback: %v", err)
		return s.searchFallback(q)
	}
	defer rows.Close()

	results := []Entry{}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.Source, &e.Soft, &e.Host, &e.Login, &e.Password); err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(e.Login), lowerQ) {
			results = append(results, e)
		}
		if len(results) >= 500 {
			break
		}
	}
	return results
}

func (s *Store) searchUsername(db *sql.DB, q string) []Entry {
	lowerQ := strings.ToLower(q)
	hasUsernameSpecial := strings.ContainsAny(q, "_-")

	var ftsQuery string
	if hasUsernameSpecial {
		tokens := tokenizeEmail(q)
		if len(tokens) == 0 {
			return s.searchFallback(q)
		}
		ftsTokens := make([]string, len(tokens))
		for i, t := range tokens {
			ftsTokens[i] = strings.ToLower(t)
		}
		ftsQuery = strings.Join(ftsTokens, " AND ")
	} else {
		ftsQuery = sanitizeFTSQuery(lowerQ)
		if ftsQuery == "" {
			return s.searchFallback(q)
		}
	}

	rows, err := db.Query(`
		SELECT l.source, l.soft, l.host, l.login, l.password
		FROM leakcheck_fts f
		JOIN leakcheck l ON l.id = f.rowid
		WHERE leakcheck_fts MATCH ?
		LIMIT 500
	`, ftsQuery)
	if err != nil {
		log.Printf("[leakcheck] fts username search failed, fallback: %v", err)
		return s.searchFallback(q)
	}
	defer rows.Close()

	results := []Entry{}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.Source, &e.Soft, &e.Host, &e.Login, &e.Password); err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(e.Login), lowerQ) {
			results = append(results, e)
		}
		if len(results) >= 500 {
			break
		}
	}
	return results
}

func (s *Store) searchFallback(q string) []Entry {
	s.mu.RLock()
	db := s.readDB
	s.mu.RUnlock()

	if db == nil {
		return []Entry{}
	}

	like := "%" + strings.ToLower(q) + "%"
	rows, err := db.Query(`
		SELECT source, soft, host, login, password
		FROM leakcheck
		WHERE lower(login) LIKE ?
		LIMIT 500
	`, like)
	if err != nil {
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
	return results
}

func (s *Store) Reload(dir string) (int, error) {
	s.mu.Lock()
	if s.reloading {
		s.mu.Unlock()
		return 0, fmt.Errorf("reload already in progress")
	}
	s.reloading = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.reloading = false
		s.mu.Unlock()
	}()

	tmpPath := filepath.Join(dir, "leakcheck_new.db")

	// bersihkan sisa build gagal sebelumnya kalau ada
	for _, f := range []string{tmpPath, tmpPath + "-shm", tmpPath + "-wal"} {
		os.Remove(f)
	}

	log.Printf("[leakcheck] reload: building new database at %s", tmpPath)
	reloadStart := time.Now()
	newDB, err := buildDatabase(tmpPath, dir)
	if err != nil {
		for _, f := range []string{tmpPath, tmpPath + "-shm", tmpPath + "-wal"} {
			os.Remove(f)
		}
		return 0, fmt.Errorf("reload: build failed: %w", err)
	}

	count := 0
	newDB.QueryRow(`SELECT COUNT(1) FROM leakcheck`).Scan(&count)
	if count == 0 {
		newDB.Close()
		for _, f := range []string{tmpPath, tmpPath + "-shm", tmpPath + "-wal"} {
			os.Remove(f)
		}
		return 0, fmt.Errorf("reload: new database is empty, aborting swap")
	}
	log.Printf("[leakcheck] reload: new database ready with %d entries, swapping", count)

	// swap pointer — search tetap jalan selama swap
	s.mu.Lock()
	oldWriteDB := s.db
	oldReadDB := s.readDB
	s.db = newDB
	s.readDB = newDB
	s.mu.Unlock()

	if oldWriteDB != nil {
		oldWriteDB.Close()
	}
	if oldReadDB != nil && oldReadDB != oldWriteDB {
		oldReadDB.Close()
	}

	// rename file temp → file canonical
	dbPath := filepath.Join(dir, "leakcheck.db")
	for _, f := range []string{dbPath, dbPath + "-shm", dbPath + "-wal"} {
		os.Remove(f)
	}
	if err := os.Rename(tmpPath, dbPath); err != nil {
		log.Printf("[leakcheck] reload: rename failed (non-fatal): %v", err)
	}

	// Buka ulang koneksi dari path canonical supaya konsisten setelah restart
	// newDB masih valid (file sudah di-rename, handle tetap open di Windows/Linux)
	// tapi untuk keamanan restart, kita buka fresh connection
	freshWrite, err := openWriteDB(dbPath)
	if err == nil {
		freshRead, err2 := openReadDB(dbPath)
		if err2 == nil {
			s.mu.Lock()
			s.db = freshWrite
			s.readDB = freshRead
			s.mu.Unlock()
			newDB.Close()
		} else {
			freshWrite.Close()
		}
	}

	count = 0
	s.mu.RLock()
	s.readDB.QueryRow(`SELECT COUNT(1) FROM leakcheck`).Scan(&count)
	s.mu.RUnlock()
	log.Printf("[leakcheck] reload: done in %s, total entries: %d", time.Since(reloadStart).Round(time.Millisecond), count)
	return count, nil
}

// ----------------------------------------------------------------
// helper: buka koneksi DB
// ----------------------------------------------------------------

func openWriteDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`
		PRAGMA journal_mode=WAL;
		PRAGMA synchronous=NORMAL;
		PRAGMA cache_size=-64000;
		PRAGMA temp_store=MEMORY;
	`); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func openReadDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	if _, err := db.Exec(`
		PRAGMA journal_mode=WAL;
		PRAGMA synchronous=NORMAL;
		PRAGMA cache_size=-128000;
		PRAGMA temp_store=MEMORY;
		PRAGMA mmap_size=268435456;
	`); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func ensureSchema(db *sql.DB) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS leakcheck (
			id       INTEGER PRIMARY KEY AUTOINCREMENT,
			source   TEXT,
			soft     TEXT,
			host     TEXT,
			login    TEXT,
			password TEXT,
			UNIQUE(login, password)
		);
	`); err != nil {
		return err
	}

	if _, err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_leakcheck_login ON leakcheck(lower(login));
		CREATE INDEX IF NOT EXISTS idx_leakcheck_host  ON leakcheck(lower(host));
	`); err != nil {
		return err
	}

	if _, err := db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS leakcheck_fts
		USING fts5(login, content='leakcheck', content_rowid='id');
	`); err != nil {
		return err
	}

	if _, err := db.Exec(`
		CREATE TRIGGER IF NOT EXISTS leakcheck_ai AFTER INSERT ON leakcheck BEGIN
			INSERT INTO leakcheck_fts(rowid, login)
			VALUES (new.id, new.login);
		END;
	`); err != nil {
		return err
	}

	if _, err := db.Exec(`
		CREATE TRIGGER IF NOT EXISTS leakcheck_ad AFTER DELETE ON leakcheck BEGIN
			INSERT INTO leakcheck_fts(leakcheck_fts, rowid, login)
			VALUES ('delete', old.id, old.login);
		END;
	`); err != nil {
		return err
	}

	return nil
}

// ----------------------------------------------------------------
// buildDatabase — untuk Reload (full rebuild ke file baru)
// ----------------------------------------------------------------

func buildDatabase(dbPath, dir string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", fmt.Sprintf("%s?_page_size=8192", dbPath))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(`
		PRAGMA journal_mode=OFF;
		PRAGMA synchronous=OFF;
		PRAGMA cache_size=-512000;
		PRAGMA temp_store=MEMORY;
		PRAGMA locking_mode=EXCLUSIVE;
		PRAGMA mmap_size=1073741824;
	`); err != nil {
		db.Close()
		return nil, err
	}

	// Tidak pakai UNIQUE constraint saat build — dedup dilakukan di Go
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS leakcheck (
			id       INTEGER PRIMARY KEY AUTOINCREMENT,
			source   TEXT,
			soft     TEXT,
			host     TEXT,
			login    TEXT,
			password TEXT
		);
	`); err != nil {
		db.Close()
		return nil, err
	}

	if _, err := db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS leakcheck_fts
		USING fts5(login, content='leakcheck', content_rowid='id');
	`); err != nil {
		db.Close()
		return nil, err
	}

	// insert semua data dulu, trigger dipasang setelah FTS rebuild
	inserted, err := insertEntriesFromDirDedup(dir, db)
	if err != nil {
		db.Close()
		return nil, err
	}

	if _, err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_leakcheck_login ON leakcheck(lower(login));
		CREATE INDEX IF NOT EXISTS idx_leakcheck_host  ON leakcheck(lower(host));
	`); err != nil {
		log.Printf("[leakcheck] index creation warning: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO leakcheck_fts(leakcheck_fts) VALUES('rebuild')`); err != nil {
		log.Printf("[leakcheck] fts rebuild warning: %v", err)
	}

	if _, err := db.Exec(`
		CREATE TRIGGER IF NOT EXISTS leakcheck_ai AFTER INSERT ON leakcheck BEGIN
			INSERT INTO leakcheck_fts(rowid, login)
			VALUES (new.id, new.login);
		END;
	`); err != nil {
		db.Close()
		return nil, err
	}

	if _, err := db.Exec(`
		CREATE TRIGGER IF NOT EXISTS leakcheck_ad AFTER DELETE ON leakcheck BEGIN
			INSERT INTO leakcheck_fts(leakcheck_fts, rowid, login)
			VALUES ('delete', old.id, old.login);
		END;
	`); err != nil {
		db.Close()
		return nil, err
	}

	if _, err := db.Exec(`
		PRAGMA journal_mode=WAL;
		PRAGMA synchronous=NORMAL;
		PRAGMA locking_mode=NORMAL;
		PRAGMA cache_size=-128000;
		PRAGMA mmap_size=268435456;
	`); err != nil {
		log.Printf("[leakcheck] pragma restore warning: %v", err)
	}

	log.Printf("[leakcheck] buildDatabase: inserted %d entries", inserted)
	return db, nil
}

// ----------------------------------------------------------------
// loadFromDir — untuk Init (DB kosong, load pertama kali)
// ----------------------------------------------------------------

func (s *Store) loadFromDir(dir string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadFromDirLocked(dir)
}

func (s *Store) loadFromDirLocked(dir string) error {
	// gunakan dedup untuk initial load supaya lebih cepat
	inserted, err := insertEntriesFromDirDedup(dir, s.db)
	if err != nil {
		return err
	}

	if _, err := s.db.Exec(`INSERT INTO leakcheck_fts(leakcheck_fts) VALUES('rebuild')`); err != nil {
		log.Printf("[leakcheck] fts rebuild warning: %v", err)
	} else {
		log.Printf("[leakcheck] fts index rebuilt")
	}

	log.Printf("[leakcheck] loaded %d entries from %s", inserted, dir)
	return nil
}

// ----------------------------------------------------------------
// insertEntriesFromDirDedup — build/initial load, dedup via Go map
// ----------------------------------------------------------------

func insertEntriesFromDirDedup(dir string, db *sql.DB) (int, error) {
	inserted := 0
	fileCount := 0
	skippedCount := 0
	errorCount := 0

	seen := make(map[string]struct{}, 20_000_000)

	const batchSize = 200_000
	var tx *sql.Tx
	var stmt *sql.Stmt
	batchCount := 0

	newBatch := func() error {
		var err error
		tx, err = db.Begin()
		if err != nil {
			return err
		}
		stmt, err = tx.Prepare(`INSERT INTO leakcheck (source, soft, host, login, password) VALUES (?, ?, ?, ?, ?)`)
		if err != nil {
			tx.Rollback()
			return err
		}
		return nil
	}

	commitBatch := func() error {
		if stmt != nil {
			stmt.Close()
		}
		if tx != nil {
			return tx.Commit()
		}
		return nil
	}

	if err := newBatch(); err != nil {
		return 0, err
	}

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(path), ".txt") {
			return nil
		}
		fileCount++
		entries, err := parseFile(path)
		if err != nil {
			log.Printf("[leakcheck] error parsing %s: %v", filepath.Base(path), err)
			errorCount++
			return nil
		}
		if len(entries) == 0 {
			skippedCount++
			return nil
		}
		for _, e := range entries {
			key := e.Login + "\x00" + e.Password
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}

			if _, err := stmt.Exec(e.Source, e.Soft, e.Host, e.Login, e.Password); err != nil {
				continue
			}
			inserted++
			batchCount++
			if batchCount >= batchSize {
				if err := commitBatch(); err != nil {
					return err
				}
				batchCount = 0
				if err := newBatch(); err != nil {
					return err
				}
			}
		}
		return nil
	})

	if err != nil {
		if tx != nil {
			tx.Rollback()
		}
		return inserted, err
	}

	if err := commitBatch(); err != nil {
		return inserted, err
	}

	log.Printf("[leakcheck] scanned %d files, skipped %d empty, %d errors, inserted %d entries (dedup)",
		fileCount, skippedCount, errorCount, inserted)
	return inserted, nil
}

// ----------------------------------------------------------------
// insertEntriesFromDirIncremental — untuk AddDir (data tambahan)
// Pakai INSERT OR IGNORE karena DB sudah punya UNIQUE constraint
// FTS diupdate otomatis via trigger — tidak perlu rebuild
// ----------------------------------------------------------------

func insertEntriesFromDirIncremental(dir string, db *sql.DB) (int, error) {
	inserted := 0
	fileCount := 0
	skippedCount := 0
	errorCount := 0

	const batchSize = 10_000 // lebih kecil karena trigger FTS jalan per-row
	var tx *sql.Tx
	var stmt *sql.Stmt
	batchCount := 0

	newBatch := func() error {
		var err error
		tx, err = db.Begin()
		if err != nil {
			return err
		}
		stmt, err = tx.Prepare(`INSERT OR IGNORE INTO leakcheck (source, soft, host, login, password) VALUES (?, ?, ?, ?, ?)`)
		if err != nil {
			tx.Rollback()
			return err
		}
		return nil
	}

	commitBatch := func() error {
		if stmt != nil {
			stmt.Close()
		}
		if tx != nil {
			return tx.Commit()
		}
		return nil
	}

	if err := newBatch(); err != nil {
		return 0, err
	}

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(path), ".txt") {
			return nil
		}
		fileCount++
		entries, err := parseFile(path)
		if err != nil {
			log.Printf("[leakcheck] error parsing %s: %v", filepath.Base(path), err)
			errorCount++
			return nil
		}
		if len(entries) == 0 {
			skippedCount++
			return nil
		}
		for _, e := range entries {
			if _, err := stmt.Exec(e.Source, e.Soft, e.Host, e.Login, e.Password); err != nil {
				continue
			}
			inserted++
			batchCount++
			if batchCount >= batchSize {
				if err := commitBatch(); err != nil {
					return err
				}
				batchCount = 0
				if err := newBatch(); err != nil {
					return err
				}
			}
		}
		return nil
	})

	if err != nil {
		if tx != nil {
			tx.Rollback()
		}
		return inserted, err
	}

	if err := commitBatch(); err != nil {
		return inserted, err
	}

	log.Printf("[leakcheck] incremental: scanned %d files, skipped %d empty, %d errors, inserted %d new entries",
		fileCount, skippedCount, errorCount, inserted)
	return inserted, nil
}

// ----------------------------------------------------------------
// parser
// ----------------------------------------------------------------

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

func tokenizeEmail(s string) []string {
	var tokens []string
	var current strings.Builder

	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			current.WriteRune(r)
		} else {
			if current.Len() >= 3 {
				tokens = append(tokens, current.String())
			}
			current.Reset()
		}
	}
	if current.Len() >= 3 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func sanitizeFTSQuery(q string) string {
	replacer := strings.NewReplacer(
		`"`, ``,
		`*`, ``,
		`(`, ``,
		`)`, ``,
		`^`, ``,
		`{`, ``,
		`}`, ``,
		`[`, ``,
		`]`, ``,
		`:`, ``,
		`+`, ``,
	)
	return strings.TrimSpace(replacer.Replace(q))
}

// ----------------------------------------------------------------
// utility
// ----------------------------------------------------------------

func (s *Store) Ping() error {
	s.mu.RLock()
	db := s.readDB
	s.mu.RUnlock()

	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	return db.Ping()
}

func (s *Store) Count() int {
	s.mu.RLock()
	db := s.readDB
	s.mu.RUnlock()

	if db == nil {
		return 0
	}
	count := 0
	db.QueryRow(`SELECT COUNT(1) FROM leakcheck`).Scan(&count)
	return count
}
