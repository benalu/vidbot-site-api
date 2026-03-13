package mediaresponse

import "vidbot-api/pkg/downloader"

type DownloadLinks struct {
	Original string `json:"original"`
	Server1  string `json:"server_1,omitempty"`
	Server2  string `json:"server_2,omitempty"`
}

// untuk vidhub
type VidhubData struct {
	Filecode  string `json:"filecode"`
	Title     string `json:"title"`
	Filename  string `json:"filename"`
	Thumbnail string `json:"thumbnail"`
	Size      int64  `json:"size"`
}

type VidhubResponse struct {
	Success  bool                 `json:"success"`
	Services string               `json:"services"`
	Sites    string               `json:"sites"`
	Type     downloader.VideoType `json:"type"`
	Data     VidhubData           `json:"data"`
	Download DownloadLinks        `json:"download"`
}

// untuk content
type Author struct {
	Name     string `json:"name"`
	Username string `json:"username,omitempty"`
}

type ContentData struct {
	Title     string `json:"title"`
	Thumbnail string `json:"thumbnail"`
	Duration  string `json:"duration,omitempty"`
	URL       string `json:"url"`
	Quality   string `json:"quality,omitempty"`
	Author    Author `json:"author,omitempty"`
}

type ContentResponse struct {
	Success  bool          `json:"success"`
	Services string        `json:"services"`
	Sites    string        `json:"sites"`
	Type     string        `json:"type"`
	Data     ContentData   `json:"data"`
	Download DownloadLinks `json:"download"`
}

type ConvertData struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	Provider string `json:"provider"`
}

type ConvertDownloadLinks struct {
	Original string `json:"original"`
	Server1  string `json:"server_1"`
	Server2  string `json:"server_2"`
}

type ConvertResponse struct {
	Success  bool                 `json:"success"`
	Services string               `json:"services"`
	Category string               `json:"category"`
	Data     ConvertData          `json:"data"`
	Download ConvertDownloadLinks `json:"download"`
}

type TikTokVideoQuality struct {
	Quality  string `json:"quality"`
	Original string `json:"original"`
	Server1  string `json:"server_1"`
	Server2  string `json:"server_2"`
}

type TikTokAudio struct {
	Original string `json:"original"`
	Server1  string `json:"server_1"`
	Server2  string `json:"server_2"`
}

type TikTokDownload struct {
	Video []TikTokVideoQuality `json:"video"`
	Audio *TikTokAudio         `json:"audio,omitempty"`
}

type TikTokData struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Thumbnail string `json:"thumbnail"`
	Duration  string `json:"duration"`
	Author    Author `json:"author"`
}

type TikTokResponse struct {
	Success  bool           `json:"success"`
	Services string         `json:"services"`
	Sites    string         `json:"sites"`
	Type     string         `json:"type"`
	Data     TikTokData     `json:"data"`
	Download TikTokDownload `json:"download"`
}
