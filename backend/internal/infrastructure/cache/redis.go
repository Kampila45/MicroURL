package cache

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"url-shortener/internal/application"
	"url-shortener/internal/domain"
)

const defaultCacheRequestTimeout = 2 * time.Second

// ErrCacheNotConfigured is returned when Upstash Redis REST credentials are absent.
var ErrCacheNotConfigured = errors.New("upstash redis cache is not configured")

// UpstashRedisAdapter implements application.Cache over Upstash's Redis REST API.
type UpstashRedisAdapter struct {
	restURL   string
	restToken string
	client    *http.Client
}

type upstashResponse struct {
	Result json.RawMessage `json:"result"`
	Error  string          `json:"error"`
}

// NewUpstashRedisAdapter creates an Upstash Redis cache adapter from REST credentials.
func NewUpstashRedisAdapter(restURL, restToken string) application.Cache {
	return &UpstashRedisAdapter{
		restURL:   strings.TrimRight(strings.TrimSpace(restURL), "/"),
		restToken: strings.TrimSpace(restToken),
		client: &http.Client{
			Timeout: defaultCacheRequestTimeout,
		},
	}
}

func (u *UpstashRedisAdapter) isConfigured() bool {
	return u.restURL != "" && u.restToken != ""
}

func (u *UpstashRedisAdapter) exec(ctx context.Context, command ...any) (json.RawMessage, error) {
	if !u.isConfigured() {
		return nil, ErrCacheNotConfigured
	}

	body, err := json.Marshal(command)
	if err != nil {
		return nil, fmt.Errorf("cache command marshal error: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.restURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("cache request error: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+u.restToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cache request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("cache response read error: %w", err)
	}

	var parsed upstashResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("cache HTTP status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("cache response decode error: %w", err)
	}

	if parsed.Error != "" {
		return nil, fmt.Errorf("upstash redis error: %s", parsed.Error)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("cache HTTP status %d", resp.StatusCode)
	}

	return parsed.Result, nil
}

// Get retrieves a cached link by key, returning application.ErrCacheMiss when absent.
func (u *UpstashRedisAdapter) Get(ctx context.Context, key string) (*domain.Link, error) {
	result, err := u.exec(ctx, "GET", key)
	if err != nil {
		return nil, err
	}

	if len(result) == 0 || bytes.Equal(bytes.TrimSpace(result), []byte("null")) {
		return nil, application.ErrCacheMiss
	}

	var payload string
	if err := json.Unmarshal(result, &payload); err != nil {
		return nil, fmt.Errorf("cache value decode error: %w", err)
	}

	var link domain.Link
	if err := json.Unmarshal([]byte(payload), &link); err != nil {
		return nil, fmt.Errorf("cache unmarshal error: %w", err)
	}

	return &link, nil
}

// Set stores a JSON-serialized link under key with an expiration TTL in seconds.
func (u *UpstashRedisAdapter) Set(ctx context.Context, key string, link *domain.Link, ttl int) error {
	data, err := json.Marshal(link)
	if err != nil {
		return fmt.Errorf("cache marshal error: %w", err)
	}

	if ttl > 0 {
		_, err = u.exec(ctx, "SET", key, string(data), "EX", ttl)
	} else {
		_, err = u.exec(ctx, "SET", key, string(data))
	}
	if err != nil {
		return fmt.Errorf("cache set error: %w", err)
	}

	return nil
}

// Delete removes key from Upstash Redis, ignoring absent keys.
func (u *UpstashRedisAdapter) Delete(ctx context.Context, key string) error {
	if _, err := u.exec(ctx, "DEL", key); err != nil {
		return fmt.Errorf("cache delete error: %w", err)
	}
	return nil
}

