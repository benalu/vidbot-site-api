package mediaresponse

import "vidbot-api/pkg/downloader"

type DownloadLinks struct {
	Original string `json:"original"`
	Server1  string `json:"server_1,omitempty"`
	Server2  string `json:"server_2,omitempty"`
}

// ==================== Vidhub Response =====================
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

// ==================== IPTV Response ========================

type IPTVCountry struct {
	Name      string   `json:"name"`
	Code      string   `json:"code"`
	Languages []string `json:"languages"`
	Flag      string   `json:"flag"`
}

type IPTVCategory struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type IPTVCountriesResponse struct {
	Success  bool          `json:"success"`
	Services string        `json:"services"`
	Total    int           `json:"total"`
	Data     []IPTVCountry `json:"data"`
}

type IPTVCategoriesResponse struct {
	Success  bool           `json:"success"`
	Services string         `json:"services"`
	Total    int            `json:"total"`
	Data     []IPTVCategory `json:"data"`
}

type IPTVStream struct {
	Title     string `json:"title,omitempty"`
	URL       string `json:"url"`
	Quality   string `json:"quality,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
	Referrer  string `json:"referrer,omitempty"`
}

type IPTVChannel struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Logo       string       `json:"logo,omitempty"`
	Country    string       `json:"country"`
	Categories []string     `json:"categories"`
	Website    string       `json:"website,omitempty"`
	Streams    []IPTVStream `json:"streams"`
}

type IPTVResponse struct {
	Success    bool          `json:"success"`
	Services   string        `json:"services"`
	Country    string        `json:"country,omitempty"`
	Category   string        `json:"category,omitempty"`
	Total      int           `json:"total"`
	Data       []IPTVChannel `json:"data"`
	Page       int           `json:"page,omitempty"`
	Limit      int           `json:"limit,omitempty"`
	TotalPages int           `json:"total_pages,omitempty"`
}

// ==================== Content Response =====================
type Author struct {
	Name     string `json:"name"`
	Username string `json:"username,omitempty"`
}

type ContentData struct {
	Title     string `json:"title"`
	Thumbnail string `json:"thumbnail"`
	Duration  string `json:"duration,omitempty"`
	URL       string `json:"url,omitempty"`
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

type ContentVideoQuality struct {
	Quality   string `json:"quality"`
	Original  string `json:"original"`
	Original1 string `json:"original_1,omitempty"`
	Server1   string `json:"server_1"`
	Server2   string `json:"server_2"`
}

type ContentAudio struct {
	Original  string `json:"original"`
	Original1 string `json:"original_1,omitempty"`
	Server1   string `json:"server_1"`
	Server2   string `json:"server_2"`
}

type ContentMultiDownload struct {
	Video []ContentVideoQuality `json:"video"`
	Audio *ContentAudio         `json:"audio,omitempty"`
}

type ContentMultiResponse struct {
	Success  bool                 `json:"success"`
	Services string               `json:"services"`
	Sites    string               `json:"sites"`
	Type     string               `json:"type"`
	Data     ContentData          `json:"data"`
	Download ContentMultiDownload `json:"download"`
}

type InstagramData struct {
	URL       string  `json:"url,omitempty"`
	Username  string  `json:"username,omitempty"`
	Author    string  `json:"author,omitempty"`
	ViewCount int64   `json:"view_count,omitempty"`
	LikeCount int64   `json:"like_count,omitempty"`
	Duration  float64 `json:"duration,omitempty"`
	Title     string  `json:"title,omitempty"`
	Thumbnail string  `json:"thumbnail,omitempty"`
}

type InstagramResponse struct {
	Success  bool                 `json:"success"`
	Services string               `json:"services"`
	Sites    string               `json:"sites"`
	Type     string               `json:"type"`
	Data     InstagramData        `json:"data"`
	Download ContentMultiDownload `json:"download"`
}

type TwitterData struct {
	URL       string  `json:"url,omitempty"`
	Author    string  `json:"author,omitempty"`
	Duration  float64 `json:"duration,omitempty"`
	Title     string  `json:"title,omitempty"`
	Thumbnail string  `json:"thumbnail,omitempty"`
}

type TwitterResponse struct {
	Success  bool                 `json:"success"`
	Services string               `json:"services"`
	Sites    string               `json:"sites"`
	Type     string               `json:"type"`
	Data     TwitterData          `json:"data"`
	Download ContentMultiDownload `json:"download"`
}

type TikTokDataNew struct {
	URL       string  `json:"url,omitempty"`
	Author    string  `json:"author,omitempty"`
	Username  string  `json:"username,omitempty"`
	Title     string  `json:"title,omitempty"`
	Thumbnail string  `json:"thumbnail,omitempty"`
	Duration  float64 `json:"duration,omitempty"`
}

type TikTokResponseNew struct {
	Success  bool                 `json:"success"`
	Services string               `json:"services"`
	Sites    string               `json:"sites"`
	Type     string               `json:"type"`
	Data     TikTokDataNew        `json:"data"`
	Download ContentMultiDownload `json:"download"`
}

type ThreadsMediaItem struct {
	Type      string `json:"type"`
	Original  string `json:"original"`
	Original1 string `json:"original_1,omitempty"`
	Server1   string `json:"server_1,omitempty"`
	Server2   string `json:"server_2,omitempty"`
}

type ThreadsDownload struct {
	Media []ThreadsMediaItem `json:"media"`
}

type ThreadsData struct {
	URL       string `json:"url,omitempty"`
	Author    string `json:"author,omitempty"`
	Title     string `json:"title,omitempty"`
	Thumbnail string `json:"thumbnail,omitempty"`
}

type ThreadsResponse struct {
	Success  bool            `json:"success"`
	Services string          `json:"services"`
	Sites    string          `json:"sites"`
	Type     string          `json:"type"`
	Data     ThreadsData     `json:"data"`
	Download ThreadsDownload `json:"download"`
}

type SpotifyData struct {
	URL       string `json:"url,omitempty"`
	Title     string `json:"title,omitempty"`
	Author    string `json:"author,omitempty"`
	Thumbnail string `json:"thumbnail,omitempty"`
	Duration  string `json:"duration,omitempty"`
	TrackID   string `json:"track_id,omitempty"`
	Quality   string `json:"quality,omitempty"`
}

type SpotifyDownload struct {
	Original string `json:"original"`
	Server1  string `json:"server_1,omitempty"`
	Server2  string `json:"server_2,omitempty"`
}

type SpotifyResponse struct {
	Success  bool            `json:"success"`
	Services string          `json:"services"`
	Sites    string          `json:"sites"`
	Type     string          `json:"type"`
	Data     SpotifyData     `json:"data"`
	Download SpotifyDownload `json:"download"`
}

// ========================= Convert Response =========================
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

// =====================================
