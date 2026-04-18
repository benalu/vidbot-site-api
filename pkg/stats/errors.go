package stats

import (
	"log/slog"
	"time"
	"vidbot-api/pkg/apikey"

	"github.com/gin-gonic/gin"
)

type errorEvent struct {
	group    string
	platform string
	code     string
	keyHash  string
}

var errorCh = make(chan errorEvent, 2000)

func init() {
	go errorWorker()
}

func errorWorker() {
	batch := make([]errorEvent, 0, 100)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case e := <-errorCh:
			batch = append(batch, e)
			if len(batch) >= 100 {
				flushErrors(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				flushErrors(batch)
				batch = batch[:0]
			}
		}
	}
}

func flushErrors(events []errorEvent) {
	if DB == nil || len(events) == 0 {
		return
	}

	tx, err := DB.Begin()
	if err != nil {
		slog.Warn("errors flush begin failed", "error", err)
		return
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO errors (grp, platform, code, key_hash)
		VALUES ($1, $2, $3, $4)
	`)
	if err != nil {
		slog.Warn("errors flush prepare failed", "error", err)
		return
	}
	defer stmt.Close()

	for _, e := range events {
		stmt.Exec(e.group, e.platform, e.code, e.keyHash)
	}

	tx.Commit()
}

// TrackError — dipanggil di handler saat terjadi error
func TrackError(c *gin.Context, group, platform, code string) {
	keyHash := ""
	if data, exists := c.Get("api_key_data"); exists {
		if keyData, ok := data.(apikey.Data); ok {
			keyHash = keyData.KeyHash
		}
	}
	select {
	case errorCh <- errorEvent{group, platform, code, keyHash}:
	default:
	}
}

func GetErrorStats(group, platform string, hours int) []map[string]interface{} {
	if DB == nil {
		return nil
	}
	rows, err := DB.Query(`
		SELECT code, COUNT(*) as count
		FROM errors
		WHERE grp = $1
		  AND ($2 = '' OR platform = $2)
		  AND created_at >= NOW() - ($3 || ' hours')::INTERVAL
		GROUP BY code
		ORDER BY count DESC
	`, group, platform, hours)
	if err != nil {
		return nil
	}
	defer rows.Close()

	result := []map[string]interface{}{}
	for rows.Next() {
		var code string
		var count int
		rows.Scan(&code, &count)
		result = append(result, map[string]interface{}{
			"code":  code,
			"count": count,
		})
	}
	return result
}

func GetRecentErrors(limit int) []map[string]interface{} {
	if DB == nil {
		return nil
	}
	rows, err := DB.Query(`
		SELECT grp, platform, code, key_hash, created_at
		FROM errors
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	result := []map[string]interface{}{}
	for rows.Next() {
		var grp, platform, code, keyHash, createdAt string
		rows.Scan(&grp, &platform, &code, &keyHash, &createdAt)
		result = append(result, map[string]interface{}{
			"group":      grp,
			"platform":   platform,
			"code":       code,
			"key_hash":   keyHash,
			"created_at": createdAt,
		})
	}
	return result
}
