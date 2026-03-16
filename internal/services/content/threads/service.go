package threads

import (
	"fmt"
	"time"
	"vidbot-api/internal/services/content/provider"
)

type MediaItemResult struct {
	Index     int
	Type      string
	URL       string
	URL2      string
	Thumbnail string
	Extension string
}

type ExtractionResult struct {
	Title      string
	Thumbnail  string
	URL        string
	Author     provider.Author
	MediaItems []MediaItemResult
}

type Service struct {
	providers []provider.Provider
}

func NewService(providers []provider.Provider) *Service {
	return &Service{providers: providers}
}

func (s *Service) Extract(url string) (*ExtractionResult, error) {
	ordered := provider.ResolveProviderForCategory(s.providers, "threads")

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
	case <-time.After(1 * time.Second):
	}

	return buildResult(primary, secondary), nil
}

func buildResult(primary, secondary *provider.MediaResult) *ExtractionResult {
	// build secondary map berdasarkan type + urutan per type
	secondaryVideoURLs := []string{}
	secondaryImageURLs := []string{}

	if secondary != nil {
		for _, m := range secondary.MediaItems {
			if m.Type == "video" {
				secondaryVideoURLs = append(secondaryVideoURLs, m.URL)
			} else {
				secondaryImageURLs = append(secondaryImageURLs, m.URL)
			}
		}
	}

	videoIdx := 0
	imageIdx := 0

	items := []MediaItemResult{}
	for _, m := range primary.MediaItems {
		url2 := ""
		if m.Type == "video" && videoIdx < len(secondaryVideoURLs) {
			url2 = secondaryVideoURLs[videoIdx]
			videoIdx++
		} else if m.Type == "image" && imageIdx < len(secondaryImageURLs) {
			url2 = secondaryImageURLs[imageIdx]
			imageIdx++
		}

		items = append(items, MediaItemResult{
			Index:     m.Index,
			Type:      m.Type,
			URL:       m.URL,
			URL2:      url2,
			Thumbnail: m.Thumbnail,
			Extension: m.Extension,
		})
	}

	return &ExtractionResult{
		Title:      primary.Title,
		Thumbnail:  primary.Thumbnail,
		URL:        primary.URL,
		Author:     primary.Author,
		MediaItems: items,
	}
}

func resolveType(items []MediaItemResult) string {
	hasVideo := false
	hasImage := false
	for _, m := range items {
		switch m.Type {
		case "video":
			hasVideo = true
		case "image":
			hasImage = true
		}
	}

	if hasVideo && hasImage {
		return "mixed"
	}
	if hasVideo {
		return "video"
	}
	return "image"
}
