package phajay

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const DefaultBaseURL = "https://payment-gateway.phajay.co/v1/api"

type authMode int

const (
	authSecretHeader authMode = iota
	authBasic
)

type Config struct {
	SecretKey  string
	IsSandbox  bool
	BaseURL    string
	HTTPClient *http.Client
}

type Client struct {
	secretKey  string
	isSandbox  bool
	baseURL    string
	httpClient *http.Client
}

type APIError struct {
	StatusCode int
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("phajay: %s (status %d)", e.Message, e.StatusCode)
	}
	return fmt.Sprintf("phajay: request failed with status %d", e.StatusCode)
}

// check config
func New(cfg Config) (*Client, error) {
	key := strings.TrimSpace(cfg.SecretKey)
	if key == "" {
		return nil, errors.New("phajay: secret key is required")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{
		secretKey:  key,
		isSandbox:  cfg.IsSandbox,
		baseURL:    baseURL,
		httpClient: httpClient,
	}, nil
}

// read from env
func NewFromEnv() (*Client, error) {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("PHAJAY_ENV")))
	return New(Config{
		SecretKey: os.Getenv("PHAJAY_SECRET_KEY"),
		IsSandbox: env == "sandbox" || env == "test",
		BaseURL:   os.Getenv("PHAJAY_BASE_URL"),
	})
}

// check if the client is in sandbox mode or not
func (c *Client) IsSandbox() bool { return c.isSandbox }

func (c *Client) do(ctx context.Context, method, path string, mode authMode, payload, out any) error {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("phajay: marshal request: %w", err)
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("phajay: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	switch mode {
	case authSecretHeader:
		req.Header.Set("secretKey", c.secretKey)
	case authBasic:
		token := base64.StdEncoding.EncodeToString([]byte(c.secretKey))
		req.Header.Set("Authorization", "Basic "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("phajay: send request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("phajay: read response: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		var errBody struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(respBytes, &errBody)
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    errBody.Message,
			Body:       string(respBytes),
		}
	}

	if out != nil && len(respBytes) > 0 {
		if err := json.Unmarshal(respBytes, out); err != nil {
			return fmt.Errorf("phajay: decode response: %w", err)
		}
	}
	return nil
}
