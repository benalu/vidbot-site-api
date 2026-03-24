package stats

import (
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
	go func() {
		for e := range trackCh {
			Track(e.group, e.platform, e.keyHash)
		}
	}()
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
