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

package scmclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitLab_GetFileContent(t *testing.T) {
	fileContent := []byte("apiVersion: tekton.dev/v1\nkind: Task\n")
	encoded := base64.StdEncoding.EncodeToString(fileContent)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/api/v4/projects/test_org%2Ftest_repo/repository/files/tasks%2Fbuild.yaml"
		if r.URL.RawPath != expectedPath {
			t.Errorf("unexpected path: %q, want %q", r.URL.RawPath, expectedPath)
		}
		if r.URL.Query().Get("ref") != "main" {
			t.Errorf("unexpected ref: %s", r.URL.Query().Get("ref"))
		}
		if r.Header.Get("Private-Token") != "test_token" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Private-Token"))
		}
		json.NewEncoder(w).Encode(map[string]string{"content": encoded})
	}))
	defer server.Close()

	client := newGitLabClient(server.URL, "test_token")
	got, err := client.GetFileContent(context.Background(), "test_org", "test_repo", "tasks/build.yaml", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(fileContent) {
		t.Errorf("got %q, want %q", got, fileContent)
	}
}

func TestGitLab_GetFileContent_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message": "404 File Not Found"}`))
	}))
	defer server.Close()

	client := newGitLabClient(server.URL, "test_token")
	_, err := client.GetFileContent(context.Background(), "test_org", "test_repo", "missing.yaml", "main")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestGitLab_GetCommitSHA(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/api/v4/projects/test_org%2Ftest_repo/repository/commits/main"
		if r.URL.RawPath != expectedPath {
			t.Errorf("unexpected path: %q, want %q", r.URL.RawPath, expectedPath)
		}
		json.NewEncoder(w).Encode(map[string]string{
			"id": "abc123def456abc123def456abc123def456abc1",
		})
	}))
	defer server.Close()

	client := newGitLabClient(server.URL, "test_token")
	got, err := client.GetCommitSHA(context.Background(), "test_org", "test_repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "abc123def456abc123def456abc123def456abc1" {
		t.Errorf("got %q, want %q", got, "abc123def456abc123def456abc123def456abc1")
	}
}

func TestGitLab_GetCommitSHA_EmptyID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"id": ""})
	}))
	defer server.Close()

	client := newGitLabClient(server.URL, "test_token")
	_, err := client.GetCommitSHA(context.Background(), "test_org", "test_repo", "main")
	if err == nil {
		t.Fatal("expected error for empty id, got nil")
	}
}

func TestGitLab_GetCloneURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/api/v4/projects/test_org%2Ftest_repo"
		if r.URL.RawPath != expectedPath {
			t.Errorf("unexpected path: %q, want %q", r.URL.RawPath, expectedPath)
		}
		json.NewEncoder(w).Encode(map[string]string{
			"http_url_to_repo": "https://gitlab.com/test_org/test_repo.git",
		})
	}))
	defer server.Close()

	client := newGitLabClient(server.URL, "test_token")
	got, err := client.GetCloneURL(context.Background(), "test_org", "test_repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://gitlab.com/test_org/test_repo.git" {
		t.Errorf("got %q, want %q", got, "https://gitlab.com/test_org/test_repo.git")
	}
}

func TestGitLab_GetCloneURL_EmptyURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"http_url_to_repo": ""})
	}))
	defer server.Close()

	client := newGitLabClient(server.URL, "test_token")
	_, err := client.GetCloneURL(context.Background(), "test_org", "test_repo")
	if err == nil {
		t.Fatal("expected error for empty http_url_to_repo, got nil")
	}
}

func TestGitLab_encodeProjectID(t *testing.T) {
	tests := []struct {
		org  string
		repo string
		want string
	}{
		{"org", "repo", "org%2Frepo"},
		{"org", "repo.with.dots", "org%2Frepo.with.dots"},
	}
	for _, tt := range tests {
		got := encodeProjectID(tt.org, tt.repo)
		if got != tt.want {
			t.Errorf("encodeProjectID(%q, %q) = %q, want %q", tt.org, tt.repo, got, tt.want)
		}
	}
}
