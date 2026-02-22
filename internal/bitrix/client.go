package bitrix

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	baseURL = strings.TrimSpace(baseURL)
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: time.Duration(25) * time.Second,
		},
	}
}

func (c *Client) Call(ctx context.Context, method string, payload any, out any) error {
	method = strings.TrimSpace(method)
	if method == "" {
		return fmt.Errorf("method is empty")
	}

	if !strings.HasSuffix(method, ".json") {
		method += ".json"
	}

	url := c.baseURL + method

	var bodyBytes []byte
	var err error
	if payload == nil {
		bodyBytes = []byte(`{}`)
	} else {
		bodyBytes, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	var apiErr APIError
	_ = json.Unmarshal(raw, &apiErr)
	if !apiErr.IsZero() {
		return apiErr
	}

	if out == nil {
		return nil
	}

	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("unmarshal response: %w; raw=%s", err, string(raw))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http status %d: %s", resp.StatusCode, string(raw))
	}

	return nil
}
