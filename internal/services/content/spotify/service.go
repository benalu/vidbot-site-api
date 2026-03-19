package spotify

import (
	"fmt"
	"vidbot-api/internal/services/content/provider"
)

type Service struct {
	providers []provider.Provider
}

func NewService(providers []provider.Provider) *Service {
	return &Service{providers: providers}
}

type ExtractionResult struct {
	Title     string
	Thumbnail string
	Duration  string
	URL       string
	TrackID   string
	Author    provider.Author
	AudioURL  string
	AudioExt  string
	Quality   string
}

func (s *Service) Extract(url string) (*ExtractionResult, error) {
	var lastErr error

	for _, p := range s.providers {
		result, err := p.Extract(url)
		if err != nil {
			lastErr = err
			continue
		}

		if result.Audio == nil {
			lastErr = fmt.Errorf("[%s] no audio found", p.Name())
			continue
		}

		return &ExtractionResult{
			Title:     result.Title,
			Thumbnail: result.Thumbnail,
			Duration:  result.Duration,
			URL:       result.URL,
			TrackID:   result.TrackID,
			Author:    result.Author,
			AudioURL:  result.Audio.URL,
			AudioExt:  result.Audio.Extension,
			Quality:   result.Audio.Quality,
		}, nil
	}

	return nil, fmt.Errorf("service is temporarily unavailable: %w", lastErr)
}
