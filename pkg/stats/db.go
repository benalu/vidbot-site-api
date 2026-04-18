package stats

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var DB *sql.DB

func Init(dsn string) error {
	if dsn == "" {
		dsn = os.Getenv("STATS_DB_DSN")
	}
	if dsn == "" {
		dsn = "host=localhost port=5432 user=postgres password=postgres dbname=vidbot_stats sslmode=disable"
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("stats: open db: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return fmt.Errorf("stats: ping failed: %w", err)
	}

	if err := migrate(db); err != nil {
		return fmt.Errorf("stats: migrate: %w", err)
	}
	ensurePartitionExists(db)

	DB = db
	slog.Info("stats db connected (postgres)", "dsn_hint", maskDSN(dsn))
	return nil
}

func migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS stats (
            id         BIGSERIAL,
            grp        TEXT        NOT NULL,
            platform   TEXT,
            key_hash   TEXT        NOT NULL,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        ) PARTITION BY RANGE (created_at)`,

		`CREATE TABLE IF NOT EXISTS stats_2026_04
            PARTITION OF stats
            FOR VALUES FROM ('2026-04-01') TO ('2026-05-01')`,

		`CREATE TABLE IF NOT EXISTS stats_2026_05
            PARTITION OF stats
            FOR VALUES FROM ('2026-05-01') TO ('2026-06-01')`,

		`CREATE INDEX IF NOT EXISTS idx_stats_grp      ON stats(grp)`,
		`CREATE INDEX IF NOT EXISTS idx_stats_platform ON stats(grp, platform)`,
		`CREATE INDEX IF NOT EXISTS idx_stats_key      ON stats(key_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_stats_time     ON stats(created_at)`,

		// errors — letakkan di sini, setelah semua stats
		`CREATE TABLE IF NOT EXISTS errors (
            id         BIGSERIAL PRIMARY KEY,
            grp        TEXT        NOT NULL,
            platform   TEXT,
            code       TEXT        NOT NULL,
            key_hash   TEXT,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        )`,

		`CREATE INDEX IF NOT EXISTS idx_errors_grp      ON errors(grp)`,
		`CREATE INDEX IF NOT EXISTS idx_errors_platform ON errors(grp, platform)`,
		`CREATE INDEX IF NOT EXISTS idx_errors_code     ON errors(code)`,
		`CREATE INDEX IF NOT EXISTS idx_errors_time     ON errors(created_at)`,
	}

	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("migrate: %w (stmt: %.60s...)", err, s)
		}
	}
	return nil
}

func ensurePartitionExists(db *sql.DB) {
	// buat partisi untuk bulan depan kalau belum ada
	next := time.Now().AddDate(0, 1, 0)
	tableName := fmt.Sprintf("stats_%s", next.Format("2006_01"))
	start := fmt.Sprintf("%s-01", next.Format("2006-01"))
	end := fmt.Sprintf("%s-01", next.AddDate(0, 1, 0).Format("2006-01"))

	db.Exec(fmt.Sprintf(`
        CREATE TABLE IF NOT EXISTS %s 
        PARTITION OF stats 
        FOR VALUES FROM ('%s') TO ('%s')
    `, tableName, start, end))
}

func GetGroupStats(group string) (totalRequests int, uniqueKeys int) {
	if DB == nil {
		return
	}
	DB.QueryRow(
		`SELECT COUNT(*), COUNT(DISTINCT key_hash) FROM stats WHERE grp = $1`, group,
	).Scan(&totalRequests, &uniqueKeys)
	return
}

func GetPlatformStats(group, platform string) (totalRequests int, uniqueKeys int) {
	if DB == nil {
		return
	}
	DB.QueryRow(
		`SELECT COUNT(*), COUNT(DISTINCT key_hash) FROM stats WHERE grp = $1 AND platform = $2`,
		group, platform,
	).Scan(&totalRequests, &uniqueKeys)
	return
}

func GetKeyUsageByGroup(keyHash string) map[string]int {
	if DB == nil {
		return map[string]int{}
	}
	rows, err := DB.Query(
		`SELECT grp, COUNT(*) FROM stats WHERE key_hash = $1 GROUP BY grp`, keyHash,
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

func GetTodayStats(group string) (totalRequests int) {
	if DB == nil {
		return
	}
	DB.QueryRow(
		`SELECT COUNT(*) FROM stats WHERE grp = $1 AND created_at >= CURRENT_DATE`,
		group,
	).Scan(&totalRequests)
	return
}

func GetDailyStats(group string, days int) []map[string]interface{} {
	if DB == nil {
		return nil
	}
	rows, err := DB.Query(`
		SELECT DATE(created_at) as date, COUNT(*) as count
		FROM stats
		WHERE grp = $1
		  AND created_at >= NOW() - ($2 || ' days')::INTERVAL
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`, group, days)
	if err != nil {
		return nil
	}
	defer rows.Close()

	result := []map[string]interface{}{}
	for rows.Next() {
		var date string
		var count int
		rows.Scan(&date, &count)
		result = append(result, map[string]interface{}{"date": date, "count": count})
	}
	return result
}

func GetHourlyStats(group string) []map[string]interface{} {
	if DB == nil {
		return nil
	}
	rows, err := DB.Query(`
		SELECT TO_CHAR(created_at, 'HH24') as hour, COUNT(*) as count
		FROM stats
		WHERE grp = $1
		  AND created_at >= CURRENT_DATE
		GROUP BY hour
		ORDER BY hour ASC
	`, group)
	if err != nil {
		return nil
	}
	defer rows.Close()

	result := []map[string]interface{}{}
	for rows.Next() {
		var hour string
		var count int
		rows.Scan(&hour, &count)
		result = append(result, map[string]interface{}{"hour": hour, "count": count})
	}
	return result
}

func GetTopKeys(limit int) []map[string]interface{} {
	if DB == nil {
		return nil
	}
	rows, err := DB.Query(`
		SELECT key_hash, COUNT(*) as count
		FROM stats
		GROUP BY key_hash
		ORDER BY count DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	result := []map[string]interface{}{}
	for rows.Next() {
		var keyHash string
		var count int
		rows.Scan(&keyHash, &count)
		result = append(result, map[string]interface{}{
			"key_hash": keyHash,
			"count":    count,
		})
	}
	return result
}

func Cleanup(days int) {
	if DB == nil {
		return
	}
	res, err := DB.Exec(
		`DELETE FROM stats WHERE created_at < NOW() - ($1 || ' days')::INTERVAL`,
		days,
	)
	if err != nil {
		slog.Warn("stats cleanup failed", "error", err)
		return
	}
	n, _ := res.RowsAffected()
	slog.Info("stats cleanup done", "deleted", n, "older_than_days", days)
}

func maskDSN(dsn string) string {
	if len(dsn) > 30 {
		return dsn[:20] + "..."
	}
	return dsn
}
