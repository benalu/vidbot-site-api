package validator

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"vidbot-api/pkg/cache"
)

func IsValidURL(raw string) bool {
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

func IsAllowedDomain(raw, site string) bool {
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())

	domains := getDomainsForSite(site)

	// kalau kosong — allow semua (jangan block user)
	if len(domains) == 0 {
		return true
	}

	for _, domain := range domains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}

func getDomainsForSite(site string) []string {
	// 1. coba Redis dulu
	domains, err := cache.SMembers(context.Background(), "allowed_domains:"+site)
	if err == nil && len(domains) > 0 {
		return domains
	}

	// 2. fallback ke JSON file
	if domains := loadFromJSON(site); len(domains) > 0 {
		return domains
	}

	// 3. semua gagal — return kosong (allow semua)
	return []string{}
}

func loadFromJSON(site string) []string {
	data, err := os.ReadFile("config/allowed_domains.json")
	if err != nil {
		return nil
	}

	var allDomains map[string][]string
	if err := json.Unmarshal(data, &allDomains); err != nil {
		return nil
	}

	return allDomains[site]
}
