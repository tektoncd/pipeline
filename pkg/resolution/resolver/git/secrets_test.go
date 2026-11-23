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

package git

import (
	"testing"
	"time"

	"github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/cache"
	fakekubeclient "k8s.io/client-go/kubernetes/fake"
	testingclock "k8s.io/utils/clock/testing"
)

func TestGetAPITokenCacheTTL(t *testing.T) {
	ctx := framework.InjectResolverConfigToContext(t.Context(), map[string]string{
		APISecretNameKey:      "config-secret",
		APISecretKeyKey:       "token",
		APISecretNamespaceKey: "config-ns",
	})
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "config-secret", Namespace: "config-ns"},
		Data:       map[string][]byte{"token": []byte("original")},
	}
	client := fakekubeclient.NewSimpleClientset(secret)
	clock := testingclock.NewFakeClock(time.Unix(0, 0))
	resolver := &GitResolver{
		Logger:     zap.NewNop().Sugar(),
		Cache:      cache.NewLRUExpireCacheWithClock(10, clock),
		TTL:        time.Minute,
		KubeClient: client,
	}
	token, err := resolver.getAPIToken(ctx, nil, APISecretNameKey)
	if err != nil || string(token) != "original" {
		t.Fatalf("initial token = %q, err = %v; want original", token, err)
	}
	secret.Data["token"] = []byte("rotated")
	if _, err := client.CoreV1().Secrets(secret.Namespace).Update(ctx, secret, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	clock.Step(resolver.TTL / 2)
	token, err = resolver.getAPIToken(ctx, nil, APISecretNameKey)
	if err != nil || string(token) != "original" {
		t.Fatalf("token before expiry = %q, err = %v; want original", token, err)
	}
	clock.Step(resolver.TTL/2 + time.Nanosecond)
	token, err = resolver.getAPIToken(ctx, nil, APISecretNameKey)
	if err != nil || string(token) != "rotated" {
		t.Fatalf("token after expiry = %q, err = %v; want rotated", token, err)
	}
}
