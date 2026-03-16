package main

import (
	"context"
	"log"
	"vidbot-api/config"
	"vidbot-api/pkg/cache"
)

func main() {
	cfg := config.Load()
	cache.Init(cfg.RedisURL)

	ctx := context.Background()

	// allowed domains
	seeds := map[string][]string{
		"allowed_domains:videb": {
			"videb.co",
		},
		"allowed_domains:vidoy": {
			"vidoy.co",
			"vidoy.cam",
			"vidstrm.cloud",
		},
		"allowed_domains:vidbos": {
			"vidbos.com",
		},
		"allowed_domains:vidarato": {
			"vidara.to",
			"vidara.so",
		},
		"allowed_domains:vidnest": {
			"vidnest.io",
		},
		"allowed_domains:spotify": {
			"open.spotify.com",
			"spotify.com",
			"spotify.link",
		},
		"allowed_domains:tiktok": {
			"tiktok.com",
			"vt.tiktok.com",
			"vm.tiktok.com",
			"douyin.com",
		},
		"allowed_domains:instagram": {
			"instagram.com",
			"instagr.am",
		},
		"allowed_domains:twitter": {
			"twitter.com",
			"x.com",
			"t.co",
		},
		"allowed_domains:threads": {
			"threads.net",
			"threads.com",
		},
	}

	for key, domains := range seeds {
		for _, domain := range domains {
			if err := cache.SAdd(ctx, key, domain); err != nil {
				log.Printf("Failed to seed %s: %v", key, err)
				continue
			}
		}
		log.Printf("Seeded %s", key)
	}

	// convert provider priority
	convertProviderKeys := []string{
		"convert:provider:audio",
		"convert:provider:document",
		"convert:provider:image",
		"convert:provider:fonts",
	}

	for _, key := range convertProviderKeys {
		if err := cache.Del(ctx, key); err != nil {
			log.Printf("Failed to del %s: %v", key, err)
		}
		if err := cache.RPush(ctx, key, "cloudconvert", "convertio"); err != nil {
			log.Printf("Failed to seed %s: %v", key, err)
			continue
		}
		log.Printf("Seeded %s", key)
	}

	// content provider priority
	contentProviderKeys := []string{
		"content:provider:spotify",
		"content:provider:tiktok",
		"content:provider:instagram",
		"content:provider:twitter",
		"content:provider:threads",
	}

	for _, key := range contentProviderKeys {
		if err := cache.Del(ctx, key); err != nil {
			log.Printf("Failed to del %s: %v", key, err)
		}
		if err := cache.RPush(ctx, key, "downr", "vidown"); err != nil {
			log.Printf("Failed to seed %s: %v", key, err)
			continue
		}
		log.Printf("Seeded %s", key)
	}

	log.Println("Seed completed")
}
