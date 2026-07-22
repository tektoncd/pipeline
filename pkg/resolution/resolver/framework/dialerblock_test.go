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
	"errors"
	"testing"
)

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
		{name: "ipv4 unspecified 0.0.0.0", network: "tcp", address: "0.0.0.0:80", blocked: true},
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

func TestRestrictedHTTPTransportInstalled(t *testing.T) {
	tr := RestrictedHTTPTransport()
	if tr == nil {
		t.Fatal("nil transport")
	}
	if tr.DialContext == nil {
		t.Fatal("expected DialContext to be installed")
	}
}
