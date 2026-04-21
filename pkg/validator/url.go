package validator

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"sync"
	"vidbot-api/pkg/cache"
)

var (
	domainCache     map[string][]string
	domainCacheOnce sync.Once
)

func loadDomainCache() {
	domainCacheOnce.Do(func() {
		data, err := os.ReadFile("config/allowed_domains.json")
		if err != nil {
			domainCache = map[string][]string{}
			return
		}
		var parsed map[string][]string
		if err := json.Unmarshal(data, &parsed); err != nil {
			domainCache = map[string][]string{}
			return
		}
		domainCache = parsed
	})
}

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
	// L1: in-memory dari JSON — loaded sekali saat startup
	loadDomainCache()
	if domains, ok := domainCache[site]; ok && len(domains) > 0 {
		return domains
	}

	// L2: Redis — hanya untuk site yang di-seed via Redis tapi belum di JSON
	// (misalnya site baru yang ditambah admin tanpa update JSON)
	domains, err := cache.SMembers(context.Background(), "allowed_domains:"+site)
	if err == nil && len(domains) > 0 {
		return domains
	}

	return []string{}
}
