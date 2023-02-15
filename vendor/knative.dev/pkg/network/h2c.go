/*
Copyright 2019 The Knative Authors

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

package network

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// NewServer returns a new HTTP Server with HTTP2 handler.
func NewServer(addr string, h http.Handler) *http.Server {
	//nolint:gosec
	h1s := &http.Server{
		Addr:    addr,
		Handler: h2c.NewHandler(h, &http2.Server{}),
	}

	return h1s
}

// NewH2CTransport constructs a new H2C transport.
// That transport will reroute all HTTPS traffic to HTTP. This is
// to explicitly allow h2c (http2 without TLS) transport.
// See https://github.com/golang/go/issues/14141 for more details.
func NewH2CTransport() http.RoundTripper {
	return newH2CTransport(false)
}

func newH2CTransport(disableCompression bool) http.RoundTripper {
	return &http2.Transport{
		AllowHTTP:          true,
		DisableCompression: disableCompression,
		DialTLS: func(netw, addr string, _ *tls.Config) (net.Conn, error) {
			return DialWithBackOff(context.Background(),
				netw, addr)
		},
	}
}

// newH2Transport constructs a neew H2 transport. That transport will handles HTTPS traffic
// with TLS config.
func newH2Transport(disableCompression bool, tlsConf *tls.Config) http.RoundTripper {
	return &http2.Transport{
		DisableCompression: disableCompression,
		DialTLS: func(netw, addr string, tlsConf *tls.Config) (net.Conn, error) {
			return DialTLSWithBackOff(context.Background(),
				netw, addr, tlsConf)
		},
		TLSClientConfig: tlsConf,
	}
}
