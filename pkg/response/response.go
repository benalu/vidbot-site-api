package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ─── Response types ───────────────────────────────────────────────────────────

type SuccessResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
}

type ErrorResponse struct {
	Success bool   `json:"success"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ─── Low-level helpers (backward compatible) ──────────────────────────────────
// Dipertahankan agar kode lama tidak break selama proses migration.
// Untuk kode baru, gunakan Write/WriteMsg/Abort/AbortMsg di helpers.go.

// Error menulis error response, code di-derive otomatis dari HTTP status.
func Error(c *gin.Context, status int, message string) {
	c.JSON(status, ErrorResponse{
		Success: false,
		Code:    httpStatusToCode(status),
		Message: message,
	})
}

// ErrorWithCode menulis error response dengan code eksplisit.
func ErrorWithCode(c *gin.Context, status int, code, message string) {
	c.JSON(status, ErrorResponse{
		Success: false,
		Code:    code,
		Message: message,
	})
}

// WriteJSON menulis success response.
func WriteJSON(c *gin.Context, status int, data interface{}) {
	c.JSON(status, data)
}

// ─── Internal ─────────────────────────────────────────────────────────────────

func httpStatusToCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "BAD_REQUEST"
	case http.StatusUnauthorized:
		return "UNAUTHORIZED"
	case http.StatusForbidden:
		return "FORBIDDEN"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusTooManyRequests:
		return "RATE_LIMITED"
	case http.StatusInternalServerError:
		return "INTERNAL_ERROR"
	default:
		return "ERROR"
	}
}
