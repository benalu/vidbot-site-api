package vidoy

type Request struct {
	URL string `json:"url" binding:"required"`
}

type ExtractionResult struct {
	Filecode    string `json:"filecode"`
	Title       string `json:"title"`
	Filename    string `json:"filename"`
	DownloadURL string `json:"download_url"`
	Size        int64  `json:"size"`
	Thumbnail   string `json:"thumbnail"`
}
