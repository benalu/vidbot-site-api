package provider

import (
	"context"
	"vidbot-api/pkg/cache"
)

type Video struct {
	Quality   string `json:"quality"`
	URL       string `json:"url"`
	Extension string `json:"extension"`
	Size      int64  `json:"size,omitempty"`
}

type Audio struct {
	URL       string `json:"url"`
	Quality   string `json:"quality"`
	Extension string `json:"extension"`
}

type Author struct {
	Name     string `json:"name"`
	Username string `json:"username,omitempty"`
}

type MediaResult struct {
	Title     string  `json:"title"`
	Thumbnail string  `json:"thumbnail"`
	Duration  string  `json:"duration"`
	Author    Author  `json:"author"`
	Videos    []Video `json:"videos,omitempty"`
	Audio     *Audio  `json:"audio,omitempty"`
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
