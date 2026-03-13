package audio

import (
	"fmt"
	"strings"
	"vidbot-api/internal/services/convert/provider"
)

var allowedFormats = map[string]bool{
	"mp3": true, "wav": true, "flac": true,
	"aac": true, "ogg": true, "m4a": true,
	"opus": true, "wma": true, "amr": true, "ac3": true,
}

var formatCompatibility = map[string][]string{
	"mp3":  {"wav", "flac", "aac", "ogg", "m4a", "opus", "wma", "ac3"},
	"wav":  {"mp3", "flac", "aac", "ogg", "m4a", "opus", "wma", "ac3"},
	"flac": {"mp3", "wav", "aac", "ogg", "m4a", "opus"},
	"aac":  {"mp3", "wav", "flac", "ogg", "m4a", "opus", "wma", "ac3"},
	"ogg":  {"mp3", "wav", "flac", "aac", "m4a", "opus"},
	"m4a":  {"mp3", "wav", "flac", "aac", "ogg", "opus", "wma"},
	"opus": {"mp3", "wav", "flac", "aac", "ogg", "m4a"},
	"wma":  {"mp3", "wav", "flac", "aac", "ogg", "m4a", "opus"},
	"amr":  {"mp3", "wav", "aac", "ogg", "m4a"},
	"ac3":  {"mp3", "wav", "flac", "aac", "ogg", "m4a"},
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
		return "", fmt.Errorf("unsupported audio format: %s", toFormat)
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
		return "", fmt.Errorf("unsupported audio format: %s", toFormat)
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
	ordered := provider.ResolveProviderForCategory(s.providers, "audio")
	for _, p := range ordered {
		for _, f := range p.SupportedFormats() {
			if f == toFormat {
				return p
			}
		}
	}
	return nil
}
