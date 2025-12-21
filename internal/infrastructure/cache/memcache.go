package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"url-shortener/internal/application"
	"url-shortener/internal/domain"

	"github.com/bradfitz/gomemcache/memcache"
)

type MemcacheAdapter struct {
	client *memcache.Client
}

func NewMemcacheAdapter(servers []string) application.Cache {
	client := memcache.New(servers...)
	return &MemcacheAdapter{client: client}
}

func (m *MemcacheAdapter) Get(ctx context.Context, key string) (*domain.Link, error) {
	item, err := m.client.Get(key)
	if err != nil {
		if err == memcache.ErrCacheMiss {
			return nil, application.ErrCacheMiss
		}
		return nil, fmt.Errorf("cache get error: %w", err)
	}

	var link domain.Link
	if err := json.Unmarshal(item.Value, &link); err != nil {
		return nil, fmt.Errorf("cache unmarshal error: %w", err)
	}

	return &link, nil
}

func (m *MemcacheAdapter) Set(ctx context.Context, key string, link *domain.Link, ttl int) error {
	data, err := json.Marshal(link)
	if err != nil {
		return fmt.Errorf("cache marshal error: %w", err)
	}

	item := &memcache.Item{
		Key:        key,
		Value:      data,
		Expiration: int32(ttl),
	}

	if err := m.client.Set(item); err != nil {
		return fmt.Errorf("cache set error: %w", err)
	}

	return nil
}

func (m *MemcacheAdapter) Delete(ctx context.Context, key string) error {
	if err := m.client.Delete(key); err != nil && err != memcache.ErrCacheMiss {
		return fmt.Errorf("cache delete error: %w", err)
	}
	return nil
}

