package http

import (
	"net/http/httptest"
	"net/netip"
	"testing"
)

func mustPrefix(t *testing.T, s string) netip.Prefix {
	t.Helper()
	if p, err := netip.ParsePrefix(s); err == nil {
		return p
	}
	addr, err := netip.ParseAddr(s)
	if err != nil {
		t.Fatalf("failed to parse prefix/addr %q: %v", s, err)
	}
	return netip.PrefixFrom(addr, addr.BitLen())
}

func TestClientIPResolver(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		trusted        []netip.Prefix
		remoteAddr     string
		xffHeaders     []string
		xRealIP        string
		wantClientIP   string
	}{
		{
			name:         "untrusted peer ignores X-Forwarded-For and X-Real-IP",
			trusted:      nil,
			remoteAddr:   "198.51.100.42:51234",
			xffHeaders:   []string{"203.0.113.9"},
			xRealIP:      "203.0.113.10",
			wantClientIP: "198.51.100.42",
		},
		{
			name:         "non-matching peer ignores spoofed X-Forwarded-For",
			trusted:      []netip.Prefix{mustPrefix(t, "10.0.0.0/8")},
			remoteAddr:   "198.51.100.42:51234",
			xffHeaders:   []string{"1.2.3.4"},
			wantClientIP: "198.51.100.42",
		},
		{
			name:         "single trusted proxy honors X-Forwarded-For",
			trusted:      []netip.Prefix{mustPrefix(t, "127.0.0.1")},
			remoteAddr:   "127.0.0.1:41000",
			xffHeaders:   []string{"203.0.113.55"},
			wantClientIP: "203.0.113.55",
		},
		{
			name:         "multi-hop trusted proxy skips trusted hops and ignores left-side spoof",
			trusted:      []netip.Prefix{mustPrefix(t, "10.0.0.0/8"), mustPrefix(t, "172.16.0.0/12")},
			remoteAddr:   "10.0.0.5:8080",
			xffHeaders:   []string{"9.9.9.9, 203.0.113.77", "172.16.0.2, 10.0.0.4"},
			wantClientIP: "203.0.113.77",
		},
		{
			name:         "normalizes IPv4-mapped IPv6 peer and forwarded IP",
			trusted:      []netip.Prefix{mustPrefix(t, "127.0.0.1")},
			remoteAddr:   "[::ffff:127.0.0.1]:50000",
			xffHeaders:   []string{"::ffff:203.0.113.88"},
			wantClientIP: "203.0.113.88",
		},
		{
			name:         "supports native IPv6 client and trusted proxy",
			trusted:      []netip.Prefix{mustPrefix(t, "fc00::/7")},
			remoteAddr:   "[fd00::10]:44321",
			xffHeaders:   []string{"2001:db8::1"},
			wantClientIP: "2001:db8::1",
		},
		{
			name:         "malformed X-Forwarded-For falls back safely to X-Real-IP then peer",
			trusted:      []netip.Prefix{mustPrefix(t, "10.0.0.0/8")},
			remoteAddr:   "10.0.0.2:1234",
			xffHeaders:   []string{"garbage-ip"},
			xRealIP:      "203.0.113.21",
			wantClientIP: "203.0.113.21",
		},
		{
			name:         "malformed X-Forwarded-For and malformed X-Real-IP fall back to peer",
			trusted:      []netip.Prefix{mustPrefix(t, "10.0.0.0/8")},
			remoteAddr:   "10.0.0.2:1234",
			xffHeaders:   []string{"not-an-ip"},
			xRealIP:      "also-not-an-ip",
			wantClientIP: "10.0.0.2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resolver := NewClientIPResolver(tt.trusted)
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr
			for _, xff := range tt.xffHeaders {
				req.Header.Add("X-Forwarded-For", xff)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}

			if got := resolver.Resolve(req); got != tt.wantClientIP {
				t.Fatalf("Resolve() = %q, want %q", got, tt.wantClientIP)
			}
		})
	}
}

