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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	v1types "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1/types"
	"github.com/tektoncd/pipeline/pkg/artifactstorage"
)

// ArtifactInput describes an artifact to download before step execution.
type ArtifactInput struct {
	Name      string            `json:"name"`                // artifact name
	URI       string            `json:"uri,omitempty"`       // OCI URI (e.g., "registry:5000/artifacts/test@sha256:...")
	Path      string            `json:"path"`                // local path to extract to
	Inline    string            `json:"inline,omitempty"`    // base64-encoded tar.gz content (used instead of URI when set)
	Digest    map[string]string `json:"digest,omitempty"`    // content digest keyed by algorithm (e.g., "sha256": "<hex>")
	Reference bool              `json:"reference,omitempty"` // true for reference artifacts — no download needed, only metadata
}

// ArtifactOutput describes an artifact to upload after step execution.
type ArtifactOutput struct {
	Name       string               `json:"name"`                 // artifact name
	Type       v1types.ArtifactType `json:"type"`                 // "content" (transported by Tekton) or "reference" (step already stored it)
	Path       string               `json:"path"`                 // for content: local dir to archive from; for reference: file containing "uri@digest"
	Repository string               `json:"repository,omitempty"` // OCI repository to push to (content only)
	MediaType  string               `json:"mediaType,omitempty"`  // artifact media type (content only)
	Subject    bool                 `json:"subject,omitempty"`    // whether this is a SLSA attestation subject
}

// UploadArtifact processes an output artifact after step execution.
//
// For "reference" artifacts, the step already stored the content elsewhere;
// this reads the URI (and required digest) the step wrote to output.Path and
// returns them as-is — no upload happens. A reference artifact with no digest
// is rejected, since Tekton must be able to guarantee every propagated
// reference is digest-pinned.
//
// For "content" artifacts, this archives output.Path into a tar.gz buffer and
// decides based on inlineThreshold and the pod-wide inline budget whether to
// inline the content in the termination message or push it to the storage
// backend via the Provider interface:
//   - size <= inlineThreshold AND pod budget allows: base64-encode and return inline
//   - size fits inlineThreshold but pod budget exhausted, with provider: push to backend
//   - size > inlineThreshold with provider: push to backend
//   - size > inlineThreshold and pod budget exhausted without provider: error (no fallback)
//
// provider may be nil when storage is disabled. budget may be nil, in which
// case only the per-artifact threshold is checked.
func UploadArtifact(ctx context.Context, output ArtifactOutput, provider artifactstorage.Provider, inlineThreshold int, budget *InlineBudgetTracker) (*v1types.ArtifactValue, error) {
	if output.Type == v1types.ArtifactTypeReference {
		return readReferenceArtifact(output)
	}

	// Create a tar.gz buffer from the output path
	buf, err := createTarGzBuffer(output.Path)
	if err != nil {
		return nil, fmt.Errorf("creating tar.gz archive: %w", err)
	}

	dataBytes := buf.Bytes()
	dataSize := int64(len(dataBytes))

	h := sha256.New()
	h.Write(dataBytes)
	digestHex := hex.EncodeToString(h.Sum(nil))

	// Try to inline if the artifact fits the per-artifact threshold
	if inlineThreshold > 0 && int(dataSize) <= inlineThreshold {
		canInline := budget == nil || budget.TryConsume(int(dataSize))
		if canInline {
			return &v1types.ArtifactValue{
				Digest: map[v1types.Algorithm]string{"sha256": digestHex}, //nolint:goconst
				Size:   dataSize,
				Inline: base64.StdEncoding.EncodeToString(dataBytes),
			}, nil
		}
		// Pod budget exhausted — fall through to backend upload.
	}

	// No provider configured — cannot fall back to backend upload.
	if provider == nil {
		if inlineThreshold > 0 {
			if budget != nil && int(dataSize) <= inlineThreshold {
				return nil, &InlineBudgetExceededError{
					ArtifactName: output.Name,
					ArtifactSize: dataSize,
					Remaining:    budget.Remaining(),
				}
			}
			return nil, fmt.Errorf("artifact %q (%d bytes) exceeds inline threshold (%d) and no storage backend is configured",
				output.Name, dataSize, inlineThreshold)
		}
		slog.Warn("Artifact storage is not enabled, content artifact will be digested but not uploaded. "+
			"Set enabled: \"true\" in config-artifact-storage ConfigMap to enable uploads.",
			slog.String("name", output.Name), slog.Int64("size", dataSize))
		return &v1types.ArtifactValue{
			Digest: map[v1types.Algorithm]string{"sha256": digestHex},
			Size:   dataSize,
		}, nil
	}

	// upload path
	ref, err := provider.Store(ctx, artifactstorage.StoreOptions{
		ArtifactName: output.Name,
		MediaType:    output.MediaType,
		Repository:   output.Repository,
	}, bytes.NewReader(dataBytes))
	if err != nil {
		return nil, fmt.Errorf("storing artifact %q: %w", output.Name, err)
	}

	return &v1types.ArtifactValue{
		Uri:    ref.Location,
		Digest: map[v1types.Algorithm]string{"sha256": strings.TrimPrefix(ref.Digest, "sha256:")},
		Size:   dataSize,
		Ref:    ref,
	}, nil
}

// readReferenceArtifact reads the "uri@sha256:digest" line the step wrote for
// a reference-type artifact, and returns it as an ArtifactValue without any
// upload. The URI must carry a digest suffix ("@sha256:..."): a step that has
// just pushed content always knows its digest, and requiring it here
// guarantees every reference artifact recorded by Tekton is digest-pinned.
func readReferenceArtifact(output ArtifactOutput) (*v1types.ArtifactValue, error) {
	data, err := os.ReadFile(output.Path)
	if err != nil {
		return nil, fmt.Errorf("reading reference artifact %q: %w", output.Name, err)
	}
	uri := strings.TrimSpace(string(data))
	if uri == "" {
		return nil, fmt.Errorf("reference artifact %q: no uri written to %s", output.Name, output.Path)
	}
	idx := strings.LastIndex(uri, "@sha256:")
	if idx == -1 {
		return nil, fmt.Errorf("reference artifact %q: uri %q must carry a digest (\"...@sha256:<digest>\")", output.Name, uri)
	}
	digest := strings.TrimPrefix(uri[idx+1:], "sha256:")
	if digest == "" {
		return nil, fmt.Errorf("reference artifact %q: empty digest in uri %q", output.Name, uri)
	}
	return &v1types.ArtifactValue{
		Uri: uri,
		Digest: map[v1types.Algorithm]string{
			"sha256": digest,
		},
	}, nil
}

// DownloadArtifact fetches an artifact via the Provider interface and extracts
// its contents to input.Path. If the input carries inline data, it decodes and
// extracts directly without any network call.
//
// Digest verification is handled by the Provider layer (via VerifiedFetch for
// non-content-addressed backends, or inherently by the transport for
// content-addressed backends like OCI).
func DownloadArtifact(ctx context.Context, input ArtifactInput, provider artifactstorage.Provider) error {
	if input.Inline != "" {
		return verifyAndExtractInlineArtifact(input)
	}

	if provider == nil {
		return fmt.Errorf("no storage provider configured for downloading artifact %q", input.Name)
	}

	digestStr := ""
	if d, ok := input.Digest["sha256"]; ok {
		digestStr = "sha256:" + d
	}
	storageRef := &v1types.ArtifactStorageRef{
		Location: input.URI,
		Digest:   digestStr,
	}

	rc, err := artifactstorage.VerifiedFetch(ctx, provider, storageRef)
	if err != nil {
		return fmt.Errorf("fetching artifact %q from %s: %w", input.Name, input.URI, err)
	}
	defer rc.Close()

	return extractTarGz(rc, input.Path)
}

// verifyAndExtractInlineArtifact decodes base64 inline data, verifies its digest if
// expected, and extracts it to the input path.
func verifyAndExtractInlineArtifact(input ArtifactInput) error {
	data, err := base64.StdEncoding.DecodeString(input.Inline)
	if err != nil {
		return fmt.Errorf("decoding inline artifact: %w", err)
	}
	if err := verifyDigest(data, input.Digest); err != nil {
		return fmt.Errorf("inline artifact %q: %w", input.Name, err)
	}
	return extractTarGz(bytes.NewReader(data), input.Path)
}

// DigestMismatchError is returned when downloaded content does not match the expected digest.
type DigestMismatchError struct {
	Algorithm string
	Expected  string
	Actual    string
}

func (e *DigestMismatchError) Error() string {
	return fmt.Sprintf("digest verification failed: expected %s:%s, got %s:%s", e.Algorithm, e.Expected, e.Algorithm, e.Actual)
}

// verifyDigest computes the SHA-256 digest of data and compares it against
// expectedDigest. If expectedDigest is empty or has no "sha256" entry, this
// is a no-op.
func verifyDigest(data []byte, expectedDigest map[string]string) error {
	if len(expectedDigest) == 0 {
		return nil
	}
	expected, ok := expectedDigest["sha256"]
	if !ok {
		return nil
	}
	h := sha256.New()
	h.Write(data)
	actual := hex.EncodeToString(h.Sum(nil))
	if actual != expected {
		return &DigestMismatchError{Algorithm: "sha256", Expected: expected, Actual: actual}
	}
	return nil
}

// createTarGzBuffer creates a tar.gz archive of the directory at path into a buffer.
func createTarGzBuffer(path string) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	err := filepath.Walk(path, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(path, file)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
		header.Name = rel

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if fi.IsDir() {
			return nil
		}

		f, err := os.Open(file) //nolint:gosec // file comes from walking the artifact path we were given
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(tw, f)
		return err
	})
	if err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}

	return &buf, nil
}

// extractTarGz decompresses a gzip stream and extracts the tar entries to dst.
func extractTarGz(r io.Reader, dst string) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(dst, header.Name) //nolint:gosec // G305: traversal is rejected just below
		if !strings.HasPrefix(target, filepath.Clean(dst)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid tar entry: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, header.FileInfo().Mode().Perm())
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil { //nolint:gosec // size bounded by artifact content
				f.Close()
				return err
			}
			f.Close()
		}
	}
	return nil
}
