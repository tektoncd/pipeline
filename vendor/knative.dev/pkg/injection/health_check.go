/*
Copyright 2023 The Knative Authors

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

package injection

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"knative.dev/pkg/logging"
)

// HealthCheckDefaultPort defines the default port number for health probes
const HealthCheckDefaultPort = 8080

// ServeHealthProbes sets up liveness and readiness probes.
// If user sets no probes explicitly via the context then defaults are added.
func ServeHealthProbes(ctx context.Context, port int) error {
	logger := logging.FromContext(ctx)
	server := http.Server{ReadHeaderTimeout: time.Minute, Handler: muxWithHandles(ctx), Addr: ":" + strconv.Itoa(port)}
	go func() {
		<-ctx.Done()
		_ = server.Shutdown(ctx)
	}()

	// start the web server on port and accept requests
	logger.Infof("Probes server listening on port %d", port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func muxWithHandles(ctx context.Context) *http.ServeMux {
	mux := http.NewServeMux()
	readiness := getReadinessHandleOrDefault(ctx)
	liveness := getLivenessHandleOrDefault(ctx)
	mux.HandleFunc("/readiness", *readiness)
	mux.HandleFunc("/health", *liveness)
	return mux
}

func newDefaultProbesHandle(sigCtx context.Context) http.HandlerFunc {
	logger := logging.FromContext(sigCtx)
	return func(w http.ResponseWriter, r *http.Request) {
		f := func() error {
			select {
			// When we get SIGTERM (sigCtx done), let readiness probes start failing.
			case <-sigCtx.Done():
				logger.Info("Signal context canceled")
				return errors.New("received SIGTERM from kubelet")
			default:
				return nil
			}
		}
		if err := f(); err != nil {
			logger.Errorf("Healthcheck failed: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}
}

type addReadinessKey struct{}

// AddReadiness signals to probe setup logic to add a user provided probe handler
func AddReadiness(ctx context.Context, handlerFunc http.HandlerFunc) context.Context {
	return context.WithValue(ctx, addReadinessKey{}, &handlerFunc)
}

func getReadinessHandleOrDefault(ctx context.Context) *http.HandlerFunc {
	if ctx.Value(addReadinessKey{}) != nil {
		return ctx.Value(addReadinessKey{}).(*http.HandlerFunc)
	}
	defaultHandle := newDefaultProbesHandle(ctx)
	return &defaultHandle
}

type addLivenessKey struct{}

// AddLiveness signals to probe setup logic to add a user provided probe handler
func AddLiveness(ctx context.Context, handlerFunc http.HandlerFunc) context.Context {
	return context.WithValue(ctx, addLivenessKey{}, &handlerFunc)
}

func getLivenessHandleOrDefault(ctx context.Context) *http.HandlerFunc {
	if ctx.Value(addLivenessKey{}) != nil {
		return ctx.Value(addLivenessKey{}).(*http.HandlerFunc)
	}
	defaultHandle := newDefaultProbesHandle(ctx)
	return &defaultHandle
}
