package middleware

import (
	"net/http"
	"vidbot-api/internal/admin"

	"github.com/gin-gonic/gin"
)

func RequireAdminSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("X-Admin-Session")
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"code":    "UNAUTHORIZED",
				"message": "Session required. Please login to obtain a session token.",
			})
			return
		}
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
	}
}
