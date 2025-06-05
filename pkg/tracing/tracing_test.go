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
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/test"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	fakesecretinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/secret/fake"
	_ "knative.dev/pkg/system/testing"
)

func TestNewTracerProvider(t *testing.T) {
	tp := New("test-service", zap.NewNop().Sugar())

	tracer := tp.Tracer("tracer")
	_, span := tracer.Start(t.Context(), "example")

	// tp.Tracer should return a nooptracer initially
	// recording is always false for spans created by nooptracer
	if span.IsRecording() {
		t.Fatalf("Span is recording before configuration")
	}
}

func TestOnStore(t *testing.T) {
	tp := New("test-service", zap.NewNop().Sugar())

	cfg := &config.Tracing{
		Enabled: false,
	}

	tp.OnStore(nil)("config-tracing", cfg)

	tracer := tp.Tracer("tracer")
	_, span := tracer.Start(t.Context(), "example")

	// tp.Tracer should return a nooptracer when tracing is disabled
	// recording is always false for spans created by nooptracer
	if span.IsRecording() {
		t.Fatalf("Span is recording with tracing disabled")
	}
}

func TestOnStoreWithSecret(t *testing.T) {
	ctx, _ := ttesting.SetupFakeContext(t)

	tp := New("test-service", zap.NewNop().Sugar())

	cfg := &config.Tracing{
		Enabled:           true,
		CredentialsSecret: "tracing-sec",
	}

	client := fakekubeclient.Get(ctx)
	informer := fakesecretinformer.Get(ctx)

	client.PrependReactor("*", "secrets", test.AddToInformer(t, informer.Informer().GetIndexer()))

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "tracing-sec",
			// system.Namespace() will return `knative-testing`
			// Set inside the imported knative testing package
			Namespace: "knative-testing",
		},
		Data: map[string][]byte{
			"username": []byte("user"),
			"password": []byte("pass"),
		},
	}
	if _, err := client.CoreV1().Secrets("knative-testing").Create(ctx, secret, metav1.CreateOptions{}); err != nil {
		t.Errorf("Unable to create secret for tracing,err : %v", err.Error())
	}

	tp.OnStore(informer.Lister())("config-tracing", cfg)

	if tp.username != "user" || tp.password != "pass" {
		t.Errorf("Tracing provider is not initialized with correct credentials")
	}
}

func TestOnStoreWithEnabled(t *testing.T) {
	tp := New("test-service", zap.NewNop().Sugar())

	cfg := &config.Tracing{
		Enabled:  true,
		Endpoint: "test-endpoint",
	}

	tp.OnStore(nil)("config-tracing", cfg)

	tracer := tp.Tracer("tracer")
	_, span := tracer.Start(t.Context(), "example")

	if !span.IsRecording() {
		t.Fatalf("Span is not recording with tracing enabled")
	}
}

func TestOnSecretWithSecretName(t *testing.T) {
	tp := New("test-service", zap.NewNop().Sugar())

	cfg := &config.Tracing{
		Enabled:           true,
		CredentialsSecret: "jaeger",
	}

	tp.OnStore(nil)("config-tracing", cfg)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "jaeger",
		},
		Data: map[string][]byte{
			"username": []byte("user"),
			"password": []byte("pass"),
		},
	}

	tp.OnSecret(secret)

	if tp.username != "user" || tp.password != "pass" {
		t.Errorf("Tracing provider is not updated with correct credentials")
	}
}

// If OnSecret was called without changing the credentials, do not initialize again
func TestOnSecretWithSameCreds(t *testing.T) {
	tp := New("test-service", zap.NewNop().Sugar())

	cfg := &config.Tracing{
		Enabled:           true,
		CredentialsSecret: "jaeger",
	}

	tp.OnStore(nil)("config-tracing", cfg)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "jaeger",
		},
		Data: map[string][]byte{
			"username": []byte("user"),
			"password": []byte("pass"),
		},
	}

	tp.OnSecret(secret)

	p := tp.provider

	tp.OnSecret(secret)

	if p != tp.provider {
		t.Errorf("Tracerprovider was reinitialized when the credentials were not changed")
	}
}

func TestOnSecretWithWrongName(t *testing.T) {
	tp := New("test-service", zap.NewNop().Sugar())

	cfg := &config.Tracing{
		Enabled:           true,
		CredentialsSecret: "jaeger",
	}

	tp.OnStore(nil)("config-tracing", cfg)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "somethingelse",
		},
		Data: map[string][]byte{
			"username": []byte("user"),
			"password": []byte("pass"),
		},
	}

	tp.OnSecret(secret)

	if tp.username == "user" || tp.password == "pass" {
		t.Errorf("Tracing provider is updated with incorrect credentials")
	}
}

func TestHandlerSecretUpdate(t *testing.T) {
	tp := New("test-service", zap.NewNop().Sugar())

	cfg := &config.Tracing{
		Enabled:           true,
		CredentialsSecret: "jaeger",
	}

	tp.OnStore(nil)("config-tracing", cfg)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "somethingelse",
		},
		Data: map[string][]byte{
			"username": []byte("user"),
			"password": []byte("pass"),
		},
	}

	tp.Handler(secret)

	if tp.username == "user" || tp.password == "pass" {
		t.Errorf("Tracing provider is updated with incorrect credentials")
	}

	secret.Data["password"] = []byte("pass1")

	tp.Handler(secret)

	if tp.username == "user" || tp.password == "pass1" {
		t.Errorf("Tracing provider is not updated when secret is updated")
	}
}
