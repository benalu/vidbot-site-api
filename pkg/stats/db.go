package stats

import (
	"database/sql"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

func Init(path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}

	db.SetMaxOpenConns(1)
	_, err = db.Exec(`
		PRAGMA journal_mode=WAL;
		PRAGMA synchronous=NORMAL;
		PRAGMA cache_size=-32000;
		PRAGMA temp_store=MEMORY;
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS stats (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			grp       TEXT NOT NULL,
			platform  TEXT,
			key_hash  TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_stats_grp ON stats(grp);
		CREATE INDEX IF NOT EXISTS idx_stats_platform ON stats(grp, platform);
		CREATE INDEX IF NOT EXISTS idx_stats_key ON stats(key_hash);
		CREATE INDEX IF NOT EXISTS idx_stats_time ON stats(created_at);
	`)
	if err != nil {
		return err
	}

	DB = db
	log.Println("[statsdb] SQLite connected:", path)
	return nil
}

func Track(group, platform, keyHash string) {
	if DB == nil {
		return
	}
	DB.Exec(
		`INSERT INTO stats (grp, platform, key_hash) VALUES (?, ?, ?)`,
		group, platform, keyHash,
	)
}

func GetGroupStats(group string) (totalRequests int, uniqueKeys int) {
	if DB == nil {
		return
	}
	DB.QueryRow(
		`SELECT COUNT(*), COUNT(DISTINCT key_hash) FROM stats WHERE grp = ?`, group,
	).Scan(&totalRequests, &uniqueKeys)
	return
}

func GetPlatformStats(group, platform string) (totalRequests int, uniqueKeys int) {
	if DB == nil {
		return
	}
	DB.QueryRow(
		`SELECT COUNT(*), COUNT(DISTINCT key_hash) FROM stats WHERE grp = ? AND platform = ?`,
		group, platform,
	).Scan(&totalRequests, &uniqueKeys)
	return
}

func GetKeyUsageByGroup(keyHash string) map[string]int {
	if DB == nil {
		return map[string]int{}
	}
	rows, err := DB.Query(
		`SELECT grp, COUNT(*) FROM stats WHERE key_hash = ? GROUP BY grp`, keyHash,
	)
	if err != nil {
		return map[string]int{}
	}
	defer rows.Close()

	result := map[string]int{}
	for rows.Next() {
		var grp string
		var count int
		rows.Scan(&grp, &count)
		result[grp] = count
	}
	return result
}

// GetTodayStats — stats hari ini saja
func GetTodayStats(group string) (totalRequests int) {
	if DB == nil {
		return
	}
	DB.QueryRow(
		`SELECT COUNT(*) FROM stats WHERE grp = ? AND DATE(created_at) = DATE('now')`,
		group,
	).Scan(&totalRequests)
	return
}

// Cleanup — hapus data lebih dari N hari untuk jaga ukuran database
func Cleanup(days int) {
	if DB == nil {
		return
	}
	DB.Exec(
		`DELETE FROM stats WHERE created_at < datetime('now', ?)`,
		-days*int(time.Hour)*24,
	)
}
