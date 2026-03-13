package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"
)

type Client struct {
	workers      []string
	workerSecret string
	client       *http.Client
}

func NewClient(workerURLs []string, workerSecret string) *Client {
	return &Client{
		workers:      workerURLs,
		workerSecret: workerSecret,
		client:       &http.Client{Timeout: 30 * time.Second},
	}
}

type workerRequest struct {
	URL      string            `json:"url"`
	Method   string            `json:"method"`
	Headers  map[string]string `json:"headers,omitempty"`
	Body     string            `json:"body,omitempty"`
	Redirect string            `json:"redirect,omitempty"`
}

type WorkerResponse struct {
	Status     int               `json:"status"`
	StatusText string            `json:"statusText"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
}

func (c *Client) doRequest(workerURL, method, targetURL string, headers map[string]string, body, redirect string) (*WorkerResponse, error) {
	payload := workerRequest{
		URL:      targetURL,
		Method:   method,
		Headers:  headers,
		Body:     body,
		Redirect: redirect,
	}

	payloadBytes, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", workerURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("proxy request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Worker-Secret", c.workerSecret)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("proxy do: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)

	var workerResp WorkerResponse
	if err := json.Unmarshal(respBytes, &workerResp); err != nil {
		return nil, fmt.Errorf("proxy parse: %w", err)
	}

	return &workerResp, nil
}

func (c *Client) Do(method, targetURL string, headers map[string]string, body, redirect string) (*WorkerResponse, error) {
	if len(c.workers) == 0 {
		return nil, fmt.Errorf("no workers configured")
	}

	// acak urutan worker, coba satu per satu sampai berhasil
	indices := rand.Perm(len(c.workers))
	maxTry := min(3, len(c.workers))

	var lastErr error
	for i := 0; i < maxTry; i++ {
		workerURL := c.workers[indices[i]]
		resp, err := c.doRequest(workerURL, method, targetURL, headers, body, redirect)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("all workers failed: %w", lastErr)
}

func (c *Client) Get(url string, headers map[string]string) (*WorkerResponse, error) {
	return c.Do("GET", url, headers, "", "follow")
}

func (c *Client) GetNoRedirect(url string, headers map[string]string) (*WorkerResponse, error) {
	return c.Do("GET", url, headers, "", "manual")
}

func (c *Client) PickWorker() string {
	return c.workers[rand.Intn(len(c.workers))]
}

func (c *Client) DoFromWorker(workerURL, method, targetURL string, headers map[string]string, body, redirect string) (*WorkerResponse, error) {
	return c.doRequest(workerURL, method, targetURL, headers, body, redirect)
}

func (c *Client) GetFromWorker(workerURL, targetURL string, headers map[string]string) (*WorkerResponse, error) {
	return c.DoFromWorker(workerURL, "GET", targetURL, headers, "", "follow")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
