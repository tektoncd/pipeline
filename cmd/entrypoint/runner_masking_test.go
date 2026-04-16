//go:build !windows

package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSecretsFromDir(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string // relative path -> content
		want    []string
		wantLen int
	}{
		{
			name:    "single secret file",
			files:   map[string]string{"my-secret/password": "s3cret!"},
			want:    []string{"s3cret!"},
			wantLen: 1,
		},
		{
			name: "multiple keys in one secret",
			files: map[string]string{
				"db-creds/username": "admin",
				"db-creds/password": "hunter2",
			},
			wantLen: 2,
		},
		{
			name: "multiple secrets",
			files: map[string]string{
				"secret-a/key": "value-a",
				"secret-b/key": "value-b",
			},
			wantLen: 2,
		},
		{
			name:    "whitespace-only file is skipped",
			files:   map[string]string{"my-secret/empty": "   \n  "},
			wantLen: 0,
		},
		{
			name:    "empty file is skipped",
			files:   map[string]string{"my-secret/empty": ""},
			wantLen: 0,
		},
		{
			name:    "dotdot files are skipped",
			files:   map[string]string{"my-secret/..data": "internal"},
			wantLen: 0,
		},
		{
			name:    "trailing whitespace is trimmed",
			files:   map[string]string{"my-secret/token": "  mytoken  \n"},
			want:    []string{"mytoken"},
			wantLen: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for relPath, content := range tc.files {
				fullPath := filepath.Join(dir, relPath)
				if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			got := loadSecretsFromDir(dir)
			if len(got) != tc.wantLen {
				t.Fatalf("got %d secrets, want %d: %v", len(got), tc.wantLen, got)
			}
			for _, want := range tc.want {
				found := false
				for _, g := range got {
					if g == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected secret %q not found in %v", want, got)
				}
			}
		})
	}
}

func TestLoadSecretsFromDirNonExistent(t *testing.T) {
	got := loadSecretsFromDir("/nonexistent/path/that/does/not/exist")
	if len(got) != 0 {
		t.Errorf("expected no secrets from nonexistent dir, got %v", got)
	}
}

func TestLoadMaskingConfig(t *testing.T) {
	tests := []struct {
		name              string
		redactPatterns    string
		secretMaskDir     string
		secretFiles       map[string]string
		wantRegexCount    int
		wantSecretsCount  int
		wantFirstRegex    string
		wantFirstSecret   string
	}{
		{
			name:             "no config returns empty",
			redactPatterns:   "",
			secretMaskDir:    "",
			wantRegexCount:   0,
			wantSecretsCount: 0,
		},
		{
			name:           "base64-encoded regex patterns decoded",
			redactPatterns: base64.StdEncoding.EncodeToString([]byte("pattern-a\npattern-b")),
			secretMaskDir:  "",
			wantRegexCount: 2,
			wantFirstRegex: "pattern-a",
		},
		{
			name:           "invalid base64 returns no patterns",
			redactPatterns: "not-valid-base64!!!",
			secretMaskDir:  "",
			wantRegexCount: 0,
		},
		{
			name:           "blank lines in patterns are skipped",
			redactPatterns: base64.StdEncoding.EncodeToString([]byte("pattern-a\n\n  \npattern-b")),
			wantRegexCount: 2,
		},
		{
			name:              "secrets loaded from directory",
			redactPatterns:    "",
			secretFiles:       map[string]string{"my-secret/key": "secret-value"},
			wantRegexCount:    0,
			wantSecretsCount:  1,
			wantFirstSecret:   "secret-value",
		},
		{
			name:             "both regex and secrets",
			redactPatterns:   base64.StdEncoding.EncodeToString([]byte("jwt-pattern")),
			secretFiles:      map[string]string{"s/k": "exact-val"},
			wantRegexCount:   1,
			wantSecretsCount: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			secretDir := ""
			if tc.secretFiles != nil {
				secretDir = t.TempDir()
				for relPath, content := range tc.secretFiles {
					fullPath := filepath.Join(secretDir, relPath)
					if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
						t.Fatal(err)
					}
				}
			}
			if tc.secretMaskDir != "" {
				secretDir = tc.secretMaskDir
			}

			rr := &realRunner{
				redactPatterns: tc.redactPatterns,
				secretMaskDir:  secretDir,
			}

			regexPatterns, exactSecrets := rr.loadMaskingConfig()

			if len(regexPatterns) != tc.wantRegexCount {
				t.Errorf("got %d regex patterns, want %d: %v", len(regexPatterns), tc.wantRegexCount, regexPatterns)
			}
			if len(exactSecrets) != tc.wantSecretsCount {
				t.Errorf("got %d secrets, want %d: %v", len(exactSecrets), tc.wantSecretsCount, exactSecrets)
			}
			if tc.wantFirstRegex != "" && len(regexPatterns) > 0 {
				if !strings.Contains(regexPatterns[0], tc.wantFirstRegex) {
					t.Errorf("first regex = %q, want %q", regexPatterns[0], tc.wantFirstRegex)
				}
			}
			if tc.wantFirstSecret != "" && len(exactSecrets) > 0 {
				if exactSecrets[0] != tc.wantFirstSecret {
					t.Errorf("first secret = %q, want %q", exactSecrets[0], tc.wantFirstSecret)
				}
			}
		})
	}
}
