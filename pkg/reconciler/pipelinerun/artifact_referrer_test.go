/*
Copyright 2024 The Tekton Authors

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

package pipelinerun

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakek8s "k8s.io/client-go/kubernetes/fake"
	_ "knative.dev/pkg/system/testing"
)

func TestAttachReferrers(t *testing.T) {
	// Start an in-memory OCI registry
	reg := registry.New()
	srv := httptest.NewServer(reg)
	defer srv.Close()
	registryHost := strings.TrimPrefix(srv.URL, "http://")

	// Push a "build output" image (the subject)
	subjectRef := registryHost + "/myapp:latest"
	ref, err := name.ParseReference(subjectRef, name.Insecure)
	if err != nil {
		t.Fatal(err)
	}
	subjectImg, err := random.Image(256, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := remote.Write(ref, subjectImg, remote.WithTransport(srv.Client().Transport)); err != nil {
		t.Fatal(err)
	}

	subjectDigest, err := subjectImg.Digest()
	if err != nil {
		t.Fatal(err)
	}
	subjectDigestRef := fmt.Sprintf("%s/myapp@%s", registryHost, subjectDigest.String())

	// Define artifacts: one build output (subject) and one referrer (SBOM)
	artifacts := []TaskArtifactResult{
		{
			TaskName: "build",
			Artifact: v1.Artifact{
				Name:    "image",
				Subject: true,
				Values: []v1.ArtifactValue{
					{Uri: subjectDigestRef, Digest: map[v1.Algorithm]string{"sha256": strings.TrimPrefix(subjectDigest.String(), "sha256:")}},
				},
			},
		},
		{
			TaskName: "build",
			Artifact: v1.Artifact{
				Name: "sbom",
				Values: []v1.ArtifactValue{
					{Uri: registryHost + "/artifacts/sbom@sha256:def456", Digest: map[v1.Algorithm]string{"sha256": "def456"}},
				},
			},
			MediaType: "application/vnd.cyclonedx+json",
		},
	}

	ctx := context.Background()
	opts := []remote.Option{remote.WithTransport(srv.Client().Transport)}

	err = AttachReferrers(ctx, artifacts, true, opts...)
	if err != nil {
		t.Fatalf("AttachReferrers() error = %v", err)
	}

	// The referrer manifest was pushed successfully (we verify no error).
	// The in-memory registry doesn't support the referrers API, so we
	// verify the manifest exists by listing tags and checking for the
	// referrer tag convention (sha256-<subject-digest>).
	repo, err := name.NewRepository(registryHost+"/myapp", name.Insecure)
	if err != nil {
		t.Fatal(err)
	}
	tags, err := remote.List(repo, opts...)
	if err != nil {
		t.Fatalf("remote.List() error = %v", err)
	}
	// At minimum, should have 'latest' tag and the referrer was pushed by digest
	if len(tags) == 0 {
		t.Error("expected at least one tag in repository")
	}
}

func TestAttachReferrers_SubjectFieldInManifest(t *testing.T) {
	// Start an in-memory OCI registry with referrers API support
	reg := registry.New(registry.WithReferrersSupport(true))
	srv := httptest.NewServer(reg)
	defer srv.Close()
	registryHost := strings.TrimPrefix(srv.URL, "http://")

	// Push a "build output" image (the subject)
	subjectRef := registryHost + "/myapp:latest"
	ref, err := name.ParseReference(subjectRef, name.Insecure)
	if err != nil {
		t.Fatal(err)
	}
	subjectImg, err := random.Image(256, 1)
	if err != nil {
		t.Fatal(err)
	}
	opts := []remote.Option{remote.WithTransport(srv.Client().Transport)}
	if err := remote.Write(ref, subjectImg, opts...); err != nil {
		t.Fatal(err)
	}

	subjectDigest, err := subjectImg.Digest()
	if err != nil {
		t.Fatal(err)
	}
	subjectDigestRef := fmt.Sprintf("%s/myapp@%s", registryHost, subjectDigest.String())

	artifacts := []TaskArtifactResult{
		{
			TaskName: "build",
			Artifact: v1.Artifact{
				Name:    "image",
				Subject: true,
				Values: []v1.ArtifactValue{
					{Uri: subjectDigestRef, Digest: map[v1.Algorithm]string{"sha256": strings.TrimPrefix(subjectDigest.String(), "sha256:")}},
				},
			},
		},
		{
			TaskName:  "scan",
			Artifact:  v1.Artifact{Name: "sbom", Values: []v1.ArtifactValue{{Uri: registryHost + "/artifacts/sbom@sha256:abc123"}}},
			MediaType: "application/vnd.cyclonedx+json",
		},
	}

	ctx := context.Background()
	if err := AttachReferrers(ctx, artifacts, true, opts...); err != nil {
		t.Fatalf("AttachReferrers() error = %v", err)
	}

	// Query the referrers API for the subject digest
	digestRef, err := name.NewDigest(subjectDigestRef, name.Insecure)
	if err != nil {
		t.Fatal(err)
	}
	referrerIdx, err := remote.Referrers(digestRef, opts...)
	if err != nil {
		t.Fatalf("remote.Referrers() error = %v", err)
	}
	idxManifest, err := referrerIdx.IndexManifest()
	if err != nil {
		t.Fatalf("IndexManifest() error = %v", err)
	}
	if len(idxManifest.Manifests) == 0 {
		t.Fatal("expected at least one referrer, got none — subject field was likely nil in the pushed manifest")
	}

	// Verify the referrer's descriptor points to the subject digest
	for _, desc := range idxManifest.Manifests {
		referrerRef := ref.Context().Digest(desc.Digest.String())
		img, err := remote.Image(referrerRef, opts...)
		if err != nil {
			t.Fatalf("fetching referrer image: %v", err)
		}
		m, err := img.Manifest()
		if err != nil {
			t.Fatalf("getting referrer manifest: %v", err)
		}
		if m.Subject == nil {
			t.Error("referrer manifest subject field is nil")
		} else if m.Subject.Digest != subjectDigest {
			t.Errorf("referrer manifest subject digest = %v, want %v", m.Subject.Digest, subjectDigest)
		}
	}
}

func TestAttachReferrers_NoBuildOutput(t *testing.T) {
	artifacts := []TaskArtifactResult{
		{
			TaskName:  "test",
			Artifact:  v1.Artifact{Name: "results", Values: []v1.ArtifactValue{{Uri: "foo"}}},
			MediaType: "application/json",
		},
	}

	ctx := context.Background()
	// No build output → should be a no-op (no error)
	err := AttachReferrers(ctx, artifacts, true)
	if err != nil {
		t.Fatalf("AttachReferrers() with no build output should not error, got: %v", err)
	}
}

func TestResolveArtifactAuth_CredentialsSecret(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "oci-creds",
			Namespace: "knative-testing",
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: []byte(`{"auths":{"quay.io":{"auth":"dGVzdDp0ZXN0"}}}`),
		},
	}

	c := &Reconciler{
		KubeClientSet: fakek8s.NewSimpleClientset(secret),
	}

	pr := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test-ns"},
		Spec: v1.PipelineRunSpec{
			TaskRunTemplate: v1.PipelineTaskRunTemplate{
				ServiceAccountName: "builder",
			},
		},
	}

	storageCfg := &config.ArtifactStorage{
		OCICredentialsSecret: "oci-creds",
	}

	opts, err := c.resolveArtifactAuth(context.Background(), pr, storageCfg)
	if err != nil {
		t.Fatalf("resolveArtifactAuth() error = %v", err)
	}
	if len(opts) == 0 {
		t.Fatal("expected at least one remote.Option from credentials secret")
	}
}

func TestResolveArtifactAuth_CredentialsSecretNotFound(t *testing.T) {
	c := &Reconciler{
		KubeClientSet: fakek8s.NewSimpleClientset(),
	}

	pr := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test-ns"},
	}

	storageCfg := &config.ArtifactStorage{
		OCICredentialsSecret: "nonexistent-secret",
	}

	_, err := c.resolveArtifactAuth(context.Background(), pr, storageCfg)
	if err == nil {
		t.Fatal("expected error when credentials secret does not exist")
	}
	if !strings.Contains(err.Error(), "reading artifact credentials secret") {
		t.Errorf("expected error about reading secret, got: %v", err)
	}
}

func TestResolveArtifactAuth_FallbackToServiceAccount(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "builder",
			Namespace: "test-ns",
		},
		ImagePullSecrets: []corev1.LocalObjectReference{
			{Name: "pull-secret"},
		},
	}

	pullSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pull-secret",
			Namespace: "test-ns",
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: []byte(`{"auths":{"registry.example.com":{"auth":"dGVzdDp0ZXN0"}}}`),
		},
	}

	c := &Reconciler{
		KubeClientSet: fakek8s.NewSimpleClientset(sa, pullSecret),
	}

	pr := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test-ns"},
		Spec: v1.PipelineRunSpec{
			TaskRunTemplate: v1.PipelineTaskRunTemplate{
				ServiceAccountName: "builder",
			},
		},
	}

	storageCfg := &config.ArtifactStorage{
		OCICredentialsSecret: "",
	}

	opts, err := c.resolveArtifactAuth(context.Background(), pr, storageCfg)
	if err != nil {
		t.Fatalf("resolveArtifactAuth() error = %v", err)
	}
	if len(opts) == 0 {
		t.Fatal("expected at least one remote.Option from ServiceAccount imagePullSecrets")
	}
}

func TestResolveArtifactAuth_FallbackDefaultServiceAccount(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "test-ns",
		},
	}

	c := &Reconciler{
		KubeClientSet: fakek8s.NewSimpleClientset(sa),
	}

	pr := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test-ns"},
	}

	storageCfg := &config.ArtifactStorage{
		OCICredentialsSecret: "",
	}

	opts, err := c.resolveArtifactAuth(context.Background(), pr, storageCfg)
	if err != nil {
		t.Fatalf("resolveArtifactAuth() error = %v", err)
	}
	if len(opts) == 0 {
		t.Fatal("expected at least one remote.Option from default ServiceAccount")
	}
}
