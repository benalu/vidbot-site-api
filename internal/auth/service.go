package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// Token berlaku 5 menit, window per 5 menit (bukan sliding)
func GenerateToken(apiKey, magicString string) string {
	window := time.Now().Unix() / 300
	raw := fmt.Sprintf("%s:%s:%d", apiKey, magicString, window)
	h := hmac.New(sha256.New, []byte(magicString))
	h.Write([]byte(raw))
	return hex.EncodeToString(h.Sum(nil))
}

func ValidateToken(token, apiKey, magicString string) bool {
	// cek window sekarang dan window sebelumnya (toleransi di ujung window)
	for _, offset := range []int64{0, -1} {
		window := time.Now().Unix()/300 + offset
		raw := fmt.Sprintf("%s:%s:%d", apiKey, magicString, window)
		h := hmac.New(sha256.New, []byte(magicString))
		h.Write([]byte(raw))
		expected := hex.EncodeToString(h.Sum(nil))
		if hmac.Equal([]byte(token), []byte(expected)) {
			return true
		}
	}
	return false
}
