package tmdb

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	baseURL      = "https://api.themoviedb.org/3"
	imageBaseURL = "https://image.tmdb.org/t/p/original"
)

type Client struct {
	apiKey     string
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ─── TMDB Response types ──────────────────────────────────────────────────────

type tmdbMovieDetail struct {
	ID           int     `json:"id"`
	Title        string  `json:"title"`
	ReleaseDate  string  `json:"release_date"` // "2024-05-01"
	Runtime      int     `json:"runtime"`      // menit
	VoteAverage  float64 `json:"vote_average"`
	Overview     string  `json:"overview"`
	PosterPath   string  `json:"poster_path"`
	BackdropPath string  `json:"backdrop_path"`
	Genres       []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
	Images struct {
		Logos []struct {
			FilePath string `json:"file_path"`
			ISO639_1 string `json:"iso_639_1"`
		} `json:"logos"`
	} `json:"images"`
}

// MovieMeta adalah data yang sudah dinormalisasi siap dipakai
type MovieMeta struct {
	TmdbID   string
	Title    string
	Year     string
	Duration string // "120 min"
	Rating   string // "7.8"
	Genre    string // "Action, Drama"
	Poster   string // full URL
	Backdrop string // full URL
	Logo     string // full URL (English logo atau fallback)
	Overview string
}

// GetMovieMeta mengambil metadata film dari TMDB berdasarkan ID
func (c *Client) GetMovieMeta(tmdbID string) (*MovieMeta, error) {
	url := fmt.Sprintf("%s/movie/%s?api_key=%s&append_to_response=images&include_image_language=en,null",
		baseURL, tmdbID, c.apiKey)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("tmdb: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tmdb: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("tmdb: movie with ID '%s' not found", tmdbID)
	}
	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("tmdb: invalid API key")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("tmdb: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var detail tmdbMovieDetail
	if err := json.Unmarshal(body, &detail); err != nil {
		return nil, fmt.Errorf("tmdb: parse response: %w", err)
	}

	return normalizeMeta(tmdbID, &detail), nil
}

// normalizeMeta mengubah raw TMDB response jadi MovieMeta yang bersih
func normalizeMeta(tmdbID string, d *tmdbMovieDetail) *MovieMeta {
	meta := &MovieMeta{
		TmdbID:   tmdbID,
		Title:    d.Title,
		Overview: d.Overview,
	}

	// year dari release_date "2024-05-01"
	if len(d.ReleaseDate) >= 4 {
		meta.Year = d.ReleaseDate[:4]
	}

	// duration dalam menit
	if d.Runtime > 0 {
		meta.Duration = strconv.Itoa(d.Runtime) + " min"
	}

	// rating 1 decimal
	if d.VoteAverage > 0 {
		meta.Rating = fmt.Sprintf("%.1f", d.VoteAverage)
	}

	// genres jadi "Action, Drama"
	genres := make([]string, 0, len(d.Genres))
	for _, g := range d.Genres {
		genres = append(genres, g.Name)
	}
	meta.Genre = strings.Join(genres, ", ")

	// poster dan backdrop
	if d.PosterPath != "" {
		meta.Poster = imageBaseURL + d.PosterPath
	}
	if d.BackdropPath != "" {
		meta.Backdrop = imageBaseURL + d.BackdropPath
	}

	// logo: prioritaskan English, fallback ke logo pertama yang ada
	for _, logo := range d.Images.Logos {
		if logo.ISO639_1 == "en" && logo.FilePath != "" {
			meta.Logo = imageBaseURL + logo.FilePath
			break
		}
	}
	if meta.Logo == "" && len(d.Images.Logos) > 0 && d.Images.Logos[0].FilePath != "" {
		meta.Logo = imageBaseURL + d.Images.Logos[0].FilePath
	}

	return meta
}
