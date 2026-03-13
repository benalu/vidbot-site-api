package fonts

import (
	"fmt"
	"strings"
	"vidbot-api/internal/services/convert/provider"
)

var allowedFormats = map[string]bool{
	"ttf": true, "otf": true, "woff": true,
	"woff2": true, "eot": true,
}

var formatCompatibility = map[string][]string{
	"ttf":   {"otf", "woff", "woff2", "eot"},
	"otf":   {"ttf", "woff", "woff2", "eot"},
	"woff":  {"ttf", "otf", "woff2", "eot"},
	"woff2": {"ttf", "otf", "woff", "eot"},
	"eot":   {"ttf", "otf", "woff", "woff2"},
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
	if !allowedFormats[toFormat] {
		return "", fmt.Errorf("unsupported font format: %s", toFormat)
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
		return "", fmt.Errorf("unsupported font format: %s", toFormat)
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
	ordered := provider.ResolveProviderForCategory(s.providers, "fonts")
	for _, p := range ordered {
		for _, f := range p.SupportedFormats() {
			if f == toFormat {
				return p
			}
		}
	}
	return nil
}
