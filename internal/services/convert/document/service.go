package document

import (
	"fmt"
	"strings"
	"vidbot-api/internal/services/convert/provider"
)

var allowedFormats = map[string]bool{
	"csv": true, "doc": true, "docm": true, "docx": true,
	"dotx": true, "html": true, "md": true, "odt": true,
	"pdf": true, "ppt": true, "pptx": true, "rtf": true,
	"txt": true, "wps": true, "xls": true, "xlsx": true,
}

var formatCompatibility = map[string][]string{
	"csv":  {"csv", "html", "pdf", "jpg", "png", "xls", "xlsx"},
	"doc":  {"doc", "docx", "html", "odt", "pdf", "rtf", "txt", "jpg", "png"},
	"docm": {"docm", "doc", "docx", "html", "odt", "pdf", "rtf", "txt"},
	"docx": {"docx", "doc", "html", "jpg", "png", "odt", "pdf", "rtf", "txt"},
	"dotx": {"dotx", "doc", "docx", "html", "pdf", "txt", "rtf", "odt"},
	"html": {"doc", "md", "docx", "html", "pdf", "txt", "odt", "rtf", "jpg", "png"},
	"md":   {"docx", "html", "md", "pdf", "txt", "rtf", "odt"},
	"odt":  {"docx", "html", "odt", "pdf", "rtf", "txt"},
	"pdf":  {"doc", "docx", "html", "jpg", "png", "pptx", "psd", "rtf", "txt", "xlsx", "webp", "psd", "ppt", "xls", "eps", "avif", "gif", "pdf"},
	"ppt":  {"html", "jpg", "odp", "pdf", "png", "ppt", "pptx", "txt"},
	"pptx": {"eps", "html", "jpg", "odp", "pdf", "png", "ppt", "pptx", "txt"},
	"rtf":  {"doc", "docx", "html", "odt", "pdf", "rtf", "txt"},
	"txt":  {"doc", "docx", "html", "odt", "pdf", "rtf", "txt", "md"},
	"wps":  {"doc", "docx", "html", "pdf", "rtf", "txt", "wps"},
	"xls":  {"csv", "html", "pdf", "txt", "xlsx", "jpg", "png"},
	"xlsx": {"csv", "html", "pdf", "txt", "xlsx", "jpg", "png"},
}

type Service struct {
	providers []provider.Provider
}

func NewService(providers []provider.Provider) *Service {
	return &Service{providers: providers}
}

func (s *Service) validateCompatibility(from, to string) error {
	if from == "" {
		return nil
	}
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
	if !allowedFormats[toFormat] {
		return "", fmt.Errorf("unsupported document format: %s", toFormat)
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
	if !allowedFormats[toFormat] {
		return "", fmt.Errorf("unsupported document format: %s", toFormat)
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
	ordered := provider.ResolveProviderForCategory(s.providers, "document")
	for _, p := range ordered {
		for _, f := range p.SupportedFormats() {
			if f == toFormat {
				return p
			}
		}
	}
	return nil
}
