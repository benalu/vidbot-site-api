package convertio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"vidbot-api/internal/services/convert/provider"
)

const (
	baseURL = "https://api.convertio.co"
	prefix  = "cv_"
)

var supportedFormats = []string{
	// audio
	"aac", "flac", "m4a", "mp3", "ogg", "opus", "wav", "wma", "amr", "ac3",
	// document
	"csv", "doc", "docm", "docx", "dotx", "html", "md", "odt", "pdf", "ppt", "pptx", "rtf", "txt", "wps", "xls", "xlsx",
	// image
	"avif", "bmp", "eps", "gif", "heic", "heif", "ico", "jfif", "jpg", "jpeg", "mrw", "png", "psd", "raf", "tiff", "webp", "svg", "raw",
	// fonts
	"eot", "otf", "ttf", "woff", "woff2",
}

type ConvertioProvider struct {
	apiKey     string
	httpClient *http.Client
}

func New(apiKey string) *ConvertioProvider {
	return &ConvertioProvider{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (p *ConvertioProvider) Name() string {
	return "convertio"
}

func (p *ConvertioProvider) SupportedFormats() []string {
	return supportedFormats
}

// =====================
// START CONVERSION
// =====================

type startRequest struct {
	APIKey       string `json:"apikey"`
	Input        string `json:"input"`
	File         string `json:"file,omitempty"`
	Filename     string `json:"filename,omitempty"`
	OutputFormat string `json:"outputformat"`
}

type startResponse struct {
	Code   int    `json:"code"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	Data   struct {
		ID      string      `json:"id"`
		Minutes interface{} `json:"-"`
	} `json:"data"`
}

func (p *ConvertioProvider) startConversion(input, file, filename, toFormat string) (string, error) {
	req := startRequest{
		APIKey:       p.apiKey,
		Input:        input,
		File:         file,
		Filename:     filename,
		OutputFormat: toFormat,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	resp, err := p.httpClient.Post(baseURL+"/convert", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	var result startResponse
	if err := json.Unmarshal(b, &result); err != nil {
		return "", err
	}

	if result.Status != "ok" {
		return "", fmt.Errorf("convertio error: %s", result.Error)
	}

	return result.Data.ID, nil
}

// =====================
// SUBMIT VIA URL
// =====================

func (p *ConvertioProvider) Submit(fileURL, toFormat string) (string, error) {
	id, err := p.startConversion("url", fileURL, "", toFormat)
	if err != nil {
		return "", fmt.Errorf("convertio submit failed: %w", err)
	}
	return prefix + id, nil
}

// =====================
// SUBMIT VIA UPLOAD
// =====================

func (p *ConvertioProvider) SubmitUpload(fileData []byte, filename, toFormat string) (string, error) {
	// step 1 — start conversion dengan input=upload
	id, err := p.startConversion("upload", "", filename, toFormat)
	if err != nil {
		return "", fmt.Errorf("convertio start upload failed: %w", err)
	}

	// step 2 — PUT file ke /convert/:id/:filename
	uploadURL := fmt.Sprintf("%s/convert/%s/%s", baseURL, id, filename)
	req, err := http.NewRequest(http.MethodPut, uploadURL, bytes.NewReader(fileData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("convertio upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("convertio upload error %d: %s", resp.StatusCode, string(b))
	}

	return prefix + id, nil
}

// =====================
// STATUS
// =====================

type statusResponse struct {
	Code   int    `json:"code"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	Data   struct {
		ID          string      `json:"id"`
		Step        string      `json:"step"`
		StepPercent int         `json:"step_percent"`
		Minutes     interface{} `json:"-"`
		Output      struct {
			URL      string `json:"url"`
			Filename string `json:"filename"`
			Size     string `json:"size"`
		} `json:"output"`
	} `json:"data"`
}

func (p *ConvertioProvider) Status(jobID string) (*provider.ConvertResult, error) {
	rawID := strings.TrimPrefix(jobID, prefix)

	resp, err := p.httpClient.Get(fmt.Sprintf("%s/convert/%s/status", baseURL, rawID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	var result statusResponse
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}

	if result.Status != "ok" {
		return nil, fmt.Errorf("convertio status error: %s", result.Error)
	}

	res := &provider.ConvertResult{
		JobID:    jobID,
		Provider: "convertio",
	}

	switch result.Data.Step {
	case "finish":
		res.Status = "finished"
		res.DownloadURL = result.Data.Output.URL
		res.Filename = result.Data.Output.Filename
		// kalau filename kosong, generate dari URL output
		if res.Filename == "" && res.DownloadURL != "" {
			parts := strings.Split(res.DownloadURL, "/")
			if len(parts) > 0 {
				res.Filename = parts[len(parts)-1]
			}
		}
		if size, err := strconv.ParseInt(result.Data.Output.Size, 10, 64); err == nil {
			res.Size = size
		}
	case "error":
		res.Status = "error"
		res.Message = result.Error
	default:
		// converting, waiting, dll
		res.Status = "processing"
	}

	return res, nil
}
