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

package subcommands

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSecretMaskInit(t *testing.T) {
	for _, tt := range []struct {
		name        string
		envValue    string
		wantErr     string
		wantContent string
	}{{
		name:        "writes decoded secret mask data",
		envValue:    mustGzipAndBase64("c2VjcmV0MQ==\ncGFzc3dvcmQ=\n"),
		wantContent: "c2VjcmV0MQ==\ncGFzc3dvcmQ=\n",
	}, {
		name:        "file has correct permissions",
		envValue:    mustGzipAndBase64("secret1\n"),
		wantContent: "secret1\n",
	}, {
		name:     "missing env var",
		envValue: "",
		wantErr:  "is not set",
	}, {
		name:     "invalid base64 payload",
		envValue: "%%%",
		wantErr:  "decode",
	}, {
		name:     "invalid gzip payload",
		envValue: base64.StdEncoding.EncodeToString([]byte("not-gzip")),
		wantErr:  "decompress",
	}} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(SecretMaskDataEnvVar, tt.envValue)
			filePath := filepath.Join(t.TempDir(), "secret-mask", "secrets")

			err := secretMaskInit(filePath)

			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			data, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("failed reading output file: %v", err)
			}
			if got := string(data); got != tt.wantContent {
				t.Errorf("got content %q, want %q", got, tt.wantContent)
			}

			info, err := os.Stat(filePath)
			if err != nil {
				t.Fatalf("failed to stat output file: %v", err)
			}
			expectedPerm := os.FileMode(0444)
			if got := info.Mode().Perm(); got != expectedPerm {
				t.Errorf("expected mode %#o, got %#o", expectedPerm, got)
			}
		})
	}
}

func mustGzipAndBase64(data string) string {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(data)); err != nil {
		panic(err)
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}
