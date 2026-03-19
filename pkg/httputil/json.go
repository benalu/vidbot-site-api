package httputil

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

// WriteJSON menulis response JSON tanpa HTML escaping.
// Mencegah \u0026 pada URL di dalam response JSON.
// Gunakan ini di semua handler sebagai pengganti c.JSON().
func WriteJSON(c *gin.Context, status int, data interface{}) {
	buf := &bytes.Buffer{}
	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	encoder.Encode(data)
	c.Data(status, "application/json; charset=utf-8", buf.Bytes())
}

// WriteJSONOK shorthand untuk status 200.
func WriteJSONOK(c *gin.Context, data interface{}) {
	WriteJSON(c, http.StatusOK, data)
}
