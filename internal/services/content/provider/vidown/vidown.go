package vidown

import (
	"encoding/json"
	"fmt"
	"vidbot-api/internal/services/content/provider"
	"vidbot-api/pkg/proxy"
)

type Vidown struct {
	client   *proxy.Client
	platform string
}

func New(client *proxy.Client) *Vidown {
	return &Vidown{client: client, platform: "tiktok"}
}

func NewForPlatform(client *proxy.Client, platform string) *Vidown {
	return &Vidown{client: client, platform: platform}
}

func (v *Vidown) Name() string {
	return "vidown"
}

type vidownTikTokResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Cover    string `json:"cover"`
		Duration int    `json:"duration"`
		Play     string `json:"play"`
		HdPlay   string `json:"hdplay"`
		Music    string `json:"music"`
		Author   struct {
			UniqueID string `json:"unique_id"`
			Nickname string `json:"nickname"`
		} `json:"author"`
	} `json:"data"`
}

type vidownTwitterResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		HdUrl  string `json:"hdUrl"`
		SdUrl  string `json:"sdUrl"`
		Cover  string `json:"cover"`
		Author struct {
			UniqueID string `json:"unique_id"`
			Nickname string `json:"nickname"`
		} `json:"author"`
	} `json:"data"`
}

type vidownInstagramResponse struct {
	Status string `json:"status"`
	Data   struct {
		Filename    string `json:"filename"`
		Type        string `json:"type"`
		DownloadURL string `json:"download_url"`
		Metadata    struct {
			Title     string `json:"title"`
			Author    string `json:"author"`
			Thumbnail string `json:"thumbnail"`
		} `json:"metadata"`
	} `json:"data"`
}

type vidownThreadsResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Cover  string `json:"cover"`
		Medias []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"medias"`
		Author struct {
			UniqueID string `json:"unique_id"`
			Nickname string `json:"nickname"`
		} `json:"author"`
	} `json:"data"`
}

func (v *Vidown) Extract(url string) (*provider.MediaResult, error) {
	switch v.platform {
	case "twitter":
		return v.extractTwitter(url)
	case "instagram":
		return v.extractInstagram(url)
	case "threads":
		return v.extractThreads(url)
	default:
		return v.extractTikTok(url)
	}
}

func (v *Vidown) extractThreads(url string) (*provider.MediaResult, error) {
	workerURL := v.client.PickWorker()
	bodyBytes, _ := json.Marshal(map[string]string{"url": url})

	headers := map[string]string{
		"Content-Type":   "application/json",
		"Accept":         "*/*",
		"Origin":         "https://www.vidown.lat",
		"Referer":        "https://www.vidown.lat/threads-downloader",
		"User-Agent":     proxy.RandomUA(),
		"Sec-Fetch-Dest": "empty",
		"Sec-Fetch-Mode": "cors",
		"Sec-Fetch-Site": "same-origin",
	}

	resp, err := v.client.DoFromWorker(workerURL, "POST", "https://www.vidown.lat/api/resolve/threads", headers, string(bodyBytes), "follow")
	if err != nil {
		return nil, fmt.Errorf("vidown threads request: %w", err)
	}
	if resp.Status != 200 {
		return nil, fmt.Errorf("vidown threads HTTP %d", resp.Status)
	}

	var data vidownThreadsResponse
	if err := json.Unmarshal([]byte(resp.Body), &data); err != nil {
		return nil, fmt.Errorf("vidown threads parse: %w", err)
	}
	if data.Code != 0 {
		return nil, fmt.Errorf("vidown threads error: %s", data.Msg)
	}

	result := &provider.MediaResult{
		Title:     data.Data.Title,
		Thumbnail: data.Data.Cover,
		Author: provider.Author{
			Name:     data.Data.Author.Nickname,
			Username: data.Data.Author.UniqueID,
		},
	}

	for i, m := range data.Data.Medias {
		ext := "mp4"
		if m.Type == "image" {
			ext = "jpg"
		}
		result.MediaItems = append(result.MediaItems, provider.MediaItem{
			Index:     i,
			Type:      m.Type,
			URL:       m.URL,
			Extension: ext,
		})
	}

	if len(result.MediaItems) == 0 {
		return nil, fmt.Errorf("vidown threads: no media found")
	}

	return result, nil
}

func (v *Vidown) extractTikTok(url string) (*provider.MediaResult, error) {
	workerURL := v.client.PickWorker()
	bodyBytes, _ := json.Marshal(map[string]string{"url": url})

	headers := map[string]string{
		"Content-Type":   "application/json",
		"Accept":         "*/*",
		"Origin":         "https://www.vidown.lat",
		"Referer":        "https://www.vidown.lat/",
		"User-Agent":     proxy.RandomUA(),
		"Sec-Fetch-Dest": "empty",
		"Sec-Fetch-Mode": "cors",
		"Sec-Fetch-Site": "same-origin",
	}

	resp, err := v.client.DoFromWorker(workerURL, "POST", "https://www.vidown.lat/api/resolve", headers, string(bodyBytes), "follow")
	if err != nil {
		return nil, fmt.Errorf("vidown tiktok request: %w", err)
	}
	if resp.Status != 200 {
		return nil, fmt.Errorf("vidown tiktok HTTP %d", resp.Status)
	}

	var data vidownTikTokResponse
	if err := json.Unmarshal([]byte(resp.Body), &data); err != nil {
		return nil, fmt.Errorf("vidown tiktok parse: %w", err)
	}
	if data.Code != 0 {
		return nil, fmt.Errorf("vidown tiktok error: %s", data.Msg)
	}

	result := &provider.MediaResult{
		Title:     data.Data.Title,
		Thumbnail: data.Data.Cover,
		Duration:  formatDuration(data.Data.Duration),
		Author: provider.Author{
			Name:     data.Data.Author.Nickname,
			Username: data.Data.Author.UniqueID,
		},
	}

	if data.Data.HdPlay != "" {
		result.Videos = append(result.Videos, provider.Video{
			Quality:   "hd_no_watermark",
			URL:       data.Data.HdPlay,
			Extension: "mp4",
		})
	}
	if data.Data.Play != "" {
		result.Videos = append(result.Videos, provider.Video{
			Quality:   "no_watermark",
			URL:       data.Data.Play,
			Extension: "mp4",
		})
	}
	if data.Data.Music != "" {
		result.Audio = &provider.Audio{
			URL:       data.Data.Music,
			Extension: "mp3",
		}
	}

	if len(result.Videos) == 0 && result.Audio == nil {
		return nil, fmt.Errorf("vidown tiktok: no media found")
	}

	return result, nil
}

func (v *Vidown) extractInstagram(url string) (*provider.MediaResult, error) {
	workerURL := v.client.PickWorker()
	bodyBytes, _ := json.Marshal(map[string]string{"url": url})

	headers := map[string]string{
		"Content-Type":   "application/json",
		"Accept":         "*/*",
		"Origin":         "https://www.vidown.lat",
		"Referer":        "https://www.vidown.lat/",
		"User-Agent":     proxy.RandomUA(),
		"Sec-Fetch-Dest": "empty",
		"Sec-Fetch-Mode": "cors",
		"Sec-Fetch-Site": "same-origin",
	}

	resp, err := v.client.DoFromWorker(workerURL, "POST", "https://rjenthusiast-instaapi.hf.space/download/reel", headers, string(bodyBytes), "follow")
	if err != nil {
		return nil, fmt.Errorf("vidown instagram request: %w", err)
	}
	if resp.Status != 200 {
		return nil, fmt.Errorf("vidown instagram HTTP %d", resp.Status)
	}

	var data vidownInstagramResponse
	if err := json.Unmarshal([]byte(resp.Body), &data); err != nil {
		return nil, fmt.Errorf("vidown instagram parse: %w", err)
	}
	if data.Status != "success" {
		return nil, fmt.Errorf("vidown instagram error: %s", data.Status)
	}
	if data.Data.DownloadURL == "" {
		return nil, fmt.Errorf("vidown instagram: no download url")
	}

	result := &provider.MediaResult{
		Title:     data.Data.Metadata.Title,
		Thumbnail: data.Data.Metadata.Thumbnail,
		Author: provider.Author{
			Name: data.Data.Metadata.Author,
		},
	}

	if data.Data.Type == "video" {
		result.Videos = append(result.Videos, provider.Video{
			Quality:   "hd_no_watermark",
			URL:       data.Data.DownloadURL,
			Extension: "mp4",
		})
	}

	if len(result.Videos) == 0 {
		return nil, fmt.Errorf("vidown instagram: no media found")
	}

	return result, nil
}

func (v *Vidown) extractTwitter(url string) (*provider.MediaResult, error) {
	workerURL := v.client.PickWorker()
	bodyBytes, _ := json.Marshal(map[string]string{"url": url})

	headers := map[string]string{
		"Content-Type":   "application/json",
		"Accept":         "*/*",
		"Origin":         "https://www.vidown.lat",
		"Referer":        "https://www.vidown.lat/twitter-downloader",
		"User-Agent":     proxy.RandomUA(),
		"Sec-Fetch-Dest": "empty",
		"Sec-Fetch-Mode": "cors",
		"Sec-Fetch-Site": "same-origin",
	}

	resp, err := v.client.DoFromWorker(workerURL, "POST", "https://www.vidown.lat/api/resolve/twitter", headers, string(bodyBytes), "follow")
	if err != nil {
		return nil, fmt.Errorf("vidown twitter request: %w", err)
	}
	if resp.Status != 200 {
		return nil, fmt.Errorf("vidown twitter HTTP %d", resp.Status)
	}

	var data vidownTwitterResponse
	if err := json.Unmarshal([]byte(resp.Body), &data); err != nil {
		return nil, fmt.Errorf("vidown twitter parse: %w", err)
	}
	if data.Code != 0 {
		return nil, fmt.Errorf("vidown twitter error: %s", data.Msg)
	}

	result := &provider.MediaResult{
		Title:     data.Data.Title,
		Thumbnail: data.Data.Cover,
		Author: provider.Author{
			Name:     data.Data.Author.Nickname,
			Username: data.Data.Author.UniqueID,
		},
	}

	if data.Data.HdUrl != "" {
		result.Videos = append(result.Videos, provider.Video{
			Quality:   "hd_no_watermark",
			URL:       data.Data.HdUrl,
			Extension: "mp4",
		})
	}
	if data.Data.SdUrl != "" && data.Data.SdUrl != data.Data.HdUrl {
		result.Videos = append(result.Videos, provider.Video{
			Quality:   "no_watermark",
			URL:       data.Data.SdUrl,
			Extension: "mp4",
		})
	}

	if len(result.Videos) == 0 {
		return nil, fmt.Errorf("vidown twitter: no media found")
	}

	return result, nil
}

func formatDuration(seconds int) string {
	if seconds == 0 {
		return ""
	}
	m := seconds / 60
	s := seconds % 60
	return fmt.Sprintf("%d:%02d", m, s)
}
