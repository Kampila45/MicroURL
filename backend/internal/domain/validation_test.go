package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateDestinationURL(t *testing.T) {
	t.Parallel()

	longValid := "https://example.com/" + strings.Repeat("a", MaxURLLength-20)
	tooLong := "https://example.com/" + strings.Repeat("a", MaxURLLength)

	tests := []struct {
		name    string
		input   string
		wantOK  bool
		wantOut string
	}{
		{
			name:    "valid https url",
			input:   "https://example.com/docs",
			wantOK:  true,
			wantOut: "https://example.com/docs",
		},
		{
			name:    "trims surrounding whitespace",
			input:   "  http://example.com/path  ",
			wantOK:  true,
			wantOut: "http://example.com/path",
		},
		{
			name:    "mixed-case scheme with port query fragment and encoded input",
			input:   "HTTPS://Example.COM:8443/search?q=hello%20world&tag=go#section-2",
			wantOK:  true,
			wantOut: "HTTPS://Example.COM:8443/search?q=hello%20world&tag=go#section-2",
		},
		{
			name:    "public ipv4 destination",
			input:   "https://1.1.1.1/dns-query",
			wantOK:  true,
			wantOut: "https://1.1.1.1/dns-query",
		},
		{
			name:    "public ipv6 destination",
			input:   "https://[2606:4700:4700::1111]/",
			wantOK:  true,
			wantOut: "https://[2606:4700:4700::1111]/",
		},
		{
			name:    "punycode IDN domain allowed",
			input:   "https://xn--mnchen-3ya.de/info",
			wantOK:  true,
			wantOut: "https://xn--mnchen-3ya.de/info",
		},
		{
			name:   "non-ASCII homograph host rejected",
			input:  "https://\u0430pple.com/login",
			wantOK: false,
		},
		{
			name:    "max length boundary allowed",
			input:   longValid,
			wantOK:  true,
			wantOut: longValid,
		},
		{
			name:   "empty url rejected",
			input:  "   ",
			wantOK: false,
		},
		{
			name:   "overlong url rejected",
			input:  tooLong,
			wantOK: false,
		},
		{
			name:   "javascript scheme rejected",
			input:  "javascript:alert(1)",
			wantOK: false,
		},
		{
			name:   "data scheme rejected",
			input:  "data:text/html,<script>alert(1)</script>",
			wantOK: false,
		},
		{
			name:   "file scheme rejected",
			input:  "file:///etc/passwd",
			wantOK: false,
		},
		{
			name:   "ftp scheme rejected",
			input:  "ftp://files.example.com/archive.tar.gz",
			wantOK: false,
		},
		{
			name:   "embedded credentials rejected",
			input:  "https://user:secret@example.com/private",
			wantOK: false,
		},
		{
			name:   "missing host rejected",
			input:  "https:///missing-host",
			wantOK: false,
		},
		{
			name:   "localhost rejected",
			input:  "http://localhost:8080/callback",
			wantOK: false,
		},
		{
			name:   "subdomain localhost rejected",
			input:  "http://api.localhost/test",
			wantOK: false,
		},
		{
			name:   "dot local hostname rejected",
			input:  "https://printer.local/admin",
			wantOK: false,
		},
		{
			name:   "single label intranet hostname rejected",
			input:  "http://intranet/dashboard",
			wantOK: false,
		},
		{
			name:   "loopback ipv4 rejected",
			input:  "http://127.0.0.1:8080/",
			wantOK: false,
		},
		{
			name:   "private ipv4 10/8 rejected",
			input:  "http://10.0.0.5/internal",
			wantOK: false,
		},
		{
			name:   "link-local metadata ipv4 rejected",
			input:  "http://169.254.169.254/latest/meta-data/",
			wantOK: false,
		},
		{
			name:   "loopback ipv6 rejected",
			input:  "http://[::1]:8080/",
			wantOK: false,
		},
		{
			name:   "embedded space rejected",
			input:  "https://example.com/has space",
			wantOK: false,
		},
		{
			name:   "invalid port rejected",
			input:  "https://example.com:99999/test",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ValidateDestinationURL(tt.input)
			if tt.wantOK {
				if err != nil {
					t.Fatalf("ValidateDestinationURL(%q) unexpected error: %v", tt.input, err)
				}
				if got != tt.wantOut {
					t.Fatalf("ValidateDestinationURL(%q) = %q, want %q", tt.input, got, tt.wantOut)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateDestinationURL(%q) expected error, got %q", tt.input, got)
			}
			if !errors.Is(err, ErrInvalidURL) {
				t.Fatalf("ValidateDestinationURL(%q) error = %v, want ErrInvalidURL", tt.input, err)
			}
		})
	}
}

func TestValidateCustomAlias(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantOK  bool
		wantOut string
	}{
		{
			name:    "valid lowercase alias",
			input:   "portfolio-2026",
			wantOK:  true,
			wantOut: "portfolio-2026",
		},
		{
			name:    "mixed-case and whitespace normalized to lowercase",
			input:   "  My_Brand_Link  ",
			wantOK:  true,
			wantOut: "my_brand_link",
		},
		{
			name:    "minimum length 3 allowed",
			input:   "abc",
			wantOK:  true,
			wantOut: "abc",
		},
		{
			name:   "too short rejected",
			input:  "ab",
			wantOK: false,
		},
		{
			name:   "too long rejected",
			input:  strings.Repeat("a", MaxAliasLength+1),
			wantOK: false,
		},
		{
			name:   "reserved api rejected case-insensitively",
			input:  "API",
			wantOK: false,
		},
		{
			name:   "reserved health rejected",
			input:  "health",
			wantOK: false,
		},
		{
			name:   "reserved docs rejected",
			input:  "docs",
			wantOK: false,
		},
		{
			name:   "reserved stats rejected",
			input:  "stats",
			wantOK: false,
		},
		{
			name:   "leading hyphen rejected",
			input:  "-invalid",
			wantOK: false,
		},
		{
			name:   "trailing underscore rejected",
			input:  "invalid_",
			wantOK: false,
		},
		{
			name:   "slash in alias rejected",
			input:  "bad/slug",
			wantOK: false,
		},
		{
			name:   "space inside alias rejected",
			input:  "my link",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ValidateCustomAlias(tt.input)
			if tt.wantOK {
				if err != nil {
					t.Fatalf("ValidateCustomAlias(%q) unexpected error: %v", tt.input, err)
				}
				if got != tt.wantOut {
					t.Fatalf("ValidateCustomAlias(%q) = %q, want %q", tt.input, got, tt.wantOut)
				}
				return
			}
			if !errors.Is(err, ErrInvalidAlias) {
				t.Fatalf("ValidateCustomAlias(%q) error = %v, want ErrInvalidAlias", tt.input, err)
			}
		})
	}
}

