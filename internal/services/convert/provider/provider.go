package provider

import (
	"context"
	"strings"
	"vidbot-api/pkg/cache"
)

type ConvertResult struct {
	JobID       string `json:"job_id"`
	Status      string `json:"status"`
	DownloadURL string `json:"download_url,omitempty"`
	Filename    string `json:"filename,omitempty"`
	Size        int64  `json:"size,omitempty"`
	Message     string `json:"message,omitempty"`
	Provider    string `json:"provider"`
}

type Provider interface {
	Name() string
	SupportedFormats() []string
	Submit(fileURL, toFormat string) (string, error)
	SubmitUpload(fileData []byte, filename, toFormat string) (string, error)
	Status(jobID string) (*ConvertResult, error)
}

// ResolveProvider — resolve provider dari prefix job_id
func ResolveProvider(providers []Provider, jobID string) Provider {
	for _, p := range providers {
		switch p.Name() {
		case "cloudconvert":
			if strings.HasPrefix(jobID, "cc_") {
				return p
			}
		case "convertio":
			if strings.HasPrefix(jobID, "cv_") {
				return p
			}
		}
	}
	return nil
}

// ResolveProviderForCategory — ambil provider priority dari Redis, fallback ke urutan default
func ResolveProviderForCategory(providers []Provider, category string) []Provider {
	ctx := context.Background()
	key := "convert:provider:" + category

	names, err := cache.LRange(ctx, key)
	if err != nil || len(names) == 0 {
		// fallback — pakai urutan yang didaftarkan di router
		return providers
	}

	// susun ulang providers sesuai priority dari Redis
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
