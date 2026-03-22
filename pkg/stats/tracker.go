package stats

import (
	"vidbot-api/pkg/apikey"

	"github.com/gin-gonic/gin"
)

func Platform(c *gin.Context, group, platform string) {
	data, exists := c.Get("api_key_data")
	if !exists {
		return
	}
	keyData, ok := data.(apikey.Data)
	if !ok {
		return
	}
	Track(group, platform, keyData.KeyHash)
}

func Group(c *gin.Context, group string) {
	Platform(c, group, "")
}
