package middleware

import (
	"vidbot-api/internal/auth"
	"vidbot-api/pkg/apikey"
	"vidbot-api/pkg/response"

	"github.com/gin-gonic/gin"
)

func RequireAccessToken(magicString string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("X-Access-Token")
		if token == "" {
			response.Abort(c, response.ErrAccessTokenMissing)
			return
		}

		data, exists := c.Get("api_key_data")
		if !exists {
			response.Abort(c, response.ErrAccessTokenInvalid)
			return
		}

		keyData := data.(apikey.Data)
		if !auth.ValidateToken(token, keyData.KeyHash, magicString) {
			response.Abort(c, response.ErrAccessTokenInvalid)
			return
		}

		c.Next()
	}
}
