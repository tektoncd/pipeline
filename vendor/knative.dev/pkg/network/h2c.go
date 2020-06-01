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
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// NewServer returns a new HTTP Server with HTTP2 handler.
func NewServer(addr string, h http.Handler) *http.Server {
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
	return &http2.Transport{
		AllowHTTP: true,
		DialTLS: func(netw, addr string, cfg *tls.Config) (net.Conn, error) {
			d := &net.Dialer{
				Timeout:   DefaultConnTimeout,
				KeepAlive: 5 * time.Second,
				DualStack: true,
			}
			return d.Dial(netw, addr)
		},
	}
}
