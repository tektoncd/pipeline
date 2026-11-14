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

package entrypoint

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	ocibackend "github.com/tektoncd/pipeline/pkg/artifactstorage/oci"
)

func testOCIProvider(srv *httptest.Server) *ocibackend.OCIProvider {
	return ocibackend.NewOCIProvider("",
		ocibackend.WithInsecure(true),
		ocibackend.WithRemoteOptions(remote.WithTransport(srv.Client().Transport)),
	)
}

func TestUploadAndDownloadArtifact(t *testing.T) {
	reg := registry.New()
	srv := httptest.NewServer(reg)
	defer srv.Close()

	registryHost := strings.TrimPrefix(srv.URL, "http://")
	provider := testOCIProvider(srv)

	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "result.json"), []byte(`{"passed": true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "sub", "nested.txt"), []byte("nested content"), 0o644); err != nil {
		t.Fatal(err)
	}

	output := ArtifactOutput{
		Name:       "test-results",
		Path:       srcDir,
		Repository: registryHost + "/artifacts/test-results",
		MediaType:  "application/vnd.tekton.artifact.test.v1+tar.gz",
	}

	ctx := context.Background()
	av, err := UploadArtifact(ctx, output, provider, 0, nil)
	if err != nil {
		t.Fatalf("UploadArtifact() error = %v", err)
	}

	if av.Uri == "" {
		t.Error("expected non-empty URI")
	}
	if len(av.Digest) == 0 {
		t.Error("expected at least one digest entry")
	}
	if _, ok := av.Digest["sha256"]; !ok {
		t.Error("expected sha256 digest")
	}
	if av.Ref == nil {
		t.Error("expected Ref to be populated for OCI upload")
	}
	if av.Size == 0 {
		t.Error("expected non-zero Size")
	}

	dstDir := t.TempDir()
	input := ArtifactInput{
		Name: "test-results",
		URI:  av.Uri,
		Path: dstDir,
	}

	if err := DownloadArtifact(ctx, input, provider); err != nil {
		t.Fatalf("DownloadArtifact() error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dstDir, "result.json"))
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(content) != `{"passed": true}` {
		t.Errorf("content mismatch: got %q", string(content))
	}

	nested, err := os.ReadFile(filepath.Join(dstDir, "sub", "nested.txt"))
	if err != nil {
		t.Fatalf("failed to read nested file: %v", err)
	}
	if string(nested) != "nested content" {
		t.Errorf("nested content mismatch: got %q", string(nested))
	}
}

func TestUploadArtifact_EmptyDir(t *testing.T) {
	reg := registry.New()
	srv := httptest.NewServer(reg)
	defer srv.Close()

	registryHost := strings.TrimPrefix(srv.URL, "http://")
	provider := testOCIProvider(srv)

	srcDir := t.TempDir()
	output := ArtifactOutput{
		Name:       "empty",
		Path:       srcDir,
		Repository: registryHost + "/artifacts/empty",
		MediaType:  "application/vnd.tekton.artifact.v1+tar.gz",
	}

	ctx := context.Background()
	av, err := UploadArtifact(ctx, output, provider, 0, nil)
	if err != nil {
		t.Fatalf("UploadArtifact() error = %v", err)
	}
	if av.Uri == "" {
		t.Error("expected non-empty URI even for empty dir")
	}
}

func TestDownloadArtifact_InvalidURI(t *testing.T) {
	reg := registry.New()
	srv := httptest.NewServer(reg)
	defer srv.Close()
	provider := testOCIProvider(srv)

	ctx := context.Background()
	input := ArtifactInput{
		Name: "bad",
		URI:  "not-a-valid-reference",
		Path: t.TempDir(),
	}
	err := DownloadArtifact(ctx, input, provider)
	if err == nil {
		t.Error("expected error for invalid URI")
	}
}

func TestDownloadArtifact_NilProvider(t *testing.T) {
	ctx := context.Background()
	input := ArtifactInput{
		Name: "test",
		URI:  "registry.example.com/artifacts/test@sha256:" + strings.Repeat("a", 64),
		Path: t.TempDir(),
	}
	err := DownloadArtifact(ctx, input, nil)
	if err == nil {
		t.Error("expected error when provider is nil")
	}
	if !strings.Contains(err.Error(), "no storage provider") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUploadArtifact_ReferenceType(t *testing.T) {
	srcDir := t.TempDir()
	uriFile := filepath.Join(srcDir, "image.uri")
	if err := os.WriteFile(uriFile, []byte("registry.example.com/myapp@sha256:"+strings.Repeat("a", 64)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	output := ArtifactOutput{
		Name: "image",
		Type: "reference",
		Path: uriFile,
	}

	av, err := UploadArtifact(context.Background(), output, nil, 0, nil)
	if err != nil {
		t.Fatalf("UploadArtifact() error = %v", err)
	}
	wantURI := "registry.example.com/myapp@sha256:" + strings.Repeat("a", 64)
	if av.Uri != wantURI {
		t.Errorf("Uri = %q, want %q", av.Uri, wantURI)
	}
	if av.Digest["sha256"] != strings.Repeat("a", 64) {
		t.Errorf("Digest[sha256] = %q, want %q", av.Digest["sha256"], strings.Repeat("a", 64))
	}
}

func TestUploadArtifact_ReferenceType_NoDigest(t *testing.T) {
	srcDir := t.TempDir()
	uriFile := filepath.Join(srcDir, "image.uri")
	if err := os.WriteFile(uriFile, []byte("registry.example.com/myapp:latest"), 0o644); err != nil {
		t.Fatal(err)
	}

	output := ArtifactOutput{
		Name: "image",
		Type: "reference",
		Path: uriFile,
	}

	if _, err := UploadArtifact(context.Background(), output, nil, 0, nil); err == nil {
		t.Fatal("expected error for reference artifact missing a digest")
	}
}

func TestUploadArtifact_ContentType_NoProvider_DigestOnly(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "data.json"), []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	output := ArtifactOutput{
		Name: "results",
		Type: "content",
		Path: srcDir,
	}

	av, err := UploadArtifact(context.Background(), output, nil, 0, nil)
	if err != nil {
		t.Fatalf("UploadArtifact() error = %v", err)
	}
	if av.Uri != "" {
		t.Errorf("expected no Uri when storage is disabled, got %q", av.Uri)
	}
	if av.Digest["sha256"] == "" {
		t.Error("expected a sha256 digest even when storage is disabled")
	}
}

func TestUploadArtifact_InlineSmallContent(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "small.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	output := ArtifactOutput{
		Name: "small",
		Type: "content",
		Path: srcDir,
	}

	av, err := UploadArtifact(context.Background(), output, nil, 4096, nil)
	if err != nil {
		t.Fatalf("UploadArtifact() error = %v", err)
	}
	if av.Inline == "" {
		t.Fatal("expected Inline to be populated for small content")
	}
	if av.Uri != "" {
		t.Errorf("expected no Uri for inlined content, got %q", av.Uri)
	}
	if av.Ref != nil {
		t.Error("expected no Ref for inlined content")
	}
	if av.Digest["sha256"] == "" {
		t.Error("expected sha256 digest for inlined content")
	}
	if av.Size == 0 {
		t.Error("expected non-zero Size for inlined content")
	}
}

func TestUploadArtifact_InlineExceedsThreshold_NoProvider_Error(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "big.bin"), make([]byte, 2000), 0o644); err != nil {
		t.Fatal(err)
	}

	output := ArtifactOutput{
		Name: "big",
		Type: "content",
		Path: srcDir,
	}

	_, err := UploadArtifact(context.Background(), output, nil, 100, nil)
	if err == nil {
		t.Fatal("expected error when content exceeds inline threshold and no provider is configured")
	}
	if !strings.Contains(err.Error(), "exceeds inline threshold") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestUploadArtifact_InlineExceedsThreshold_WithProvider_Uploads(t *testing.T) {
	reg := registry.New()
	srv := httptest.NewServer(reg)
	defer srv.Close()
	registryHost := strings.TrimPrefix(srv.URL, "http://")
	provider := testOCIProvider(srv)

	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "big.bin"), make([]byte, 2000), 0o644); err != nil {
		t.Fatal(err)
	}

	output := ArtifactOutput{
		Name:       "big",
		Type:       "content",
		Path:       srcDir,
		Repository: registryHost + "/artifacts/big",
	}

	av, err := UploadArtifact(context.Background(), output, provider, 100, nil)
	if err != nil {
		t.Fatalf("UploadArtifact() error = %v", err)
	}
	if av.Inline != "" {
		t.Error("expected no Inline for content above threshold")
	}
	if av.Uri == "" {
		t.Error("expected Uri when falling back to backend upload")
	}
	if av.Ref == nil {
		t.Error("expected Ref for backend upload")
	}
	if av.Ref != nil && av.Ref.Backend != "oci" {
		t.Errorf("expected backend=oci, got %q", av.Ref.Backend)
	}
}

func TestUploadArtifact_InlineBudgetExhausted_FallsBackToProvider(t *testing.T) {
	reg := registry.New()
	srv := httptest.NewServer(reg)
	defer srv.Close()
	registryHost := strings.TrimPrefix(srv.URL, "http://")
	provider := testOCIProvider(srv)

	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "small.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	output := ArtifactOutput{
		Name:       "small",
		Type:       "content",
		Path:       srcDir,
		Repository: registryHost + "/artifacts/small",
	}

	budgetFile := filepath.Join(t.TempDir(), "budget")
	budget := NewInlineBudgetTracker(budgetFile, 10)
	budget.TryConsume(10)

	av, err := UploadArtifact(context.Background(), output, provider, 4096, budget)
	if err != nil {
		t.Fatalf("UploadArtifact() error = %v", err)
	}
	if av.Inline != "" {
		t.Error("expected no Inline when pod budget is exhausted")
	}
	if av.Uri == "" {
		t.Error("expected Uri when falling back to backend upload")
	}
}

func TestUploadArtifact_InlineBudgetExhausted_NoProvider_Error(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "small.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	output := ArtifactOutput{
		Name: "small",
		Type: "content",
		Path: srcDir,
	}

	budgetFile := filepath.Join(t.TempDir(), "budget")
	budget := NewInlineBudgetTracker(budgetFile, 10)
	budget.TryConsume(10)

	_, err := UploadArtifact(context.Background(), output, nil, 4096, budget)
	if err == nil {
		t.Fatal("expected error when budget is exhausted and no provider is configured")
	}
	var budgetErr *InlineBudgetExceededError
	if !errors.As(err, &budgetErr) {
		t.Errorf("expected InlineBudgetExceededError, got %T: %v", err, err)
	}
}

func TestUploadArtifact_InlineBudgetAllowsInline(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "small.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	output := ArtifactOutput{
		Name: "small",
		Type: "content",
		Path: srcDir,
	}

	budgetFile := filepath.Join(t.TempDir(), "budget")
	budget := NewInlineBudgetTracker(budgetFile, 2048)

	av, err := UploadArtifact(context.Background(), output, nil, 4096, budget)
	if err != nil {
		t.Fatalf("UploadArtifact() error = %v", err)
	}
	if av.Inline == "" {
		t.Fatal("expected Inline to be populated when budget allows")
	}
	if budget.Consumed() == 0 {
		t.Error("expected budget to be consumed after inlining")
	}
}

func TestDownloadArtifact_OCI_RoundTrip(t *testing.T) {
	reg := registry.New()
	srv := httptest.NewServer(reg)
	defer srv.Close()
	registryHost := strings.TrimPrefix(srv.URL, "http://")
	provider := testOCIProvider(srv)

	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "data.json"), []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	output := ArtifactOutput{
		Name:       "verified",
		Type:       "content",
		Path:       srcDir,
		Repository: registryHost + "/artifacts/verified",
	}

	ctx := context.Background()
	av, err := UploadArtifact(ctx, output, provider, 0, nil)
	if err != nil {
		t.Fatalf("UploadArtifact() error = %v", err)
	}

	dstDir := t.TempDir()
	input := ArtifactInput{
		Name:   "verified",
		URI:    av.Uri,
		Path:   dstDir,
		Digest: map[string]string{"sha256": av.Digest["sha256"]},
	}
	if err := DownloadArtifact(ctx, input, provider); err != nil {
		t.Fatalf("DownloadArtifact() error = %v", err)
	}
	content, err := os.ReadFile(filepath.Join(dstDir, "data.json"))
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(content) != `{"ok":true}` {
		t.Errorf("content mismatch: got %q", string(content))
	}
}

func TestDownloadArtifact_Inline_DigestVerification(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "hello.txt"), []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	buf, err := createTarGzBuffer(srcDir)
	if err != nil {
		t.Fatal(err)
	}
	dataBytes := buf.Bytes()
	encoded := base64.StdEncoding.EncodeToString(dataBytes)

	h := sha256.New()
	h.Write(dataBytes)
	correctDigest := hex.EncodeToString(h.Sum(nil))

	t.Run("correct digest", func(t *testing.T) {
		dstDir := t.TempDir()
		input := ArtifactInput{
			Name:   "inline-verified",
			Inline: encoded,
			Path:   dstDir,
			Digest: map[string]string{"sha256": correctDigest},
		}
		if err := DownloadArtifact(context.Background(), input, nil); err != nil {
			t.Fatalf("DownloadArtifact() error = %v", err)
		}
		content, err := os.ReadFile(filepath.Join(dstDir, "hello.txt"))
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if string(content) != "hello world" {
			t.Errorf("content mismatch: got %q", string(content))
		}
	})

	t.Run("wrong digest", func(t *testing.T) {
		dstDir := t.TempDir()
		input := ArtifactInput{
			Name:   "inline-verified",
			Inline: encoded,
			Path:   dstDir,
			Digest: map[string]string{"sha256": strings.Repeat("f", 64)},
		}
		err := DownloadArtifact(context.Background(), input, nil)
		if err == nil {
			t.Fatal("expected error for wrong digest")
		}
		var mismatch *DigestMismatchError
		if !errors.As(err, &mismatch) {
			t.Fatalf("expected DigestMismatchError, got %T: %v", err, err)
		}
	})

	t.Run("no digest skips verification", func(t *testing.T) {
		dstDir := t.TempDir()
		input := ArtifactInput{
			Name:   "inline-verified",
			Inline: encoded,
			Path:   dstDir,
		}
		if err := DownloadArtifact(context.Background(), input, nil); err != nil {
			t.Fatalf("DownloadArtifact() error = %v", err)
		}
	})
}

func TestVerifyDigest(t *testing.T) {
	data := []byte("test data for digest verification")
	h := sha256.New()
	h.Write(data)
	correctDigest := hex.EncodeToString(h.Sum(nil))

	tests := []struct {
		name    string
		digest  map[string]string
		wantErr bool
	}{
		{name: "nil digest", digest: nil, wantErr: false},
		{name: "empty digest", digest: map[string]string{}, wantErr: false},
		{name: "correct sha256", digest: map[string]string{"sha256": correctDigest}, wantErr: false},
		{name: "wrong sha256", digest: map[string]string{"sha256": strings.Repeat("0", 64)}, wantErr: true},
		{name: "unknown algorithm only", digest: map[string]string{"md5": "abc"}, wantErr: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyDigest(data, tc.digest)
			if (err != nil) != tc.wantErr {
				t.Errorf("verifyDigest() error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				var mismatch *DigestMismatchError
				if !errors.As(err, &mismatch) {
					t.Errorf("expected DigestMismatchError, got %T", err)
				}
			}
		})
	}
}
