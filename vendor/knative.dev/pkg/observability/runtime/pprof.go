/*
Copyright 2025 The Knative Authors

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

package runtime

import (
	"net/http"
	"net/http/pprof"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

const (
	// ProfilingPortEnvKey specified the name of an environment variable that
	// may be used to override the default profiling port.
	ProfilingPortEnvKey = "PROFILING_PORT"

	// ProfilingDefaultPort specifies the default port where profiling data is available when profiling is enabled
	ProfilingDefaultPort = 8008
)

type ProfilingServer struct {
	*http.Server
	*ProfilingHandler
}

type ProfilingHandler struct {
	enabled *atomic.Bool
	handler http.Handler
}

// NewHandler create a new ProfilingHandler which serves runtime profiling data
// according to the given context path
func NewProfilingHandler() *ProfilingHandler {
	const pprofPrefix = "GET /debug/pprof/"

	mux := http.NewServeMux()
	mux.HandleFunc(pprofPrefix, pprof.Index)
	mux.HandleFunc(pprofPrefix+"cmdline", pprof.Cmdline)
	mux.HandleFunc(pprofPrefix+"profile", pprof.Profile)
	mux.HandleFunc(pprofPrefix+"symbol", pprof.Symbol)
	mux.HandleFunc(pprofPrefix+"trace", pprof.Trace)

	enabled := &atomic.Bool{}
	enabled.Store(false)

	return &ProfilingHandler{
		handler: mux,
		enabled: enabled,
	}
}

func (h *ProfilingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.enabled.Load() {
		h.handler.ServeHTTP(w, r)
	} else {
		http.NotFound(w, r)
	}
}

func (h *ProfilingHandler) SetEnabled(enabled bool) (old bool) {
	return h.enabled.Swap(enabled)
}

// NewServer creates a new http server that exposes profiling data on the default profiling port
func NewProfilingServer() *ProfilingServer {
	port := os.Getenv(ProfilingPortEnvKey)
	if port == "" {
		port = strconv.Itoa(ProfilingDefaultPort)
	}

	h := NewProfilingHandler()
	return &ProfilingServer{
		ProfilingHandler: h,
		Server: &http.Server{
			Addr:    ":" + port,
			Handler: h,
			// https://medium.com/a-journey-with-go/go-understand-and-mitigate-slowloris-attack-711c1b1403f6
			ReadHeaderTimeout: 15 * time.Second,
		},
	}
}
