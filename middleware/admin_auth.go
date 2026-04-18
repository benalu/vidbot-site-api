package middleware

import (
	"net/http"
	"vidbot-api/internal/admin"

	"github.com/gin-gonic/gin"
)

func RequireAdminAuth(masterKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if mk := c.GetHeader("X-Master-Key"); mk != "" {
			if mk == masterKey {
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"code":    "UNAUTHORIZED",
				"message": "Invalid master key.",
			})
			return
		}

		if token := c.GetHeader("X-Admin-Session"); token != "" {
			sessionData, err := admin.ValidateAdminSession(token)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"success": false,
					"code":    "SESSION_EXPIRED",
					"message": "Session expired or invalid. Please login again.",
				})
				return
			}
			c.Set("admin_session", sessionData)
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"code":    "UNAUTHORIZED",
			"message": "Authentication required. Use X-Master-Key or X-Admin-Session.",
		})
	}
}
