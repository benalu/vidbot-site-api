package limiter

var (
	HLSDownload  = New(3)  // yt-dlp concurrent max 3
	DirectStream = New(10) // direct relay max 10
)
