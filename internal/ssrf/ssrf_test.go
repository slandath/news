package ssrf

import (
	"context"
	"net"
	"strings"
	"testing"
)

func TestPublicIPs(t *testing.T) {
	tests := []struct {
		name string
		ips  []string
		want bool
	}{
		{name: "empty", ips: nil, want: false},
		{name: "ipv4 loopback", ips: []string{"127.0.0.1"}, want: false},
		{name: "ipv6 loopback", ips: []string{"::1"}, want: false},
		{name: "private 10", ips: []string{"10.1.2.3"}, want: false},
		{name: "private 172.16", ips: []string{"172.16.0.1"}, want: false},
		{name: "private 192.168", ips: []string{"192.168.1.1"}, want: false},
		{name: "link local metadata", ips: []string{"169.254.169.254"}, want: false},
		{name: "carrier-grade NAT low", ips: []string{"100.64.0.1"}, want: false},
		{name: "carrier-grade NAT high", ips: []string{"100.127.255.255"}, want: false},
		{name: "benchmarking", ips: []string{"198.18.0.1", "198.19.255.255"}, want: false},
		{name: "documentation v4", ips: []string{"192.0.2.1", "198.51.100.1", "203.0.113.1"}, want: false},
		{name: "documentation v6", ips: []string{"2001:db8::1"}, want: false},
		{name: "unspecified v4", ips: []string{"0.0.0.0"}, want: false},
		{name: "unspecified v6", ips: []string{"::"}, want: false},
		{name: "multicast", ips: []string{"224.0.0.1"}, want: false},
		{name: "mixed with private", ips: []string{"8.8.8.8", "10.0.0.1"}, want: false},
		{name: "public v4", ips: []string{"8.8.8.8", "1.1.1.1"}, want: true},
		{name: "public v6", ips: []string{"2606:4700:4700::1111"}, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var ips []net.IP

			for _, s := range test.ips {
				ips = append(ips, net.ParseIP(s))
			}

			if got := publicIPs(ips); got != test.want {
				t.Errorf("publicIPs(%v) = %v, want %v", test.ips, got, test.want)
			}
		})
	}
}

// Dialing a loopback address is refused before any connection is attempted,
// so this exercises the guard with no network.
func TestDialPublicIPRefusesLoopback(t *testing.T) {
	_, err := DialPublicIP(context.Background(), "tcp", "127.0.0.1:1")
	if err == nil || !strings.Contains(err.Error(), "non-public") {
		t.Errorf("err = %v, want a non-public refusal", err)
	}
}

func TestDialPublicIPRejectsBadAddress(t *testing.T) {
	_, err := DialPublicIP(context.Background(), "tcp", "not-an-address")
	if err == nil {
		t.Error("err is nil, want a host/port error")
	}
}
