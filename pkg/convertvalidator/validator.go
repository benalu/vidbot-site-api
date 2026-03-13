package convertvalidator

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Category string

const (
	Audio    Category = "audio"
	Document Category = "document"
	Image    Category = "image"
	Fonts    Category = "fonts"
)

var maxSizes = map[Category]int64{
	Audio:    100 * 1024 * 1024, // 100MB
	Document: 100 * 1024 * 1024, // 100MB
	Image:    50 * 1024 * 1024,  // 50MB
	Fonts:    50 * 1024 * 1024,  // 50MB
}

var allowedContentTypes = map[Category][]string{
	Audio: {
		"audio/mpeg", "audio/mp3", "audio/wav", "audio/wave",
		"audio/flac", "audio/aac", "audio/ogg", "audio/m4a",
		"audio/mp4", "audio/x-m4a", "audio/x-wav",
		"audio/x-flac", "audio/opus", "application/octet-stream",
		"audio/x-ms-wma", // wma
		"audio/amr",      // amr
		"audio/ac3",      // ac3
	},
	Document: {
		"application/pdf",
		"application/msword",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.ms-excel",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.ms-powerpoint",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"application/vnd.oasis.opendocument.text", // odt
		"application/rtf", "text/rtf", // rtf
		"text/plain",                  // txt, md
		"text/html",                   // html
		"text/markdown",               // md
		"application/zip",             // docx/xlsx/pptx kadang terdeteksi zip
		"application/vnd.ms-excel",    // xls
		"text/csv", "application/csv", // csv
		"application/vnd.ms-powerpoint", // ppt
		"application/vnd.ms-works",      // wps
		"application/msword",            // doc
		"application/octet-stream",
	},
	Image: {
		"image/jpeg", "image/jpg", "image/png",
		"image/webp", "image/gif", "image/svg+xml",
		"image/avif",                               // avif
		"image/bmp",                                // bmp
		"image/x-icon", "image/vnd.microsoft.icon", // ico
		"image/jfif", "image/pjpeg", // jfif
		"image/tiff",                // tiff
		"image/vnd.adobe.photoshop", // psd
		"image/x-fuji-raf",          // raf
		"image/x-minolta-mrw",       // mrw
		"image/heic", "image/heif",  // heic/heif
		"application/postscript", // eps
		"image/x-eps",
		"image/x-raw", // raw
		"application/octet-stream",
	},
	Fonts: {
		"font/ttf", "font/otf", "font/woff", "font/woff2",
		"application/font-woff", "application/font-woff2",
		"application/x-font-ttf", "application/x-font-otf",
		"application/vnd.ms-fontobject", // eot
		"application/octet-stream",
	},
}

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	},
}

type ValidationError struct {
	Code    string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

func Validate(fileURL string, category Category) *ValidationError {
	req, err := http.NewRequest(http.MethodHead, fileURL, nil)
	if err != nil {
		return &ValidationError{"INVALID_URL", "Invalid URL format."}
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; VidBot/1.0)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return &ValidationError{"URL_UNREACHABLE", "File URL is not accessible."}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &ValidationError{"FILE_NOT_FOUND", fmt.Sprintf("File returned HTTP %d.", resp.StatusCode)}
	}

	// cek content-type
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	// ambil hanya mime type, buang parameter seperti charset
	if idx := strings.Index(contentType, ";"); idx != -1 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	if contentType != "" {
		allowed := allowedContentTypes[category]
		matched := false
		for _, ct := range allowed {
			if ct == contentType {
				matched = true
				break
			}
		}
		if !matched {
			return &ValidationError{
				"INVALID_CONTENT_TYPE",
				fmt.Sprintf("Content-Type '%s' is not allowed for %s conversion.", contentType, string(category)),
			}
		}
	}

	// cek ukuran
	maxSize := maxSizes[category]
	contentLength := resp.ContentLength
	if contentLength > maxSize {
		mb := maxSize / 1024 / 1024
		return &ValidationError{
			"FILE_TOO_LARGE",
			fmt.Sprintf("File exceeds maximum allowed size of %dMB for %s conversion.", mb, string(category)),
		}
	}

	return nil
}

func ValidateUpload(fileData []byte, fileSize int64, category Category) *ValidationError {
	// cek ukuran
	maxSize := maxSizes[category]
	if fileSize > maxSize {
		mb := maxSize / 1024 / 1024
		return &ValidationError{
			"FILE_TOO_LARGE",
			fmt.Sprintf("File exceeds maximum allowed size of %dMB for %s conversion.", mb, string(category)),
		}
	}

	// deteksi mime type dari bytes
	mime := http.DetectContentType(fileData)
	if idx := strings.Index(mime, ";"); idx != -1 {
		mime = strings.TrimSpace(mime[:idx])
	}

	allowed := allowedContentTypes[category]
	for _, ct := range allowed {
		if ct == mime {
			return nil
		}
	}

	return &ValidationError{
		"INVALID_CONTENT_TYPE",
		fmt.Sprintf("Content-Type '%s' is not allowed for %s conversion.", mime, string(category)),
	}
}
