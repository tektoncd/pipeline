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
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

// RestrictedAddrError is returned by the dial control callback when a
// connection target resolves to a disallowed address class (loopback,
// private, link-local, multicast, or unspecified).
type RestrictedAddrError struct {
	Address string
	Reason  string
}

func (e *RestrictedAddrError) Error() string {
	return fmt.Sprintf("restricted dial: refusing to dial %s: %s", e.Address, e.Reason)
}

// isDisallowedIP reports whether ip belongs to an address class that the
// resolver-side dial guard refuses to dial. The categories follow the
// "block-private-ips" goal tracked in issue #9602.
func isDisallowedIP(ip net.IP) (bool, string) {
	if ip == nil {
		return true, "unparseable IP"
	}
	if ip.IsUnspecified() {
		return true, "unspecified address"
	}
	if ip.IsLoopback() {
		return true, "loopback address"
	}
	if ip.IsPrivate() {
		return true, "RFC1918/RFC4193 private address"
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true, "link-local address"
	}
	if ip.IsMulticast() {
		return true, "multicast address"
	}
	if ip.IsInterfaceLocalMulticast() {
		return true, "interface-local multicast address"
	}
	// CGNAT / RFC6598 100.64.0.0/10 — not flagged by IsPrivate but should
	// not be a resolver target.
	if v4 := ip.To4(); v4 != nil {
		if v4[0] == 100 && v4[1]&0xc0 == 64 {
			return true, "RFC6598 CGNAT address"
		}
	}
	return false, ""
}

// verifyAddrAllowed verifies that the given network/address pair (as passed
// to net.Dialer.Control) resolves only to addresses that are safe to dial
// from a resolver running inside the Tekton controller pod.
func verifyAddrAllowed(network, address string) error {
	switch network {
	case "tcp", "tcp4", "tcp6", "udp", "udp4", "udp6":
	default:
		return &RestrictedAddrError{Address: address, Reason: "unsupported network " + network}
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return &RestrictedAddrError{Address: address, Reason: err.Error()}
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return &RestrictedAddrError{Address: address, Reason: "address is not a literal IP after resolution"}
	}
	if bad, reason := isDisallowedIP(ip); bad {
		return &RestrictedAddrError{Address: address, Reason: reason}
	}
	return nil
}

// BlockPrivateNetworkControl is intended for use as net.Dialer.Control. It
// is invoked after DNS resolution and immediately before connect(2), so
// checking the resolved address here closes the time-of-check / time-of-use
// gap that a pre-dial DNS lookup would leave open.
func BlockPrivateNetworkControl(network, address string, _ syscall.RawConn) error {
	return verifyAddrAllowed(network, address)
}

// RestrictedHTTPTransport returns an *http.Transport whose dialer rejects
// connections to disallowed address classes. The transport otherwise mirrors
// http.DefaultTransport.
func RestrictedHTTPTransport() *http.Transport {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   BlockPrivateNetworkControl,
	}
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.DialContext = dialer.DialContext
	return t
}
