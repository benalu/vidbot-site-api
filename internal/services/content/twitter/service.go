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
		return nil, fmt.Errorf("no providers available")
	}

	primaryCh := make(chan *provider.MediaResult, 1)
	secondaryCh := make(chan *provider.MediaResult, 1)

	for i, p := range ordered {
		ch := secondaryCh
		if i == 0 {
			ch = primaryCh
		}
		go func(pr provider.Provider, out chan *provider.MediaResult) {
			res, err := pr.Extract(url)
			if err != nil || res == nil {
				out <- nil
				return
			}
			out <- res
		}(p, ch)
	}

	if len(ordered) == 1 {
		primary := <-primaryCh
		if primary == nil {
			return nil, fmt.Errorf("provider failed")
		}
		return buildResult(primary, nil), nil
	}

	primary := <-primaryCh

	if primary == nil {
		secondary := <-secondaryCh
		if secondary == nil {
			return nil, fmt.Errorf("all providers failed")
		}
		return buildResult(secondary, nil), nil
	}

	var secondary *provider.MediaResult
	select {
	case secondary = <-secondaryCh:
	case <-time.After(3 * time.Second):
	}

	return buildResult(primary, secondary), nil
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
