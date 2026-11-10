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
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"testing"
)

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

func TestVerifyingReader_MatchingDigest(t *testing.T) {
	content := []byte("hello world")
	h := sha256.Sum256(content)
	expected := hex.EncodeToString(h[:])

	vr := NewVerifyingReader(nopCloser{bytes.NewReader(content)}, expected)
	got, err := io.ReadAll(vr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
}

func TestVerifyingReader_MismatchedDigest(t *testing.T) {
	content := []byte("hello world")
	wrongDigest := "0000000000000000000000000000000000000000000000000000000000000000"

	vr := NewVerifyingReader(nopCloser{bytes.NewReader(content)}, wrongDigest)
	_, err := io.ReadAll(vr)
	if err == nil {
		t.Fatal("expected digest mismatch error, got nil")
	}

	var mismatch *DigestMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("expected DigestMismatchError, got %T: %v", err, err)
	}
	if mismatch.Expected != wrongDigest {
		t.Errorf("expected digest %q, got %q", wrongDigest, mismatch.Expected)
	}
}

func TestVerifyingReader_EmptyContent(t *testing.T) {
	content := []byte{}
	h := sha256.Sum256(content)
	expected := hex.EncodeToString(h[:])

	vr := NewVerifyingReader(nopCloser{bytes.NewReader(content)}, expected)
	got, err := io.ReadAll(vr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty content, got %d bytes", len(got))
	}
}

func TestExtractSHA256(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"sha256:abc123", "abc123"},
		{"abc123", "abc123"},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractSHA256(tt.input)
		if got != tt.want {
			t.Errorf("extractSHA256(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
