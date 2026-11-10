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

package artifactstorage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"strings"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1/types"
)

// VerifyingReader wraps an io.ReadCloser and verifies the SHA-256 digest of
// the content when the stream is fully consumed. It returns DigestMismatchError
// from Read once EOF is reached and the digest does not match.
type VerifyingReader struct {
	inner    io.ReadCloser
	hasher   hash.Hash
	expected string
	done     bool
}

// NewVerifyingReader returns a reader that hashes everything read from inner
// and compares against expectedDigest on EOF. expectedDigest should be the
// bare hex string (no "sha256:" prefix).
func NewVerifyingReader(inner io.ReadCloser, expectedDigest string) *VerifyingReader {
	return &VerifyingReader{
		inner:    inner,
		hasher:   sha256.New(),
		expected: expectedDigest,
	}
}

func (vr *VerifyingReader) Read(p []byte) (int, error) {
	n, err := vr.inner.Read(p)
	if n > 0 {
		vr.hasher.Write(p[:n])
	}
	if err == io.EOF && !vr.done {
		vr.done = true
		actual := hex.EncodeToString(vr.hasher.Sum(nil))
		if actual != vr.expected {
			return n, &DigestMismatchError{
				Algorithm: "sha256",
				Expected:  vr.expected,
				Actual:    actual,
			}
		}
	}
	return n, err
}

func (vr *VerifyingReader) Close() error {
	return vr.inner.Close()
}

// DigestMismatchError is returned when fetched content does not match the expected digest.
type DigestMismatchError struct {
	Algorithm string
	Expected  string
	Actual    string
}

func (e *DigestMismatchError) Error() string {
	return fmt.Sprintf("digest verification failed: expected %s:%s, got %s:%s",
		e.Algorithm, e.Expected, e.Algorithm, e.Actual)
}

// VerifiedFetch fetches content from the provider, wrapping it in a
// digest-verifying reader unless the provider is content-addressed.
// This is the function that Tekton core should call instead of calling
// Provider.Fetch directly.
func VerifiedFetch(ctx context.Context, provider Provider, ref *v1.ArtifactStorageRef) (io.ReadCloser, error) {
	rc, err := provider.Fetch(ctx, ref)
	if err != nil {
		return nil, err
	}

	if cap, ok := provider.(ContentAddressedProvider); ok && cap.ContentAddressed() {
		return rc, nil
	}

	digest := extractSHA256(ref.Digest)
	if digest == "" {
		return rc, nil
	}
	return NewVerifyingReader(rc, digest), nil
}

// extractSHA256 returns the hex portion of a "sha256:<hex>" digest string,
// or the string itself if it has no prefix.
func extractSHA256(digest string) string {
	if digest == "" {
		return ""
	}
	return strings.TrimPrefix(digest, "sha256:")
}
