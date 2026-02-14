//go:build !windows

/*
Copyright 2025 The Tekton Authors

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

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaskingWriter(t *testing.T) {
	testCases := []struct {
		name     string
		secrets  []string
		input    string
		expected string
	}{
		{
			name:     "no secrets",
			secrets:  nil,
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "single secret",
			secrets:  []string{"password123"},
			input:    "The password is password123!",
			expected: "The password is ***!",
		},
		{
			name:     "multiple secrets",
			secrets:  []string{"secret1", "secret2"},
			input:    "secret1 and secret2 are here",
			expected: "*** and *** are here",
		},
		{
			name:     "short secret skipped",
			secrets:  []string{"ab"},
			input:    "ab ab ab", //nolint:dupword // intentional test data with repeated words
			expected: "ab ab ab", //nolint:dupword // intentional test data with repeated words
		},
		{
			name:     "repeated secret",
			secrets:  []string{"token"},
			input:    "token token token", //nolint:dupword // intentional test data with repeated words
			expected: "*** *** ***",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := newMaskingWriter(&buf, tc.secrets)
			if _, err := w.Write([]byte(tc.input)); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err := w.Flush(); err != nil {
				t.Fatalf("unexpected flush error: %v", err)
			}
			if got := buf.String(); got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestMaskingWriterSplitAcrossWrites(t *testing.T) {
	var buf bytes.Buffer
	w := newMaskingWriter(&buf, []string{"supersecretpassword"})

	if _, err := w.Write([]byte("prefix supersecret")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := w.Write([]byte("password suffix")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("unexpected flush error: %v", err)
	}

	if got, want := buf.String(), "prefix *** suffix"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestMaskingWriterLargeSecretAcrossChunks(t *testing.T) {
	secret := strings.Repeat("s", 70000)
	var buf bytes.Buffer
	w := newMaskingWriter(&buf, []string{secret})

	prefix := "start "
	if _, err := w.Write([]byte(prefix)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := 0; i < len(secret); i += 8192 {
		end := i + 8192
		if end > len(secret) {
			end = len(secret)
		}
		if _, err := w.Write([]byte(secret[i:end])); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	suffix := " end"
	if _, err := w.Write([]byte(suffix)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("unexpected flush error: %v", err)
	}

	got := buf.String()
	if strings.Contains(got, secret) {
		t.Fatal("expected long secret to be masked")
	}
	want := prefix + "***" + suffix
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestMaskingWriterOverlappingSecrets(t *testing.T) {
	var buf bytes.Buffer
	w := newMaskingWriter(&buf, []string{"abc", "abcdef"})

	if _, err := w.Write([]byte("abcdef")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("unexpected flush error: %v", err)
	}

	if got, want := buf.String(), "***"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestLoadSecretsForMasking(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "secrets")
	content := "c2VjcmV0MQ==\ncGFzc3dvcmQ=\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	got, err := loadSecretsForMasking(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := map[string]bool{"secret1": true, "password": true}
	for _, s := range got {
		if !expected[s] {
			t.Fatalf("unexpected secret: %s", s)
		}
		delete(expected, s)
	}
	if len(expected) != 0 {
		t.Fatalf("missing secrets: %v", expected)
	}
}

func TestLoadSecretsForMaskingInvalidLine(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "secrets")
	if err := os.WriteFile(filePath, []byte("not-base64\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	if _, err := loadSecretsForMasking(filePath); err == nil {
		t.Fatalf("expected error")
	}
}
