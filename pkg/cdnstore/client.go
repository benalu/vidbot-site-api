package cdnstore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	baseURL    = "https://api.stor.co.id/v1"
	cdnBaseURL = "https://cdn.stor.co.id"
)

type Client struct {
	apiKey     string
	folderID   string
	httpClient *http.Client
}

func NewClient(apiKey, folderID string) *Client {
	return &Client{
		apiKey:   apiKey,
		folderID: folderID,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// ─── CDN Types ────────────────────────────────────────────────────────────────

type CDNFile struct {
	ID           string    `json:"id"`
	Key          string    `json:"key"`
	OriginalName string    `json:"originalName"`
	MimeType     string    `json:"mimeType"`
	Size         int64     `json:"size"`
	Downloads    int       `json:"downloads"`
	IsPublic     bool      `json:"isPublic"`
	FolderID     string    `json:"folderId"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type SearchResponse struct {
	Files      []CDNFile `json:"files"`
	Total      int       `json:"total"`
	Page       int       `json:"page"`
	Limit      int       `json:"limit"`
	TotalPages int       `json:"totalPages"`
}

type DownloadResponse struct {
	URL          string    `json:"url"`
	ExpiresIn    int       `json:"expiresIn"`
	MaxDownloads int       `json:"maxDownloads"`
	ExpiresAt    time.Time `json:"expiresAt"`
	Filename     string    `json:"filename"`
}

// ─── Search files by keyword ──────────────────────────────────────────────────

func (c *Client) SearchFiles(ctx context.Context, keyword string, page, limit int) (*SearchResponse, error) {
	params := url.Values{}
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Set("q", keyword)
	if c.folderID != "" {
		params.Set("folderId", c.folderID)
	}

	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/files?%s", baseURL, params.Encode()), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cdn search: %w", err)
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("cdn search HTTP %d: %s", resp.StatusCode, string(b))
	}

	var result SearchResponse
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, fmt.Errorf("cdn search parse: %w", err)
	}
	return &result, nil
}

// ─── Get signed download URL for a file ──────────────────────────────────────

// expiresIn dalam jam, CDN API menerima dalam detik
func (c *Client) GetDownloadURL(ctx context.Context, fileID string, expiresInHours int) (*DownloadResponse, error) {
	expiresInSecs := expiresInHours * 3600

	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/files/%s/download?expiresIn=%d", baseURL, fileID, expiresInSecs), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cdn download url: %w", err)
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("cdn download url HTTP %d: %s", resp.StatusCode, string(b))
	}

	var result DownloadResponse
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, fmt.Errorf("cdn download url parse: %w", err)
	}
	return &result, nil
}
