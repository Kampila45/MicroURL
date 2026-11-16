package http

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// ClientIPResolver derives a canonical client IP address while only honoring
// proxy forwarding headers when the direct peer matches TrustedProxies.
type ClientIPResolver struct {
	trustedProxies []netip.Prefix
}

// NewClientIPResolver builds a resolver from pre-validated trusted proxy prefixes.
func NewClientIPResolver(trusted []netip.Prefix) *ClientIPResolver {
	copied := make([]netip.Prefix, 0, len(trusted))
	for _, prefix := range trusted {
		addr := prefix.Addr().Unmap()
		bits := prefix.Bits()
		// Normalize 4-in-6 /96+/128 prefixes onto canonical IPv4 prefixes where appropriate.
		if prefix.Addr().Is4In6() && bits >= 96 {
			bits -= 96
		}
		copied = append(copied, netip.PrefixFrom(addr, bits).Masked())
	}
	return &ClientIPResolver{trustedProxies: copied}
}

// Resolve returns the normalized client IP string for rate limiting and Turnstile checks.
func (c *ClientIPResolver) Resolve(r *http.Request) string {
	peerAddr, hasPeer := parseIPFromAddr(r.RemoteAddr)
	if !hasPeer {
		return ""
	}

	if !c.isTrusted(peerAddr) {
		return peerAddr.String()
	}

	if clientIP, ok := c.resolveFromForwardedFor(r.Header.Values("X-Forwarded-For")); ok {
		return clientIP.String()
	}

	if realIPHeader := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIPHeader != "" {
		if realAddr, ok := parseSingleIP(realIPHeader); ok && !c.isTrusted(realAddr) {
			return realAddr.String()
		}
	}

	return peerAddr.String()
}

func (c *ClientIPResolver) resolveFromForwardedFor(headers []string) (netip.Addr, bool) {
	if len(headers) == 0 {
		return netip.Addr{}, false
	}

	var hops []string
	for _, header := range headers {
		for _, part := range strings.Split(header, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				hops = append(hops, trimmed)
			}
		}
	}

	// Walk right-to-left so spoofed left-side entries from clients cannot override
	// the hop recorded by our trusted reverse-proxy chain.
	for i := len(hops) - 1; i >= 0; i-- {
		addr, ok := parseSingleIP(hops[i])
		if !ok {
			return netip.Addr{}, false
		}
		if c.isTrusted(addr) {
			continue
		}
		return addr, true
	}

	return netip.Addr{}, false
}

func (c *ClientIPResolver) isTrusted(addr netip.Addr) bool {
	for _, prefix := range c.trustedProxies {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func parseIPFromAddr(remoteAddr string) (netip.Addr, bool) {
	trimmed := strings.TrimSpace(remoteAddr)
	if trimmed == "" {
		return netip.Addr{}, false
	}
	if host, _, err := net.SplitHostPort(trimmed); err == nil {
		return parseSingleIP(host)
	}
	return parseSingleIP(trimmed)
}

func parseSingleIP(raw string) (netip.Addr, bool) {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return netip.Addr{}, false
	}
	addr, err := netip.ParseAddr(cleaned)
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

