package fileutil

import (
	"regexp"
	"strings"
)

// Sanitize membersihkan nama file dari karakter yang tidak aman.
// Maksimal 200 karakter. Digunakan di semua service vidhub dan content.
func Sanitize(name string) string {
	name = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`).ReplaceAllString(name, "_")
	name = strings.TrimSpace(name)
	if len(name) > 200 {
		name = name[:200]
	}
	return name
}

// SanitizeWithExt membersihkan nama file dan memastikan ekstensi tetap ada.
// Contoh: SanitizeWithExt("my:video", "mp4") → "my_video.mp4"
func SanitizeWithExt(name, ext string) string {
	suffix := "." + ext

	// lepas ekstensi dulu kalau sudah ada
	if strings.HasSuffix(strings.ToLower(name), suffix) {
		name = name[:len(name)-len(suffix)]
	}

	// sanitize karakter tidak aman (subset ketat untuk filename di header)
	var result []rune
	for _, r := range name {
		if r > 127 || r == '"' || r == '/' || r == '\\' || r == ':' ||
			r == '*' || r == '?' || r == '<' || r == '>' || r == '|' {
			continue
		}
		result = append(result, r)
	}

	s := strings.TrimSpace(string(result))
	if len(s) > 80 {
		s = s[:80]
	}
	if s == "" {
		s = "vidbot_download"
	}
	return s + suffix
}
