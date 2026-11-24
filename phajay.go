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

type Env string

const (
	EnvProduction Env = "production"
	EnvTest       Env = "test"
	EnvMock       Env = "mock"
)

type authMode int

const (
	authSecretHeader authMode = iota
	authBasic
)

type Config struct {
	SecretKey  string
	Env        Env
	BaseURL    string
	HTTPClient *http.Client
}

type Client struct {
	secretKey  string
	env        Env
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

func normalizeEnv(raw string) Env {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "test", "dev":
		return EnvTest
	case "mock", "sandbox":
		return EnvMock
	default:
		return EnvProduction
	}
}

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
		env:        normalizeEnv(string(cfg.Env)),
		baseURL:    baseURL,
		httpClient: httpClient,
	}, nil
}

func NewFromEnv() (*Client, error) {
	return New(Config{
		SecretKey: os.Getenv("PHAJAY_SECRET_KEY"),
		Env:       normalizeEnv(os.Getenv("PHAJAY_ENV")),
		BaseURL:   os.Getenv("PHAJAY_BASE_URL"),
	})
}

func (c *Client) Env() Env { return c.env }

func (c *Client) validatePayment(amount float64, description string, bank Bank) error {
	if amount <= 0 {
		return errors.New("phajay: amount must be greater than 0")
	}
	desc := strings.TrimSpace(description)
	if desc == "" {
		return errors.New("phajay: description is required")
	}

	if c.env == EnvTest || c.env == EnvMock {
		if amount < 1 || amount > 999 {
			return errors.New("phajay: test mode amount must be between 1 and 999")
		}
		if containsLao(desc) {
			return errors.New("phajay: test mode description does not support Lao characters")
		}
	}

	if bank == BankBCEL && containsLaoOrThai(desc) {
		return errors.New("phajay: bcel description does not support Lao or Thai characters")
	}
	return nil
}

func containsLao(s string) bool {
	for _, r := range s {
		if r >= 0x0E80 && r <= 0x0EFF {
			return true
		}
	}
	return false
}

func containsLaoOrThai(s string) bool {
	for _, r := range s {
		if (r >= 0x0E00 && r <= 0x0E7F) || (r >= 0x0E80 && r <= 0x0EFF) {
			return true
		}
	}
	return false
}

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
