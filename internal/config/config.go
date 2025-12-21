package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL      string
	BaseURL          string
	Port             string
	RateLimitRPS     float64
	RateLimitBurst   int
	CacheServers     []string
	CacheTTL         int
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		BaseURL:     os.Getenv("BASE_URL"),
		Port:        os.Getenv("PORT"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("BASE_URL environment variable is required")
	}

	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	rps := os.Getenv("RATE_LIMIT_RPS")
	if rps == "" {
		cfg.RateLimitRPS = 10.0
	} else {
		var err error
		cfg.RateLimitRPS, err = parseFloat64(rps)
		if err != nil {
			return nil, fmt.Errorf("invalid RATE_LIMIT_RPS: %w", err)
		}
		if cfg.RateLimitRPS <= 0 {
			return nil, fmt.Errorf("RATE_LIMIT_RPS must be greater than 0")
		}
	}

	burst := os.Getenv("RATE_LIMIT_BURST")
	if burst == "" {
		cfg.RateLimitBurst = 20
	} else {
		var err error
		cfg.RateLimitBurst, err = parseInt(burst)
		if err != nil {
			return nil, fmt.Errorf("invalid RATE_LIMIT_BURST: %w", err)
		}
		if cfg.RateLimitBurst <= 0 {
			return nil, fmt.Errorf("RATE_LIMIT_BURST must be greater than 0")
		}
	}

	cacheServers := os.Getenv("CACHE_SERVERS")
	if cacheServers == "" {
		cfg.CacheServers = []string{"localhost:11211"}
	} else {
		cfg.CacheServers = splitString(cacheServers, ",")
	}

	ttl := os.Getenv("CACHE_TTL")
	if ttl == "" {
		cfg.CacheTTL = 3600
	} else {
		var err error
		cfg.CacheTTL, err = parseInt(ttl)
		if err != nil {
			return nil, fmt.Errorf("invalid CACHE_TTL: %w", err)
		}
		if cfg.CacheTTL <= 0 {
			return nil, fmt.Errorf("CACHE_TTL must be greater than 0")
		}
	}

	return cfg, nil
}

func splitString(s, sep string) []string {
	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func parseFloat64(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

func parseInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}

