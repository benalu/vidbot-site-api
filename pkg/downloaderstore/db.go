package downloaderstore

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
	"flac":   true,
	"movies": true,
	// tambah platform lain di sini nanti: "ebooks", "anime", "movies", "games", "courses"
}

type platformDB struct {
	write *sql.DB
	read  *sql.DB
}

var dbs = map[string]*platformDB{}

func getWriteDB(platform string) (*sql.DB, error) {
	p, ok := dbs[platform]
	if !ok || p == nil {
		return nil, fmt.Errorf("downloaderstore: platform '%s' not initialized", platform)
	}
	return p.write, nil
}

func getReadDB(platform string) (*sql.DB, error) {
	p, ok := dbs[platform]
	if !ok || p == nil {
		return nil, fmt.Errorf("downloaderstore: platform '%s' not initialized", platform)
	}
	return p.read, nil
}

func IsValidPlatform(platform string) bool {
	return validPlatforms[platform]
}

func Init(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("downloaderstore: mkdir: %w", err)
	}
	for platform := range validPlatforms {
		if err := initPlatform(dir, platform); err != nil {
			return err
		}
	}
	return nil
}

func initPlatform(dir, platform string) error {
	path := filepath.Join(dir, platform+".db")

	writeDB, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("downloaderstore: open writeDB %s: %w", platform, err)
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
			return fmt.Errorf("downloaderstore: pragma writeDB %s [%s]: %w", platform, pragma, err)
		}
	}

	migrateFn, ok := migrators[platform]
	if !ok {
		return fmt.Errorf("downloaderstore: no migrator for platform '%s'", platform)
	}
	if err := migrateFn(writeDB); err != nil {
		return fmt.Errorf("downloaderstore: migrate %s: %w", platform, err)
	}

	readDB, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("downloaderstore: open readDB %s: %w", platform, err)
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
			return fmt.Errorf("downloaderstore: pragma readDB %s [%s]: %w", platform, pragma, err)
		}
	}

	dbs[platform] = &platformDB{write: writeDB, read: readDB}

	// WAL checkpoint goroutine
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

	var count int
	readDB.QueryRow(`SELECT COUNT(*) FROM ` + platformTableName(platform)).Scan(&count)
	slog.Info("downloaderstore db ready", "platform", platform, "entries", count)
	return nil
}

// migrators — map platform → fungsi migrasi schema-nya
var migrators = map[string]func(*sql.DB) error{
	"flac":   migrateFlac,
	"movies": migrateMovies,
}

func platformTableName(platform string) string {
	switch platform {
	case "flac":
		return "flac_entries"
	case "movies":
		return "movies_entries"
	default:
		return platform + "_entries"
	}
}

// sanitizeFTS — hapus karakter khusus FTS5 dari query
func sanitizeFTS(q string) string {
	replacer := strings.NewReplacer(
		`"`, ``, `*`, ``, `(`, ``, `)`, ``,
		`^`, ``, `{`, ``, `}`, ``, `[`, ``,
		`]`, ``, `:`, ``, `+`, ``,
	)
	return strings.TrimSpace(replacer.Replace(q))
}
