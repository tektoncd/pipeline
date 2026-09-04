/*
Copyright 2023 The Tekton Authors

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

package tracing

import (
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"k8s.io/client-go/util/cert"
)

func TestCertPoolFromCACertInlinePEM(t *testing.T) {
	pemBytes, _, err := cert.GenerateSelfSignedCertKey("test", nil, nil)
	if err != nil {
		t.Fatalf("failed to generate test cert: %v", err)
	}

	pool, err := certPoolFromCACert(string(pemBytes))
	if err != nil {
		t.Fatalf("certPoolFromCACert() error = %v", err)
	}
	if pool == nil {
		t.Fatal("expected non-nil cert pool")
	}
}

func TestCertPoolFromCACertFilePath(t *testing.T) {
	pemBytes, _, err := cert.GenerateSelfSignedCertKey("test", nil, nil)
	if err != nil {
		t.Fatalf("failed to generate test cert: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "ca.crt")
	if err := os.WriteFile(path, pemBytes, 0o644); err != nil {
		t.Fatalf("failed to write cert file: %v", err)
	}

	pool, err := certPoolFromCACert(path)
	if err != nil {
		t.Fatalf("certPoolFromCACert() error = %v", err)
	}
	if pool == nil {
		t.Fatal("expected non-nil cert pool")
	}
}

func TestCertPoolFromCACertInvalid(t *testing.T) {
	_, err := certPoolFromCACert("not-a-valid-cert")
	if err == nil {
		t.Fatal("expected error for invalid cert, got nil")
	}
}

func TestCreateTracerProviderTLSExport(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/traces" {
			t.Errorf("expected path /v1/traces, got %s", r.URL.Path)
		}
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	endpoint := server.URL + "/v1/traces"

	t.Run("succeeds with CA", func(t *testing.T) {
		requests.Store(0)

		tp, err := createTracerProvider("test-service", &config.Tracing{
			Enabled:  true,
			Endpoint: endpoint,
			CACert:   string(certPEM),
		}, "", "")
		if err != nil {
			t.Fatalf("createTracerProvider() error = %v", err)
		}
		t.Cleanup(func() {
			if err := tp.(*tracesdk.TracerProvider).Shutdown(context.Background()); err != nil {
				t.Errorf("Shutdown() error = %v", err)
			}
		})

		ctx := context.Background()
		_, span := tp.Tracer("test").Start(ctx, "test-span")
		span.End()

		if err := tp.(*tracesdk.TracerProvider).ForceFlush(ctx); err != nil {
			t.Fatalf("ForceFlush() error = %v", err)
		}
		if got := requests.Load(); got != 1 {
			t.Fatalf("expected 1 export request, got %d", got)
		}
	})

	t.Run("fails without CA", func(t *testing.T) {
		requests.Store(0)

		tp, err := createTracerProvider("test-service", &config.Tracing{
			Enabled:  true,
			Endpoint: endpoint,
		}, "", "")
		if err != nil {
			t.Fatalf("createTracerProvider() error = %v", err)
		}
		t.Cleanup(func() {
			if err := tp.(*tracesdk.TracerProvider).Shutdown(context.Background()); err != nil {
				t.Errorf("Shutdown() error = %v", err)
			}
		})

		ctx := context.Background()
		_, span := tp.Tracer("test").Start(ctx, "test-span")
		span.End()

		_ = tp.(*tracesdk.TracerProvider).ForceFlush(ctx)
		if got := requests.Load(); got != 0 {
			t.Fatalf("expected export to fail without CA cert, got %d requests", got)
		}
	})
}

func TestCreateTracerProviderWithInvalidCACert(t *testing.T) {
	cfg := &config.Tracing{
		Enabled:  true,
		Endpoint: "https://collector.example.svc:4318/v1/traces",
		CACert:   "not-a-valid-cert",
	}

	_, err := createTracerProvider("test-service", cfg, "", "")
	if err == nil {
		t.Fatal("expected error for invalid CA cert, got nil")
	}
}
