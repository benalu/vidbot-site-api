package middleware

import (
	"vidbot-api/internal/admin"
	"vidbot-api/pkg/response"

	"github.com/gin-gonic/gin"
)

func RequireAdminAuth(masterKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if mk := c.GetHeader("X-Master-Key"); mk != "" {
			if mk == masterKey {
				c.Next()
				return
			}
			response.Abort(c, response.ErrAdminUnauthorized)
			return
		}

		if token := c.GetHeader("X-Admin-Session"); token != "" {
			sessionData, err := admin.ValidateAdminSession(token)
			if err != nil {
				response.Abort(c, response.ErrAdminSessionExpired)
				return
			}
			c.Set("admin_session", sessionData)
			c.Next()
			return
		}

		response.Abort(c, response.ErrAdminUnauthorized)
	}
}
