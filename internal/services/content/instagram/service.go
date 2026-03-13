package instagram

import (
	"fmt"
	"vidbot-api/internal/services/content/provider"
)

type VideoResult struct {
	Quality   string
	URL       string
	Extension string
}

type ExtractionResult struct {
	ID        string
	Title     string
	Thumbnail string
	Duration  string
	Author    provider.Author
	Videos    []VideoResult
	AudioURL  string
	AudioExt  string
}

type Service struct {
	providers []provider.Provider
}

func NewService(providers []provider.Provider) *Service {
	return &Service{providers: providers}
}

func (s *Service) Extract(url string) (*ExtractionResult, error) {
	var lastErr error

	ordered := provider.ResolveProviderForCategory(s.providers, "instagram")

	for _, p := range ordered {
		result, err := p.Extract(url)
		if err != nil {
			lastErr = err
			continue
		}

		if len(result.Videos) == 0 && result.Audio == nil {
			lastErr = fmt.Errorf("[%s] no media found", p.Name())
			continue
		}

		videos := []VideoResult{}
		for _, v := range result.Videos {
			videos = append(videos, VideoResult{
				Quality:   v.Quality,
				URL:       v.URL,
				Extension: v.Extension,
			})
		}

		res := &ExtractionResult{
			Title:     result.Title,
			Thumbnail: result.Thumbnail,
			Duration:  result.Duration,
			Author:    result.Author,
			Videos:    videos,
		}

		if result.Audio != nil {
			res.AudioURL = result.Audio.URL
			res.AudioExt = result.Audio.Extension
		}

		return res, nil
	}

	return nil, fmt.Errorf("all providers failed: %w", lastErr)
}
