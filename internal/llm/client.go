package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	endpoint       string
	apikey         string
	model          string
	maxRetries     int
	retryBaseDelay time.Duration
	httpClient     *http.Client
}

type streamOpts struct {
	IncludeUsage bool `json:"include_usage"`
}

type ChatRequest struct {
	Model         string      `json:"model"`
	Messages      []Message   `json:"messages"`
	Tools         []ToolDef   `json:"tools,omitempty"`
	Stream        bool        `json:"stream"`
	StreamOptions *streamOpts `json:"stream_options,omitempty"`
}

type ApiError struct {
	StatusCode int
	Body       string
}

func (e *ApiError) Error() string {
	return fmt.Sprintf("llm api error: status %d: %s", e.StatusCode, e.Body)
}

func NewClient(cfg Config) (*Client, error) {
	httpClient := cfg.HttpCli
	if httpClient == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.ResponseHeaderTimeout = time.Second * 60
		httpClient = &http.Client{Transport: transport}
	}

	return &Client{
		endpoint:       strings.TrimRight(cfg.BaseURL, "/") + "/chat/completions",
		apikey:         cfg.ApiKey,
		model:          cfg.Model,
		maxRetries:     cfg.MaxRetry,
		retryBaseDelay: 500 * time.Millisecond,
		httpClient:     httpClient,
	}, nil
}

func (c *Client) Stream(ctx context.Context, messages []Message, tools []ToolDef, onText func(string)) (*Response, error) {
	body, err := json.Marshal(ChatRequest{
		Model:         c.model,
		Messages:      messages,
		Tools:         tools,
		Stream:        true,
		StreamOptions: &streamOpts{IncludeUsage: true},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal chat: %w", err)
	}

	resp, err := c.postWithRetry(ctx, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return readStream(resp.Body, onText)
}

func (c *Client) postWithRetry(ctx context.Context, body []byte) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := c.retryBaseDelay<<uint(attempt) - 1
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		resp, err := c.postWithJson(ctx, body)
		if err == nil {
			return resp, nil
		}
		if ctx.Err() != nil || !isRetryableError(err) {
			return nil, err
		}
		lastErr = err
	}

	return nil, fmt.Errorf("post with retry=%d: %w", c.maxRetries, lastErr)
}

func (c *Client) postWithJson(ctx context.Context, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if c.apikey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apikey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		msg, _ := ioutil.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return nil, &ApiError{
			StatusCode: resp.StatusCode,
			Body:       string(msg),
		}
	}

	return resp, nil
}

func isRetryableError(err error) bool {
	apiErr := &ApiError{}
	if ok := errors.As(err, apiErr); ok {
		if apiErr.StatusCode == 429 || apiErr.StatusCode >= 500 {
			return true
		}
	}

	return true
}
