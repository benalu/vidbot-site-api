package cloudconvert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

const baseURL = "https://api.cloudconvert.com/v2"

type Client struct {
	apiKey     string
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

type Task struct {
	Operation    string                 `json:"operation"`
	URL          string                 `json:"url,omitempty"`
	Input        interface{}            `json:"input,omitempty"`
	OutputFormat string                 `json:"output_format,omitempty"`
	Options      map[string]interface{} `json:"options,omitempty"`
}

type JobRequest struct {
	Tag   string          `json:"tag,omitempty"`
	Tasks map[string]Task `json:"tasks"`
}

type TaskResult struct {
	Files []struct {
		Filename string `json:"filename"`
		Size     int64  `json:"size"`
		URL      string `json:"url"`
	} `json:"files"`
}

type TaskStatus struct {
	ID      string      `json:"id"`
	Name    string      `json:"name"`
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Result  *TaskResult `json:"result"`
}

type JobStatus struct {
	ID     string       `json:"id"`
	Status string       `json:"status"`
	Tasks  []TaskStatus `json:"tasks"`
}

type jobResponse struct {
	Data JobStatus `json:"data"`
}

type TaskDetail struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Result *struct {
		Form struct {
			URL        string            `json:"url"`
			Parameters map[string]string `json:"parameters"`
		} `json:"form"`
	} `json:"result"`
}

type taskDetailResponse struct {
	Data TaskDetail `json:"data"`
}

func (c *Client) GetTask(taskID string) (*TaskDetail, error) {
	httpReq, err := http.NewRequest("GET", baseURL+"/tasks/"+taskID, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("cloudconvert error %d: %s", resp.StatusCode, string(b))
	}

	var result taskDetailResponse
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func (c *Client) CreateJob(req JobRequest) (*JobStatus, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", baseURL+"/jobs", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("cloudconvert error %d: %s", resp.StatusCode, string(b))
	}

	var result jobResponse
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func (c *Client) GetJob(jobID string) (*JobStatus, error) {
	httpReq, err := http.NewRequest("GET", baseURL+"/jobs/"+jobID, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("cloudconvert error %d: %s", resp.StatusCode, string(b))
	}

	var result jobResponse
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func (c *Client) CreateUploadJob(outputFormat, tag string) (*JobStatus, error) {
	body, err := json.Marshal(JobRequest{
		Tag: tag,
		Tasks: map[string]Task{
			"import-file": {
				Operation: "import/upload",
			},
			"convert-file": {
				Operation:    "convert",
				Input:        "import-file",
				OutputFormat: outputFormat,
			},
			"export-file": {
				Operation: "export/url",
				Input:     "convert-file",
			},
		},
	})
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", baseURL+"/jobs", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("cloudconvert error %d: %s", resp.StatusCode, string(b))
	}

	var result jobResponse
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func (c *Client) UploadFile(uploadURL string, parameters map[string]string, filename string, fileData []byte) error {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// tambah parameters dulu (harus sebelum file)
	for key, val := range parameters {
		writer.WriteField(key, val)
	}

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return err
	}
	if _, err := part.Write(fileData); err != nil {
		return err
	}
	writer.Close()

	httpReq, err := http.NewRequest("POST", uploadURL, body)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 && resp.StatusCode != 204 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload error %d: %s", resp.StatusCode, string(b))
	}
	return nil
}
