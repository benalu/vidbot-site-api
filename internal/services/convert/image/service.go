package image

import (
	"fmt"
	"strings"
	"vidbot-api/internal/services/convert/provider"
)

var allowedFormats = map[string]bool{
	"jpg": true, "jpeg": true, "png": true,
	"webp": true, "gif": true, "avif": true,
	"bmp": true, "ico": true, "jfif": true,
	"tiff": true, "psd": true, "raf": true,
	"mrw": true, "heic": true, "heif": true,
	"eps": true, "svg": true, "raw": true,
}

// format yang bisa dikonversi ke format lain
var formatCompatibility = map[string][]string{
	"jpg":  {"png", "webp", "gif", "avif", "bmp", "ico", "tiff", "jfif", "eps", "svg"},
	"jpeg": {"png", "webp", "gif", "avif", "bmp", "ico", "tiff", "jfif", "eps", "svg"},
	"png":  {"jpg", "webp", "gif", "avif", "bmp", "ico", "tiff", "eps", "svg"},
	"webp": {"jpg", "png", "gif", "avif", "bmp", "tiff"},
	"gif":  {"jpg", "png", "webp", "avif", "bmp", "tiff"},
	"avif": {"jpg", "png", "webp", "gif", "bmp", "tiff"},
	"bmp":  {"jpg", "png", "webp", "gif", "avif", "tiff", "ico"},
	"ico":  {"jpg", "png", "webp", "bmp"},
	"jfif": {"jpg", "png", "webp", "gif", "bmp", "tiff"},
	"tiff": {"jpg", "png", "webp", "gif", "avif", "bmp", "eps", "pdf"},
	"psd":  {"jpg", "png", "webp", "gif", "bmp", "tiff", "eps"},
	"heic": {"jpg", "png", "webp", "gif", "avif", "tiff"},
	"heif": {"jpg", "png", "webp", "gif", "avif", "tiff"},
	"eps":  {"jpg", "png", "webp", "gif", "bmp", "tiff", "svg"},
	"svg":  {"jpg", "png", "webp", "gif", "bmp", "tiff", "eps"},
	"raw":  {"jpg", "png", "webp", "tiff", "bmp"},
	"raf":  {"jpg", "png", "webp", "tiff", "bmp"},
	"mrw":  {"jpg", "png", "webp", "tiff", "bmp"},
}

type Service struct {
	providers []provider.Provider
}

func NewService(providers []provider.Provider) *Service {
	return &Service{providers: providers}
}

func (s *Service) validateCompatibility(from, to string) error {
	compatible, exists := formatCompatibility[from]
	if !exists {
		return fmt.Errorf("unknown source format: %s", from)
	}
	for _, f := range compatible {
		if f == to {
			return nil
		}
	}
	return fmt.Errorf("conversion from %s to %s is not supported", from, to)
}

func (s *Service) Submit(fileURL, fromFormat, toFormat string) (string, error) {
	toFormat = strings.ToLower(strings.TrimPrefix(toFormat, "."))
	if toFormat == "jpeg" {
		toFormat = "jpg"
	}
	if !allowedFormats[toFormat] {
		return "", fmt.Errorf("unsupported image format: %s", toFormat)
	}
	if err := s.validateCompatibility(fromFormat, toFormat); err != nil {
		return "", err
	}
	p := s.findProvider(toFormat)
	if p == nil {
		return "", fmt.Errorf("no provider available for format: %s", toFormat)
	}
	return p.Submit(fileURL, toFormat)
}

func (s *Service) SubmitUpload(fileData []byte, filename, fromFormat, toFormat string) (string, error) {
	toFormat = strings.ToLower(strings.TrimPrefix(toFormat, "."))
	if toFormat == "jpeg" {
		toFormat = "jpg"
	}
	if !allowedFormats[toFormat] {
		return "", fmt.Errorf("unsupported image format: %s", toFormat)
	}
	if err := s.validateCompatibility(fromFormat, toFormat); err != nil {
		return "", err
	}
	p := s.findProvider(toFormat)
	if p == nil {
		return "", fmt.Errorf("no provider available for format: %s", toFormat)
	}
	return p.SubmitUpload(fileData, filename, toFormat)
}

func (s *Service) SubmitAndWait(fileURL, fromFormat, toFormat string) (*provider.ConvertResult, error) {
	jobID, err := s.Submit(fileURL, fromFormat, toFormat)
	if err != nil {
		return nil, err
	}
	return provider.WaitForJob(jobID, s.providers)
}

func (s *Service) SubmitAndWaitUpload(fileData []byte, filename, fromFormat, toFormat string) (*provider.ConvertResult, error) {
	jobID, err := s.SubmitUpload(fileData, filename, fromFormat, toFormat)
	if err != nil {
		return nil, err
	}
	return provider.WaitForJob(jobID, s.providers)
}

func (s *Service) Status(jobID string) (*provider.ConvertResult, error) {
	p := provider.ResolveProvider(s.providers, jobID)
	if p == nil {
		return nil, fmt.Errorf("unknown provider for job: %s", jobID)
	}
	return p.Status(jobID)
}

func (s *Service) findProvider(toFormat string) provider.Provider {
	ordered := provider.ResolveProviderForCategory(s.providers, "image")
	for _, p := range ordered {
		for _, f := range p.SupportedFormats() {
			if f == toFormat {
				return p
			}
		}
	}
	return nil
}
