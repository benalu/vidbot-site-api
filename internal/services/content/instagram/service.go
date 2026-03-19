package instagram

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
	ID        string
	URL       string
	Title     string
	Thumbnail string
	Duration  string
	Author    provider.Author
	Videos    []VideoResult
	AudioURL  string
	AudioURL2 string
	AudioExt  string
	ViewCount int64
	LikeCount int64
}

type Service struct {
	providers []provider.Provider
}

func NewService(providers []provider.Provider) *Service {
	return &Service{providers: providers}
}

func (s *Service) Extract(url string) (*ExtractionResult, error) {
	ordered := provider.ResolveProviderForCategory(s.providers, "instagram")

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
	// ambil URL pertama dari secondary — tidak perlu quality matching
	var secondaryFirstURL string
	var secondaryAudioURL string

	if secondary != nil {
		if len(secondary.Videos) > 0 {
			secondaryFirstURL = secondary.Videos[0].URL
		}
		if secondary.Audio != nil {
			secondaryAudioURL = secondary.Audio.URL
		}
	}

	videos := []VideoResult{}
	for i, v := range primary.Videos {
		url2 := ""
		if i == 0 {
			url2 = secondaryFirstURL
		}
		videos = append(videos, VideoResult{
			Quality:   v.Quality,
			URL:       v.URL,
			URL2:      url2,
			Extension: v.Extension,
		})
	}

	res := &ExtractionResult{
		URL:       primary.URL,
		Title:     primary.Title,
		Thumbnail: primary.Thumbnail,
		Duration:  primary.Duration,
		Author:    primary.Author,
		Videos:    videos,
		ViewCount: primary.ViewCount,
		LikeCount: primary.LikeCount,
	}

	if primary.Audio != nil {
		res.AudioURL = primary.Audio.URL
		res.AudioURL2 = secondaryAudioURL
		res.AudioExt = primary.Audio.Extension
	}

	return res
}
