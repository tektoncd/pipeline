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
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	fakesecretinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/secret/fake"
	"knative.dev/pkg/system"
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
			Name:      "tracing-sec",
			Namespace: system.Namespace(),
		},
		Data: map[string][]byte{
			"username": []byte("user"),
			"password": []byte("pass"),
		},
	}
	if _, err := client.CoreV1().Secrets(system.Namespace()).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
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
			Name:      "jaeger",
			Namespace: system.Namespace(),
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
			Name:      "jaeger",
			Namespace: system.Namespace(),
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

func TestOnStoreMissingSecretThenCreateSecret(t *testing.T) {
	ctx, _ := ttesting.SetupFakeContext(t)

	tp := New("test-service", zap.NewNop().Sugar())

	client := fakekubeclient.Get(ctx)
	informer := fakesecretinformer.Get(ctx)

	client.PrependReactor("*", "secrets", test.AddToInformer(t, informer.Informer().GetIndexer()))

	// Step 1: Set up working tracing with endpoint A (no credentials needed).
	cfgA := &config.Tracing{
		Enabled:  true,
		Endpoint: "http://collector-a:4318/v1/traces",
	}
	tp.OnStore(informer.Lister())("config-tracing", cfgA)

	if _, ok := tp.provider.(*tracesdk.TracerProvider); !ok {
		t.Fatalf("expected a real TracerProvider after valid config, got %T", tp.provider)
	}

	// Step 2: Update config to endpoint B with a Secret that does not exist yet.
	// The Secret lookup should fail, the old exporter must be shut down (noop),
	// and t.cfg must keep the new config so OnSecret can recover.
	cfgB := &config.Tracing{
		Enabled:           true,
		Endpoint:          "http://collector-b:4318/v1/traces",
		CredentialsSecret: "tracing-sec",
	}
	tp.OnStore(informer.Lister())("config-tracing", cfgB)

	// Provider must be noop — not the old exporter pointing at A.
	if _, ok := tp.provider.(*tracesdk.TracerProvider); ok {
		t.Fatalf("expected noop provider after missing Secret, got real TracerProvider (stale export)")
	}

	// t.cfg must still reference the new config so OnSecret can match the Secret name.
	if tp.cfg == nil || tp.cfg.CredentialsSecret != "tracing-sec" {
		t.Fatalf("expected t.cfg to keep the new config for OnSecret recovery, got %+v", tp.cfg)
	}

	// Step 3: Create the Secret. OnSecret should recover and reinitialize the provider.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tracing-sec",
			Namespace: system.Namespace(),
		},
		Data: map[string][]byte{
			"username": []byte("user"),
			"password": []byte("pass"),
		},
	}
	if _, err := client.CoreV1().Secrets(system.Namespace()).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
		t.Fatalf("unable to create secret: %v", err)
	}

	tp.OnSecret(secret)

	if tp.username != "user" || tp.password != "pass" {
		t.Errorf("expected credentials user/pass after OnSecret, got %q/%q", tp.username, tp.password)
	}

	if _, ok := tp.provider.(*tracesdk.TracerProvider); !ok {
		t.Fatalf("expected real TracerProvider after OnSecret recovery, got %T", tp.provider)
	}
}

func TestOnSecretIgnoresCrossNamespace(t *testing.T) {
	tp := New("test-service", zap.NewNop().Sugar())

	cfg := &config.Tracing{
		Enabled:           true,
		CredentialsSecret: "tracing-sec",
	}

	tp.OnStore(nil)("config-tracing", cfg)

	// A Secret with the correct name but from a different namespace
	// must not overwrite the tracing credentials.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tracing-sec",
			Namespace: "workload-namespace",
		},
		Data: map[string][]byte{
			"username": []byte("attacker"),
			"password": []byte("evil"),
		},
	}

	tp.OnSecret(secret)

	if tp.username == "attacker" || tp.password == "evil" {
		t.Errorf("OnSecret accepted a Secret from a wrong namespace")
	}
}

func TestOnSecretIgnoresEmptyCredentials(t *testing.T) {
	tp := New("test-service", zap.NewNop().Sugar())

	cfg := &config.Tracing{
		Enabled:           true,
		CredentialsSecret: "tracing-sec",
	}

	tp.OnStore(nil)("config-tracing", cfg)

	// A Secret that exists but has no username/password data
	// should not trigger reinitialization.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tracing-sec",
			Namespace: system.Namespace(),
		},
		Data: map[string][]byte{},
	}

	tp.OnSecret(secret)

	if tp.username != "" || tp.password != "" {
		t.Errorf("OnSecret should not update credentials from an empty Secret")
	}
}

func TestOnSecretIgnoresPartialCredentials(t *testing.T) {
	tp := New("test-service", zap.NewNop().Sugar())

	cfg := &config.Tracing{
		Enabled:           true,
		CredentialsSecret: "tracing-sec",
	}

	tp.OnStore(nil)("config-tracing", cfg)

	// A Secret with only username but no password should be rejected.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tracing-sec",
			Namespace: system.Namespace(),
		},
		Data: map[string][]byte{
			"username": []byte("user"),
		},
	}

	tp.OnSecret(secret)

	if tp.username == "user" {
		t.Errorf("OnSecret should not accept a Secret with only username")
	}
}
func TestOnSecretValidToPartialToValid(t *testing.T) {
	tp := New("test-service", zap.NewNop().Sugar())

	cfg := &config.Tracing{
		Enabled:           true,
		CredentialsSecret: "valid-sec",
		Endpoint:          "http://collector-a:4318/v1/traces",
	}

	tp.OnStore(nil)("config-tracing", cfg)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "valid-sec",
			Namespace: system.Namespace(),
		},
		Data: map[string][]byte{
			"username": []byte("user"),
			"password": []byte("pass"),
		},
	}

	tp.OnSecret(secret)
	if _, ok := tp.provider.(*tracesdk.TracerProvider); !ok {
		t.Fatalf("expected real TracerProvider after valid Secret")
	}
	if tp.username != "user" || tp.password != "pass" {
		t.Errorf("Tracing provider is updated with incorrect credentials")
	}

	secret.Data["password"] = []byte("")
	tp.OnSecret(secret)
	if _, ok := tp.provider.(*tracesdk.TracerProvider); ok {
		t.Fatalf("expected noop provider after partial Secret (empty password)")
	}
	if tp.username != "" || tp.password != "" {
		t.Errorf("Tracing provider is not updated with incorrect credentials")
	}

	secret.Data["username"] = []byte("")
	secret.Data["password"] = []byte("pass")
	tp.OnSecret(secret)
	if _, ok := tp.provider.(*tracesdk.TracerProvider); ok {
		t.Fatalf("expected noop TracerProvider after valid Secret")
	}
	if tp.username != "" || tp.password != "" {
		t.Errorf("Tracing provider is not updated with incorrect credentials")
	}
	secret.Data["username"] = []byte("")
	secret.Data["password"] = []byte("")
	tp.OnSecret(secret)
	if _, ok := tp.provider.(*tracesdk.TracerProvider); ok {
		t.Fatalf("expected noop TracerProvider after valid Secret")
	}
	if tp.username != "" || tp.password != "" {
		t.Errorf("Tracing provider is not updated with empty credentials")
	}

	secret.Data["username"] = []byte("user")
	secret.Data["password"] = []byte("pass")
	tp.OnSecret(secret)
	if _, ok := tp.provider.(*tracesdk.TracerProvider); !ok {
		t.Fatalf("expected real TracerProvider after valid Secret recovery")
	}
	if tp.username != "user" || tp.password != "pass" {
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
