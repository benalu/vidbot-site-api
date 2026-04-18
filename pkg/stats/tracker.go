package stats

import (
	"log/slog"
	"time"
	"vidbot-api/pkg/apikey"

	"github.com/gin-gonic/gin"
)

type trackEvent struct {
	group    string
	platform string
	keyHash  string
}

var trackCh = make(chan trackEvent, 2000)

func init() {
	go batchWorker()
	go cleanupScheduler()
}

func batchWorker() {
	batch := make([]trackEvent, 0, 100)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case e := <-trackCh:
			batch = append(batch, e)
			if len(batch) >= 100 {
				flushBatch(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				flushBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

func flushBatch(events []trackEvent) {
	if DB == nil || len(events) == 0 {
		return
	}

	tx, err := DB.Begin()
	if err != nil {
		slog.Warn("stats flush begin failed", "error", err)
		return
	}
	defer tx.Rollback()

	// PostgreSQL pakai $1, $2, $3 bukan ?, ?, ?
	stmt, err := tx.Prepare(`INSERT INTO stats (grp, platform, key_hash) VALUES ($1, $2, $3)`)
	if err != nil {
		slog.Warn("stats flush prepare failed", "error", err)
		return
	}
	defer stmt.Close()

	for _, e := range events {
		stmt.Exec(e.group, e.platform, e.keyHash)
	}

	tx.Commit()
}

// cleanupScheduler — auto cleanup data > 90 hari, jalan setiap hari jam 02:00
func cleanupScheduler() {
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 2, 0, 0, 0, now.Location())
		time.Sleep(time.Until(next))
		Cleanup(90)
	}
}

func Platform(c *gin.Context, group, platform string) {
	data, exists := c.Get("api_key_data")
	if !exists {
		return
	}
	keyData, ok := data.(apikey.Data)
	if !ok {
		return
	}
	select {
	case trackCh <- trackEvent{group, platform, keyData.KeyHash}:
	default:
	}
}

func Group(c *gin.Context, group string) {
	Platform(c, group, "")
}
