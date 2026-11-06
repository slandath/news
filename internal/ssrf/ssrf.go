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

// nonPublicCIDRs are ranges Go's predicates still call global unicast but
// which are never publicly routable: carrier-grade NAT (RFC 6598),
// benchmarking (RFC 2544), and documentation/test networks (RFC 5737,
// RFC 3849). A deployment that routes one of these internally must refuse
// them like the other special ranges.
var nonPublicCIDRs = []string{
	"100.64.0.0/10",   // RFC 6598 carrier-grade NAT
	"198.18.0.0/15",   // RFC 2544 benchmarking
	"192.0.2.0/24",    // RFC 5737 TEST-NET-1
	"198.51.100.0/24", // RFC 5737 TEST-NET-2
	"203.0.113.0/24",  // RFC 5737 TEST-NET-3
	"2001:db8::/32",   // RFC 3849 documentation
}

// nonPublic holds the parsed nonPublicCIDRs. The literals are fixed, so a
// parse failure is a programming error, not a runtime condition.
var nonPublic = func() []*net.IPNet {
	networks := make([]*net.IPNet, 0, len(nonPublicCIDRs))

	for _, cidr := range nonPublicCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			panic("parse non-public CIDR " + cidr + ": " + err.Error())
		}

		networks = append(networks, network)
	}

	return networks
}()

// publicIPs reports whether every address is a routable public IP: global
// unicast, and none of private, loopback, link-local, multicast,
// unspecified, or the non-public ranges above. An empty set is not public.
func publicIPs(ips []net.IP) bool {
	if len(ips) == 0 {
		return false
	}

	for _, ip := range ips {
		if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() ||
			ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
			ip.IsUnspecified() || isNonPublic(ip) {
			return false
		}
	}

	return true
}

// isNonPublic reports whether ip falls in one of the nonPublicCIDRs.
func isNonPublic(ip net.IP) bool {
	for _, network := range nonPublic {
		if network.Contains(ip) {
			return true
		}
	}

	return false
}
