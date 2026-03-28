package limiter

import "sync"

var (
	HLSDownload   = New(3)  // yt-dlp concurrent max 3
	DirectStream  = New(10) // direct relay max 10
	cdnLimiterMu  sync.Mutex
	cdnLimiters   = make(map[string]*Limiter)
	cdnMaxPerHost = 1
)

func AcquireCDN(host string) bool {
	cdnLimiterMu.Lock()
	if _, ok := cdnLimiters[host]; !ok {
		cdnLimiters[host] = New(cdnMaxPerHost)
	}
	l := cdnLimiters[host]
	cdnLimiterMu.Unlock()
	return l.Acquire()
}

func ReleaseCDN(host string) {
	cdnLimiterMu.Lock()
	if l, ok := cdnLimiters[host]; ok {
		l.Release()
	}
	cdnLimiterMu.Unlock()
}
