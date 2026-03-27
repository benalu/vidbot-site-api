package downloader

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type Payload struct {
	URL      string `json:"url"`
	Title    string `json:"title"`
	Filename string `json:"filename"`
	Filecode string `json:"filecode"`
	Ext      string `json:"ext"`
	Service  string `json:"service"`
	Secret   string `json:"secret"`
}

var (
	encryptKey []byte
	hmacKey    []byte
)

func InitKeys(encKey, hmacKeyStr string) {
	h := sha256.Sum256([]byte(encKey))
	encryptKey = h[:]
	h2 := sha256.Sum256([]byte(hmacKeyStr))
	hmacKey = h2[:]
}

// =====================
// SERVER 2 — AES+HMAC (dipertahankan untuk DecodePayload)
// =====================

func encodePayload(payload Payload) (string, error) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.Encode(payload)
	jsonBytes := bytes.TrimSpace(buf.Bytes())

	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	if _, err := gz.Write(jsonBytes); err != nil {
		return "", err
	}
	gz.Close()

	block, err := aes.NewCipher(encryptKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	encrypted := gcm.Seal(nonce, nonce, compressed.Bytes(), nil)

	encoded := hex.EncodeToString(encrypted)

	noise := make([]byte, 3)
	rand.Read(noise)
	prefix := hex.EncodeToString(noise)[:4]
	withNoise := prefix + encoded

	mac := hmac.New(sha256.New, hmacKey)
	mac.Write([]byte(withNoise))
	sig := hex.EncodeToString(mac.Sum(nil))[:16]

	return withNoise + "." + sig, nil
}

func DecodePayload(token string) (*Payload, error) {
	// coba shortlink dulu (16 hex chars, tanpa titik)
	if len(token) == 16 && !strings.Contains(token, ".") {
		if isHex(token) {
			return resolveShortlink(token)
		}
	}

	// fallback ke AES decode lama
	return decodeAESPayload(token)
}

func resolveShortlink(key string) (*Payload, error) {
	// import dihindari circular — pakai package shortlink via interface
	// kita delegasikan ke ShortlinkResolver yang di-inject saat init
	if shortlinkResolver == nil {
		return nil, fmt.Errorf("shortlink resolver not initialized")
	}
	return shortlinkResolver(key)
}

// ShortlinkResolver — di-set dari main/router untuk hindari circular import
var shortlinkResolver func(key string) (*Payload, error)

func SetShortlinkResolver(fn func(key string) (*Payload, error)) {
	shortlinkResolver = fn
}

func decodeAESPayload(token string) (*Payload, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid token format")
	}
	data, sig := parts[0], parts[1]

	mac := hmac.New(sha256.New, hmacKey)
	mac.Write([]byte(data))
	expectedSig := hex.EncodeToString(mac.Sum(nil))[:16]
	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return nil, fmt.Errorf("invalid signature")
	}

	if len(data) < 4 {
		return nil, fmt.Errorf("token too short")
	}
	encoded := data[4:]

	encrypted, err := hex.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode error: %w", err)
	}

	block, err := aes.NewCipher(encryptKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(encrypted) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := encrypted[:nonceSize], encrypted[nonceSize:]
	compressed, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt error: %w", err)
	}

	gr, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("decompress error: %w", err)
	}
	defer gr.Close()
	jsonBytes, err := io.ReadAll(gr)
	if err != nil {
		return nil, fmt.Errorf("read decompress: %w", err)
	}

	var p Payload
	if err := json.Unmarshal(jsonBytes, &p); err != nil {
		return nil, fmt.Errorf("json decode: %w", err)
	}

	return &p, nil
}

func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// =====================
// SERVER 1 — XOR + hex
// =====================

func xorEncrypt(data []byte, key string) []byte {
	result := make([]byte, len(data))
	for i, b := range data {
		result[i] = b ^ key[i%len(key)] ^ byte(i&0xff)
	}
	return result
}

func encodeWorkerPayload(payload Payload, xorKey string) string {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.Encode(payload)
	jsonBytes := bytes.TrimSpace(buf.Bytes())
	encrypted := xorEncrypt(jsonBytes, xorKey)
	return hex.EncodeToString(encrypted)
}

// =====================
// GENERATE URLs
// =====================

func GenerateServer1URL(downloadWorkerURL, downloadWorkerSecret, xorKey, downloadURL, title, filename, filecode, ext, service string) string {
	encoded := encodeWorkerPayload(Payload{
		URL:      downloadURL,
		Title:    title,
		Filename: filename,
		Filecode: filecode,
		Ext:      ext,
		Service:  service,
		Secret:   downloadWorkerSecret,
	}, xorKey)
	base := strings.TrimRight(downloadWorkerURL, "/")
	return fmt.Sprintf("%s/dl?url=%s", base, encoded)
}

// GenerateServer2URL — pakai shortlink Redis, idempoten via cacheKey
// BARU
func GenerateServer2URL(appURL, streamSecret, cacheKey, downloadURL, title, filename, filecode, ext, service string) string {
	payload := Payload{
		URL:      downloadURL,
		Title:    title,
		Filename: filename,
		Filecode: filecode,
		Ext:      ext,
		Service:  service,
		Secret:   streamSecret,
	}

	if shortlinkCreator != nil {
		key, err := shortlinkCreator(payload, cacheKey)
		if err == nil {
			base := strings.TrimRight(appURL, "/")
			return fmt.Sprintf("%s/dl?url=%s", base, key)
		}
	}

	// fallback ke AES encode
	encoded, err := encodePayload(payload)
	if err != nil {
		return ""
	}
	base := strings.TrimRight(appURL, "/")
	return fmt.Sprintf("%s/dl?url=%s", base, encoded)
}

// update signature ShortlinkCreator
var shortlinkCreator func(payload Payload, cacheKey string) (string, error)

func SetShortlinkCreator(fn func(payload Payload, cacheKey string) (string, error)) {
	shortlinkCreator = fn
}

func ResolveFilename(title, filename, filecode, ext string) string {
	name := ""
	if strings.TrimSpace(title) != "" {
		name = strings.TrimSpace(title)
	} else if strings.TrimSpace(filename) != "" {
		name = filename
		for _, e := range []string{".mp4", ".mkv", ".avi", ".mov", ".webm", ".mp3", ".m4a"} {
			if strings.HasSuffix(strings.ToLower(name), e) {
				name = name[:len(name)-len(e)]
				break
			}
		}
	} else {
		name = filecode
	}
	return fmt.Sprintf("vidbot_%s.%s", name, ext)
}
