package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestLogger — structured HTTP access log untuk setiap request
// Menggantikan gin.Default() default logger yang tidak terstruktur
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// skip log untuk health check agar tidak spam
		if path == "/health" {
			c.Next()
			return
		}

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		ip := c.ClientIP()
		reqID, _ := c.Get("request_id")

		fields := []any{
			"method", method,
			"path", path,
			"status", status,
			"latency_ms", latency.Milliseconds(),
			"ip", ip,
			"request_id", reqID,
		}

		if query != "" {
			fields = append(fields, "query", query)
		}
		if len(c.Errors) > 0 {
			fields = append(fields, "errors", c.Errors.String())
		}

		// level berdasarkan status code
		switch {
		case status >= 500:
			slog.Error("request completed", fields...)
		case status >= 400:
			slog.Warn("request completed", fields...)
		default:
			slog.Info("request completed", fields...)
		}
	}
}
