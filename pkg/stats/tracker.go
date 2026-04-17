package stats

import (
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
		return
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO stats (grp, platform, key_hash) VALUES (?, ?, ?)`)
	if err != nil {
		return
	}
	defer stmt.Close()

	for _, e := range events {
		stmt.Exec(e.group, e.platform, e.keyHash)
	}

	tx.Commit()
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
