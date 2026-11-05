// Package ssrf guards the server's outbound fetches against server-side
// request forgery: article and feed URLs ultimately come from users, so
// dialing is restricted to public IPs.
package ssrf

import (
	"context"
	"fmt"
	"net"
)

// DialPublicIP is an http.Transport DialContext func that only dials public
// IP addresses. Article URLs are user-controlled, so without this the scraper
// would follow a link to a loopback, LAN, link-local (including the cloud
// metadata endpoints), or multicast address — server-side request forgery.
// The hostname is resolved and every address it resolves to must be public;
// dialing then proceeds against the hostname so TLS ServerName verification
// keeps working. That re-resolution leaves a narrow DNS-rebinding window,
// accepted here because this reader fetches news outlets, not
// attacker-controlled DNS.
//
// DialPublicIP must see the article host itself: callers must dial origins
// directly and never route the scraper through a proxy, or the check would
// vet the proxy's address while the private destination goes unchecked.
func DialPublicIP(ctx context.Context, network, address string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("split host and port: %w", err)
	}

	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", host, err)
	}

	if !publicIPs(ips) {
		return nil, fmt.Errorf("refusing to fetch non-public address for %s", host)
	}

	var dialer net.Dialer

	return dialer.DialContext(ctx, network, address)
}

// cgnat is the RFC 6598 shared address space (100.64.0.0/10). Go's
// IsGlobalUnicast accepts it and IsPrivate does not cover it, yet it is
// not publicly routable, so it must be refused like the other ranges.
var cgnat = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

// publicIPs reports whether every address is a routable public IP: global
// unicast, and none of private, loopback, link-local, multicast,
// unspecified, or carrier-grade NAT. An empty set is not public.
func publicIPs(ips []net.IP) bool {
	if len(ips) == 0 {
		return false
	}

	for _, ip := range ips {
		if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() ||
			ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
			ip.IsUnspecified() || cgnat.Contains(ip) {
			return false
		}
	}

	return true
}
