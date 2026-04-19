package response

import "github.com/gin-gonic/gin"

// ─── APIError helpers ─────────────────────────────────────────────────────────
// Gunakan fungsi-fungsi ini untuk kode baru.
// Semua menerima APIError dari katalog di errors.go.

// Write menulis error response. Gunakan di handler (tidak stop chain).
//
// Contoh: response.Write(c, response.ErrBadRequest)
func Write(c *gin.Context, e APIError) {
	c.JSON(e.HTTPStatus, ErrorResponse{
		Success: false,
		Code:    e.Code,
		Message: e.Message,
	})
}

// WriteMsg sama seperti Write tapi override pesan default.
// Gunakan kalau pesan perlu lebih spesifik, terutama di admin handler.
//
// Contoh: response.WriteMsg(c, response.ErrAdminBadRequest, "name wajib diisi.")
func WriteMsg(c *gin.Context, e APIError, msg string) {
	c.JSON(e.HTTPStatus, ErrorResponse{
		Success: false,
		Code:    e.Code,
		Message: msg,
	})
}

// Abort menulis error response dan stop middleware chain.
// Gunakan di middleware, bukan di handler.
//
// Contoh: response.Abort(c, response.ErrUnauthorized)
func Abort(c *gin.Context, e APIError) {
	c.AbortWithStatusJSON(e.HTTPStatus, ErrorResponse{
		Success: false,
		Code:    e.Code,
		Message: e.Message,
	})
}

// AbortMsg sama seperti Abort tapi override pesan default.
//
// Contoh: response.AbortMsg(c, response.ErrServiceUnavailable, "The 'spotify' service is temporarily unavailable.")
func AbortMsg(c *gin.Context, e APIError, msg string) {
	c.AbortWithStatusJSON(e.HTTPStatus, ErrorResponse{
		Success: false,
		Code:    e.Code,
		Message: msg,
	})
}
