package domain

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

// MaxURLLength caps accepted destination URLs to prevent abuse and overlong redirects.
const MaxURLLength = 2048

const (
	MinAliasLength = 3
	MaxAliasLength = 64
)

var reservedAliases = map[string]struct{}{
	"api":               {},
	"health":            {},
	"docs":              {},
	"openapi.json":      {},
	"robots.txt":        {},
	"favicon.ico":       {},
	"favicon.svg":       {},
	"favicon-32x32.png": {},
	"assets":            {},
	"stats":             {},
	"analytics":         {},
	"manage":            {},
	"404":               {},
	"410":               {},
	"500":               {},
}

// ValidateCustomAlias normalizes a caller-chosen alias to lowercase ASCII and enforces
// length, character-set, and reserved-route constraints.
func ValidateCustomAlias(raw string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if len(normalized) < MinAliasLength || len(normalized) > MaxAliasLength {
		return "", fmt.Errorf("custom alias must be between %d and %d characters: %w", MinAliasLength, MaxAliasLength, ErrInvalidAlias)
	}

	if _, reserved := reservedAliases[normalized]; reserved {
		return "", fmt.Errorf("custom alias uses a reserved path: %w", ErrInvalidAlias)
	}

	for i := 0; i < len(normalized); i++ {
		c := normalized[i]
		isLower := c >= 'a' && c <= 'z'
		isDigit := c >= '0' && c <= '9'
		isSep := c == '-' || c == '_'
		if !isLower && !isDigit && !isSep {
			return "", fmt.Errorf("custom alias may only contain letters, digits, hyphens, and underscores: %w", ErrInvalidAlias)
		}
		if (i == 0 || i == len(normalized)-1) && isSep {
			return "", fmt.Errorf("custom alias must start and end with a letter or digit: %w", ErrInvalidAlias)
		}
	}

	return normalized, nil
}

// ValidateDestinationURL ensures raw is a safe, public HTTP/HTTPS URL within length bounds.
func ValidateDestinationURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("url is required: %w", ErrInvalidURL)
	}
	if len(trimmed) > MaxURLLength {
		return "", fmt.Errorf("url exceeds %d characters: %w", MaxURLLength, ErrInvalidURL)
	}
	if strings.IndexFunc(trimmed, func(r rune) bool {
		return unicode.IsControl(r) || unicode.IsSpace(r)
	}) != -1 {
		return "", fmt.Errorf("url contains invalid whitespace or control characters: %w", ErrInvalidURL)
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid url format: %w", ErrInvalidURL)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("url scheme must be http or https: %w", ErrInvalidURL)
	}

	if parsed.User != nil {
		return "", fmt.Errorf("url must not contain embedded credentials: %w", ErrInvalidURL)
	}

	if portStr := parsed.Port(); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			return "", fmt.Errorf("invalid url port: %w", ErrInvalidURL)
		}
	}

	host := parsed.Hostname()
	if err := validatePublicHost(host); err != nil {
		return "", fmt.Errorf("%v: %w", err, ErrInvalidURL)
	}

	return trimmed, nil
}

func validatePublicHost(host string) error {
	if host == "" || len(host) > 253 {
		return fmt.Errorf("invalid hostname")
	}

	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() ||
			ip.IsPrivate() ||
			ip.IsLinkLocalUnicast() ||
			ip.IsLinkLocalMulticast() ||
			ip.IsUnspecified() ||
			ip.IsMulticast() {
			return fmt.Errorf("private or loopback IP destinations are not allowed")
		}
		return nil
	}

	cleanHost := strings.TrimSuffix(strings.ToLower(host), ".")
	if cleanHost == "" || strings.HasPrefix(cleanHost, ".") || strings.Contains(cleanHost, "..") {
		return fmt.Errorf("invalid hostname syntax")
	}

	if cleanHost == "localhost" ||
		strings.HasSuffix(cleanHost, ".localhost") ||
		strings.HasSuffix(cleanHost, ".local") ||
		strings.HasSuffix(cleanHost, ".internal") {
		return fmt.Errorf("localhost or internal hostnames are not allowed")
	}

	labels := strings.Split(cleanHost, ".")
	if len(labels) < 2 {
		return fmt.Errorf("hostname must include a domain suffix")
	}

	for _, label := range labels {
		if !isValidHostLabel(label) {
			return fmt.Errorf("invalid hostname label")
		}
	}

	return nil
}

func isValidHostLabel(label string) bool {
	if len(label) == 0 || len(label) > 63 {
		return false
	}
	if label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}
	// Require canonical ASCII LDH labels (allows punycode xn--... while preventing
	// raw non-ASCII IDN homographs in redirect Location headers).
	for i := 0; i < len(label); i++ {
		c := label[i]
		isLower := c >= 'a' && c <= 'z'
		isDigit := c >= '0' && c <= '9'
		if !isLower && !isDigit && c != '-' {
			return false
		}
	}
	return true
}
