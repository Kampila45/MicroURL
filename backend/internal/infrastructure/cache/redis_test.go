package cache

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"url-shortener/internal/application"
	"url-shortener/internal/domain"
)

func TestUpstashRedisAdapter_SetGetDelete(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	expectedLink := &domain.Link{
		ID:          "link-42",
		Slug:        "docs",
		OriginalURL: "https://example.com/docs",
		ClickCount:  7,
		CreatedAt:   now,
	}

	store := make(map[string]string)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret-token" {
			t.Fatalf("Authorization = %q, want Bearer secret-token", got)
		}

		var cmd []json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
			t.Fatalf("decode command: %v", err)
		}
		if len(cmd) < 2 {
			t.Fatalf("unexpected command length: %d", len(cmd))
		}

		var op, key string
		_ = json.Unmarshal(cmd[0], &op)
		_ = json.Unmarshal(cmd[1], &key)

		w.Header().Set("Content-Type", "application/json")
		switch op {
		case "SET":
			var val string
			_ = json.Unmarshal(cmd[2], &val)
			if len(cmd) != 5 {
				t.Fatalf("SET command len = %d, want 5 with EX", len(cmd))
			}
			var exFlag string
			var ttl int
			_ = json.Unmarshal(cmd[3], &exFlag)
			_ = json.Unmarshal(cmd[4], &ttl)
			if exFlag != "EX" || ttl != 3600 {
				t.Fatalf("SET expiry = %q %d, want EX 3600", exFlag, ttl)
			}
			store[key] = val
			_, _ = w.Write([]byte(`{"result":"OK"}`))
		case "GET":
			val, ok := store[key]
			if !ok {
				_, _ = w.Write([]byte(`{"result":null}`))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"result": val})
		case "DEL":
			if _, ok := store[key]; ok {
				delete(store, key)
				_, _ = w.Write([]byte(`{"result":1}`))
				return
			}
			_, _ = w.Write([]byte(`{"result":0}`))
		default:
			t.Fatalf("unexpected redis command: %s", op)
		}
	}))
	defer srv.Close()

	adapter := NewUpstashRedisAdapter(srv.URL+"/", "secret-token")
	ctx := context.Background()

	if _, err := adapter.Get(ctx, "link:slug:docs"); !errors.Is(err, application.ErrCacheMiss) {
		t.Fatalf("Get on empty cache err = %v, want ErrCacheMiss", err)
	}

	if err := adapter.Set(ctx, "link:slug:docs", expectedLink, 3600); err != nil {
		t.Fatalf("Set error: %v", err)
	}

	got, err := adapter.Get(ctx, "link:slug:docs")
	if err != nil {
		t.Fatalf("Get hit error: %v", err)
	}
	if got.ID != expectedLink.ID || got.Slug != expectedLink.Slug || got.OriginalURL != expectedLink.OriginalURL || got.ClickCount != expectedLink.ClickCount {
		t.Fatalf("got link %+v, want %+v", got, expectedLink)
	}

	if err := adapter.Delete(ctx, "link:slug:docs"); err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	// Deleting missing key remains idempotent.
	if err := adapter.Delete(ctx, "link:slug:docs"); err != nil {
		t.Fatalf("Delete missing key error: %v", err)
	}
	if _, err := adapter.Get(ctx, "link:slug:docs"); !errors.Is(err, application.ErrCacheMiss) {
		t.Fatalf("Get after Delete err = %v, want ErrCacheMiss", err)
	}
}

func TestUpstashRedisAdapter_ErrorsAndDegradedMode(t *testing.T) {
	ctx := context.Background()

	unconfigured := NewUpstashRedisAdapter("", "")
	if _, err := unconfigured.Get(ctx, "k"); !errors.Is(err, ErrCacheNotConfigured) {
		t.Fatalf("unconfigured Get err = %v, want ErrCacheNotConfigured", err)
	}
	if err := unconfigured.Set(ctx, "k", &domain.Link{Slug: "k"}, 60); !errors.Is(err, ErrCacheNotConfigured) {
		t.Fatalf("unconfigured Set err = %v, want ErrCacheNotConfigured", err)
	}
	if err := unconfigured.Delete(ctx, "k"); !errors.Is(err, ErrCacheNotConfigured) {
		t.Fatalf("unconfigured Delete err = %v, want ErrCacheNotConfigured", err)
	}

	errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"WRONGPASS invalid or missing auth token"}`))
	}))
	defer errServer.Close()

	adapter := NewUpstashRedisAdapter(errServer.URL, "bad-token")
	if _, err := adapter.Get(ctx, "k"); err == nil || errors.Is(err, application.ErrCacheMiss) || !strings.Contains(err.Error(), "WRONGPASS") {
		t.Fatalf("auth failure Get err = %v, want non-ErrCacheMiss WRONGPASS error", err)
	}
}
