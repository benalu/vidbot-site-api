package twitter

import (
	"fmt"
	"time"
	"vidbot-api/internal/services/content/provider"
)

type VideoResult struct {
	Quality   string
	URL       string
	URL2      string
	Extension string
}

type ExtractionResult struct {
	Title     string
	Thumbnail string
	Duration  string
	URL       string
	Author    provider.Author
	Videos    []VideoResult
	AudioURL  string
	AudioURL2 string
	AudioExt  string
}

type Service struct {
	providers []provider.Provider
}

func NewService(providers []provider.Provider) *Service {
	return &Service{providers: providers}
}

func (s *Service) Extract(url string) (*ExtractionResult, error) {
	ordered := provider.ResolveProviderForCategory(s.providers, "twitter")

	if len(ordered) == 0 {
		return nil, fmt.Errorf("service is temporarily unavailable")
	}

	type extractResult struct {
		media *provider.MediaResult
		err   error
	}

	primaryCh := make(chan extractResult, 1)
	secondaryCh := make(chan extractResult, 1)

	for i, p := range ordered {
		ch := secondaryCh
		if i == 0 {
			ch = primaryCh
		}
		go func(pr provider.Provider, out chan extractResult) {
			res, err := pr.Extract(url)
			out <- extractResult{media: res, err: err}
		}(p, ch)
	}

	if len(ordered) == 1 {
		r := <-primaryCh
		if r.err != nil || r.media == nil {
			return nil, fmt.Errorf("service is temporarily unavailable: %w", r.err)
		}
		return buildResult(r.media, nil), nil
	}

	r := <-primaryCh

	if r.err != nil || r.media == nil {
		r2 := <-secondaryCh
		if r2.err != nil || r2.media == nil {
			return nil, fmt.Errorf("service is temporarily unavailable: primary=%v, secondary=%v", r.err, r2.err)
		}
		return buildResult(r2.media, nil), nil
	}

	var secondary *provider.MediaResult
	select {
	case r2 := <-secondaryCh:
		secondary = r2.media
	case <-time.After(1 * time.Second):
	}

	return buildResult(r.media, secondary), nil
}

func buildResult(primary, secondary *provider.MediaResult) *ExtractionResult {
	secondaryVideoMap := map[string]string{}
	var secondaryAudioURL string
	if secondary != nil {
		for _, v := range secondary.Videos {
			secondaryVideoMap[v.Quality] = v.URL
		}
		if secondary.Audio != nil {
			secondaryAudioURL = secondary.Audio.URL
		}
	}

	videos := []VideoResult{}
	for _, v := range primary.Videos {
		videos = append(videos, VideoResult{
			Quality:   v.Quality,
			URL:       v.URL,
			URL2:      secondaryVideoMap[v.Quality],
			Extension: v.Extension,
		})
	}

	res := &ExtractionResult{
		Title:     primary.Title,
		Thumbnail: primary.Thumbnail,
		Duration:  primary.Duration,
		URL:       primary.URL,
		Author:    primary.Author,
		Videos:    videos,
	}

	if primary.Audio != nil {
		res.AudioURL = primary.Audio.URL
		res.AudioURL2 = secondaryAudioURL
		res.AudioExt = primary.Audio.Extension
	}

	return res
}
