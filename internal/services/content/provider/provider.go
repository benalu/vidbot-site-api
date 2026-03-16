package provider

import (
	"context"
	"vidbot-api/pkg/cache"
)

type Video struct {
	Quality   string `json:"quality"`
	URL       string `json:"url"`
	URL2      string `json:"url2,omitempty"`
	Extension string `json:"extension"`
	Size      int64  `json:"size,omitempty"`
}

type Audio struct {
	URL       string `json:"url"`
	URL2      string `json:"url2,omitempty"`
	Quality   string `json:"quality"`
	Extension string `json:"extension"`
}

type Author struct {
	Name     string `json:"name"`
	Username string `json:"username,omitempty"`
}

// MediaItem untuk platform yang support mixed media (video + image)
type MediaItem struct {
	Index     int    `json:"index"`
	Type      string `json:"type"` // "video" atau "image"
	URL       string `json:"url"`
	URL2      string `json:"url2,omitempty"`
	Thumbnail string `json:"thumbnail,omitempty"`
	Extension string `json:"extension"`
}

type MediaResult struct {
	Title      string      `json:"title"`
	Thumbnail  string      `json:"thumbnail"`
	Duration   string      `json:"duration"`
	Author     Author      `json:"author"`
	Videos     []Video     `json:"videos,omitempty"`
	Audio      *Audio      `json:"audio,omitempty"`
	MediaItems []MediaItem `json:"media_items,omitempty"` // untuk threads
	URL        string      `json:"url,omitempty"`
	ViewCount  int64       `json:"view_count,omitempty"`
	LikeCount  int64       `json:"like_count,omitempty"`
}

type Provider interface {
	Name() string
	Extract(url string) (*MediaResult, error)
}

func ResolveProviderForCategory(providers []Provider, category string) []Provider {
	ctx := context.Background()
	key := "content:provider:" + category

	names, err := cache.LRange(ctx, key)
	if err != nil || len(names) == 0 {
		return providers
	}

	providerMap := make(map[string]Provider)
	for _, p := range providers {
		providerMap[p.Name()] = p
	}

	ordered := []Provider{}
	for _, name := range names {
		if p, ok := providerMap[name]; ok {
			ordered = append(ordered, p)
		}
	}

	if len(ordered) == 0 {
		return providers
	}
	return ordered
}
