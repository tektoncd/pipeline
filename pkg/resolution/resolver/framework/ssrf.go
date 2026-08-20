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
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"syscall"
	"time"
)

const BlockPrivateIPsConfigKey = "block-private-ips"

var cgnatPrefix = netip.MustParsePrefix("100.64.0.0/10")

// restrictedTransport is a package-level singleton so all callers share
// one connection pool instead of leaking a new pool per request.
var restrictedTransport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   BlockPrivateNetworkControl,
	}).DialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          100,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

type RestrictedAddrError struct {
	Addr   string
	Reason string
}

func (e *RestrictedAddrError) Error() string {
	return fmt.Sprintf("address %s is blocked: %s", e.Addr, e.Reason)
}

func isBlockedIP(addr netip.Addr) (bool, string) {
	addr = addr.Unmap()
	if !addr.IsValid() {
		return true, "invalid IP address"
	}
	if addr.IsUnspecified() {
		return true, "unspecified address"
	}
	if addr.IsLoopback() {
		return true, "loopback address"
	}
	if addr.IsPrivate() {
		return true, "RFC1918/RFC4193 private address"
	}
	if addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() {
		return true, "link-local address"
	}
	if addr.IsMulticast() {
		return true, "multicast address"
	}
	if cgnatPrefix.Contains(addr) {
		return true, "RFC6598 CGNAT address"
	}
	return false, ""
}

// BlockPrivateNetworkControl is a net.Dialer.Control callback that rejects
// connections to private, loopback, link-local, multicast, CGNAT, and
// unspecified addresses. It runs after DNS resolution and before connect(2).
func BlockPrivateNetworkControl(network, address string, _ syscall.RawConn) error {
	switch network {
	case "tcp", "tcp4", "tcp6", "udp", "udp4", "udp6": //nolint:goconst
	default:
		return &RestrictedAddrError{Addr: address, Reason: "unsupported network " + network}
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return &RestrictedAddrError{Addr: address, Reason: err.Error()}
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return &RestrictedAddrError{Addr: address, Reason: "address is not a literal IP after resolution"}
	}
	if blocked, reason := isBlockedIP(addr); blocked {
		return &RestrictedAddrError{Addr: address, Reason: reason}
	}
	return nil
}

// RestrictedHTTPTransport returns the shared *http.Transport whose dialer
// rejects connections to private network address classes. The transport is
// a package-level singleton — all callers share one connection pool.
func RestrictedHTTPTransport() *http.Transport {
	return restrictedTransport
}

// BlockPrivateIPs reports whether the resolver should refuse to dial
// private network addresses. Reads BlockPrivateIPsConfigKey from the
// resolver-scoped ConfigMap attached to ctx.
func BlockPrivateIPs(ctx context.Context) bool {
	conf := GetResolverConfigFromContext(ctx)
	if conf == nil {
		return true
	}
	v, ok := conf[BlockPrivateIPsConfigKey]
	if !ok {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "false", "0", "no", "off": //nolint:goconst
		return false
	default:
		return true
	}
}

// ResolverHTTPTransport returns the transport a resolver should use for
// outbound HTTP requests. When BlockPrivateIPs(ctx) is true, connections
// to private/loopback/link-local/CGNAT addresses are refused.
func ResolverHTTPTransport(ctx context.Context) http.RoundTripper {
	if BlockPrivateIPs(ctx) {
		return RestrictedHTTPTransport()
	}
	return http.DefaultTransport
}
