/*
Copyright 2026 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package framework

import (
	"context"
	"errors"
	"net/http"
	"net/netip"
	"testing"
)

func TestIsBlockedIP(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		blocked bool
	}{
		// Loopback
		{name: "loopback IPv4", ip: "127.0.0.1", blocked: true},
		{name: "loopback IPv4 other", ip: "127.0.0.2", blocked: true},
		{name: "loopback IPv6", ip: "::1", blocked: true},

		// Private RFC 1918
		{name: "private 10.x", ip: "10.0.0.1", blocked: true},
		{name: "private 172.16.x", ip: "172.16.0.1", blocked: true},
		{name: "private 192.168.x", ip: "192.168.1.1", blocked: true},

		// Private IPv6
		{name: "private IPv6 fc00", ip: "fc00::1", blocked: true},
		{name: "private IPv6 fd00", ip: "fd00::1", blocked: true},

		// Link-local (includes cloud metadata)
		{name: "cloud metadata", ip: "169.254.169.254", blocked: true},
		{name: "link-local IPv4", ip: "169.254.1.1", blocked: true},
		{name: "link-local IPv6", ip: "fe80::1", blocked: true},

		// Unspecified
		{name: "unspecified IPv4", ip: "0.0.0.0", blocked: true},
		{name: "unspecified IPv6", ip: "::", blocked: true},

		// Multicast
		{name: "multicast IPv4", ip: "224.0.0.1", blocked: true},
		{name: "multicast IPv6", ip: "ff02::1", blocked: true},

		// Carrier-grade NAT (RFC 6598)
		{name: "CGNAT start", ip: "100.64.0.1", blocked: true},
		{name: "CGNAT end", ip: "100.127.255.254", blocked: true},

		// IPv4-mapped IPv6 bypass attempts
		{name: "mapped loopback", ip: "::ffff:127.0.0.1", blocked: true},
		{name: "mapped metadata", ip: "::ffff:169.254.169.254", blocked: true},
		{name: "mapped private", ip: "::ffff:10.0.0.1", blocked: true},

		// Allowed public IPs
		{name: "public DNS 8.8.8.8", ip: "8.8.8.8", blocked: false},
		{name: "public DNS 1.1.1.1", ip: "1.1.1.1", blocked: false},
		{name: "public IP", ip: "142.250.80.46", blocked: false},
		{name: "public IPv6", ip: "2607:f8b0:4004:800::200e", blocked: false},
		{name: "100.63 not cgnat", ip: "100.63.0.1", blocked: false},
		{name: "100.128 not cgnat", ip: "100.128.0.1", blocked: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			addr := netip.MustParseAddr(tc.ip)
			blocked, _ := isBlockedIP(addr)
			if blocked != tc.blocked {
				t.Errorf("isBlockedIP(%s) = %v, want %v", tc.ip, blocked, tc.blocked)
			}
		})
	}
}

func TestBlockPrivateNetworkControl(t *testing.T) {
	cases := []struct {
		name    string
		network string
		address string
		blocked bool
	}{
		{name: "ipv4 loopback", network: "tcp", address: "127.0.0.1:80", blocked: true},
		{name: "ipv4 loopback range", network: "tcp", address: "127.5.4.3:443", blocked: true},
		{name: "ipv4 rfc1918 10.x", network: "tcp", address: "10.0.0.1:80", blocked: true},
		{name: "ipv4 rfc1918 172.16", network: "tcp", address: "172.16.0.5:80", blocked: true},
		{name: "ipv4 rfc1918 192.168", network: "tcp", address: "192.168.1.1:80", blocked: true},
		{name: "ipv4 link-local 169.254", network: "tcp", address: "169.254.169.254:80", blocked: true},
		{name: "ipv4 unspecified", network: "tcp", address: "0.0.0.0:80", blocked: true},
		{name: "ipv4 multicast", network: "tcp", address: "224.0.0.1:80", blocked: true},
		{name: "ipv4 cgnat 100.64", network: "tcp", address: "100.64.0.1:80", blocked: true},
		{name: "ipv4 cgnat 100.127", network: "tcp", address: "100.127.255.254:80", blocked: true},
		{name: "ipv6 loopback", network: "tcp", address: "[::1]:80", blocked: true},
		{name: "ipv6 ula fd00", network: "tcp", address: "[fd00::1]:80", blocked: true},
		{name: "ipv6 link-local fe80", network: "tcp", address: "[fe80::1]:80", blocked: true},
		{name: "ipv6 unspecified", network: "tcp", address: "[::]:80", blocked: true},
		{name: "non-IP host", network: "tcp", address: "example.com:80", blocked: true},
		{name: "bad network", network: "unix", address: "/var/run/sock:0", blocked: true},
		{name: "missing port", network: "tcp", address: "1.1.1.1", blocked: true},

		{name: "public ipv4", network: "tcp", address: "1.1.1.1:443", blocked: false},
		{name: "public ipv4 second", network: "tcp", address: "8.8.8.8:443", blocked: false},
		{name: "public ipv6", network: "tcp", address: "[2606:4700:4700::1111]:443", blocked: false},
		{name: "udp public", network: "udp", address: "8.8.4.4:53", blocked: false},
		{name: "100.63 not cgnat", network: "tcp", address: "100.63.0.1:80", blocked: false},
		{name: "100.128 not cgnat", network: "tcp", address: "100.128.0.1:80", blocked: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := BlockPrivateNetworkControl(tc.network, tc.address, nil)
			gotBlocked := err != nil
			if gotBlocked != tc.blocked {
				t.Fatalf("blocked = %v want %v (err=%v)", gotBlocked, tc.blocked, err)
			}
			if err != nil {
				var be *RestrictedAddrError
				if !errors.As(err, &be) {
					t.Fatalf("expected *RestrictedAddrError, got %T: %v", err, err)
				}
			}
		})
	}
}

func TestBlockPrivateIPs(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]string
		want   bool
	}{
		{name: "nil config", config: nil, want: true},
		{name: "key absent", config: map[string]string{}, want: true},
		{name: "true", config: map[string]string{BlockPrivateIPsConfigKey: "true"}, want: true},
		{name: "True", config: map[string]string{BlockPrivateIPsConfigKey: "True"}, want: true},
		{name: "yes", config: map[string]string{BlockPrivateIPsConfigKey: "yes"}, want: true},
		{name: "typo", config: map[string]string{BlockPrivateIPsConfigKey: "fals"}, want: true},
		{name: "false", config: map[string]string{BlockPrivateIPsConfigKey: "false"}, want: false},
		{name: "False", config: map[string]string{BlockPrivateIPsConfigKey: "False"}, want: false},
		{name: "FALSE", config: map[string]string{BlockPrivateIPsConfigKey: "FALSE"}, want: false},
		{name: "0", config: map[string]string{BlockPrivateIPsConfigKey: "0"}, want: false},
		{name: "no", config: map[string]string{BlockPrivateIPsConfigKey: "no"}, want: false},
		{name: "No", config: map[string]string{BlockPrivateIPsConfigKey: "No"}, want: false},
		{name: "off", config: map[string]string{BlockPrivateIPsConfigKey: "off"}, want: false},
		{name: "Off", config: map[string]string{BlockPrivateIPsConfigKey: "Off"}, want: false},
		{name: "false with spaces", config: map[string]string{BlockPrivateIPsConfigKey: " false "}, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.config != nil {
				ctx = InjectResolverConfigToContext(ctx, tc.config)
			}
			got := BlockPrivateIPs(ctx)
			if got != tc.want {
				t.Errorf("BlockPrivateIPs() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestResolverHTTPTransport(t *testing.T) {
	t.Run("blocking enabled returns restricted transport", func(t *testing.T) {
		ctx := InjectResolverConfigToContext(context.Background(), map[string]string{})
		transport := ResolverHTTPTransport(ctx)
		if transport == http.DefaultTransport {
			t.Fatal("expected restricted transport, got http.DefaultTransport")
		}
		if _, ok := transport.(*http.Transport); !ok {
			t.Fatalf("expected *http.Transport, got %T", transport)
		}
	})

	t.Run("blocking disabled returns default transport", func(t *testing.T) {
		ctx := InjectResolverConfigToContext(context.Background(), map[string]string{
			BlockPrivateIPsConfigKey: "false",
		})
		transport := ResolverHTTPTransport(ctx)
		if transport != http.DefaultTransport {
			t.Fatal("expected http.DefaultTransport when blocking is disabled")
		}
	})
}

func TestRestrictedHTTPTransport(t *testing.T) {
	tr := RestrictedHTTPTransport()
	if tr == nil {
		t.Fatal("nil transport")
	}
	if tr.DialContext == nil {
		t.Fatal("expected DialContext to be installed")
	}
}
