package webhook

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"time"
)

// isForbiddenIP reports whether ip is in a range that must never be contacted
// for webhook delivery (SSRF protection).
func isForbiddenIP(ip netip.Addr) bool {
	if !ip.IsValid() {
		return true
	}
	switch {
	case ip.IsLoopback(),
		ip.IsLinkLocalUnicast(),
		ip.IsLinkLocalMulticast(),
		ip.IsPrivate(),
		ip.IsUnspecified(),
		ip.IsMulticast():
		return true
	}
	// Also catch IPv4-mapped IPv6 of all the above.
	if ip.Is4In6() {
		return isForbiddenIP(netip.AddrFrom4(ip.As4()))
	}
	return false
}

// IsForbiddenHost reports whether host is an IP literal in a forbidden range.
// Hostnames (non-IP) always return false — they are validated at DNS
// resolution time in resolveDialHostForConfig. This is a fast early-reject for
// defense-in-depth in the setWebhook handler.
func IsForbiddenHost(host string) bool {
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return false // not an IP literal — will be checked after DNS resolution
	}
	return isForbiddenIP(ip)
}

// ssrfDialContext returns a DialContext that resolves the hostname and rejects
// connections to forbidden (internal/private/loopback) IPs. The resolved IP is
// used for the actual connection to defeat DNS-rebinding attacks.
//
// Used for the placeholder http.Client created in NewDeliverer (before Start
// installs the full transport with IP caching and dial gates).
func ssrfDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("SSRF: bad address %q: %w", addr, err)
	}

	resolver := &net.Resolver{PreferGo: true}
	addrs, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("SSRF: DNS lookup failed for %q: %w", host, err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("SSRF: no DNS records for %q", host)
	}

	// Check ALL resolved IPs — reject if any is forbidden.
	for _, a := range addrs {
		ip, err := netip.ParseAddr(a.IP.String())
		if err != nil || isForbiddenIP(ip) {
			return nil, fmt.Errorf("SSRF: forbidden IP %s for host %q", a.IP, host)
		}
	}

	// Connect to the first resolved IP directly to prevent rebinding.
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	return dialer.DialContext(ctx, network, net.JoinHostPort(addrs[0].IP.String(), port))
}
