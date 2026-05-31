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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBitbucketCloud_GetFileContent_BasicAuth(t *testing.T) {
	fileContent := []byte("apiVersion: tekton.dev/v1\nkind: Task\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/repositories/test_org/test_repo/src/main/tasks/build.yaml" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		// Basic auth — username:api_token format
		username, password, ok := r.BasicAuth()
		if !ok {
			t.Error("expected Basic auth, got none")
		}
		if username != "test_user" {
			t.Errorf("unexpected username: %s", username)
		}
		if password != "test_apitoken" {
			t.Errorf("unexpected password: %s", password)
		}
		w.Write(fileContent)
	}))
	defer server.Close()

	client := newBitbucketCloudClient(server.URL, "test_user:test_apitoken")
	got, err := client.GetFileContent(context.Background(), "test_org", "test_repo", "tasks/build.yaml", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(fileContent) {
		t.Errorf("got %q, want %q", got, fileContent)
	}
}

func TestBitbucketCloud_GetFileContent_BearerAuth(t *testing.T) {
	fileContent := []byte("apiVersion: tekton.dev/v1\nkind: Task\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bearer auth — bare token, no username
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test_repotoken" {
			t.Errorf("unexpected Authorization header: %s", authHeader)
		}
		_, _, ok := r.BasicAuth()
		if ok {
			t.Error("expected Bearer auth, got Basic auth")
		}
		w.Write(fileContent)
	}))
	defer server.Close()

	client := newBitbucketCloudClient(server.URL, "test_repotoken")
	got, err := client.GetFileContent(context.Background(), "test_org", "test_repo", "tasks/build.yaml", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(fileContent) {
		t.Errorf("got %q, want %q", got, fileContent)
	}
}

func TestBitbucketCloud_GetFileContent_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"type": "error", "error": {"message": "Not Found"}}`))
	}))
	defer server.Close()

	client := newBitbucketCloudClient(server.URL, "test_user:test_apitoken")
	_, err := client.GetFileContent(context.Background(), "test_org", "test_repo", "missing.yaml", "main")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestBitbucketCloud_GetFileContent_RawBytes(t *testing.T) {
	// Verify raw bytes are returned directly — no base64 decoding
	rawContent := []byte("raw: content\nno: encoding\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(rawContent)
	}))
	defer server.Close()

	client := newBitbucketCloudClient(server.URL, "test_user:test_apitoken")
	got, err := client.GetFileContent(context.Background(), "test_org", "test_repo", "file.yaml", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(rawContent) {
		t.Errorf("got %q, want %q", got, rawContent)
	}
}

func TestBitbucketCloud_GetCommitSHA(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/repositories/test_org/test_repo/commit/main" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		// Bitbucket Cloud uses "hash" not "sha"
		io.WriteString(w, `{"hash": "abc123def456abc123def456abc123def456abc1"}`)
	}))
	defer server.Close()

	client := newBitbucketCloudClient(server.URL, "test_user:test_apitoken")
	got, err := client.GetCommitSHA(context.Background(), "test_org", "test_repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "abc123def456abc123def456abc123def456abc1" {
		t.Errorf("got %q, want %q", got, "abc123def456abc123def456abc123def456abc1")
	}
}

func TestBitbucketCloud_GetCommitSHA_EmptyHash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"hash": ""}`)
	}))
	defer server.Close()

	client := newBitbucketCloudClient(server.URL, "test_user:test_apitoken")
	_, err := client.GetCommitSHA(context.Background(), "test_org", "test_repo", "main")
	if err == nil {
		t.Fatal("expected error for empty hash, got nil")
	}
}

func TestBitbucketCloud_GetCloneURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/repositories/test_org/test_repo" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		io.WriteString(w, `{
			"links": {
				"clone": [
					{"name": "ssh", "href": "git@bitbucket.org:test_org/test_repo.git"},
					{"name": "https", "href": "https://test_user@bitbucket.org/test_org/test_repo.git"}
				]
			}
		}`)
	}))
	defer server.Close()

	client := newBitbucketCloudClient(server.URL, "test_user:test_apitoken")
	got, err := client.GetCloneURL(context.Background(), "test_org", "test_repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://test_user@bitbucket.org/test_org/test_repo.git" {
		t.Errorf("got %q, want %q", got, "https://test_user@bitbucket.org/test_org/test_repo.git")
	}
}

func TestBitbucketCloud_GetCloneURL_NoHTTPS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only SSH link, no HTTPS
		io.WriteString(w, `{
			"links": {
				"clone": [
					{"name": "ssh", "href": "git@bitbucket.org:test_org/test_repo.git"}
				]
			}
		}`)
	}))
	defer server.Close()

	client := newBitbucketCloudClient(server.URL, "test_user:test_apitoken")
	_, err := client.GetCloneURL(context.Background(), "test_org", "test_repo")
	if err == nil {
		t.Fatal("expected error when no https clone URL found, got nil")
	}
	if !strings.Contains(err.Error(), "no https clone URL found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestBitbucketCloud_splitUsernamePassword(t *testing.T) {
	tests := []struct {
		token        string
		wantUsername string
		wantPassword string
	}{
		{"test_user:test_password", "test_user", "test_password"},
		{"test_user:pass:with:colons", "test_user", "pass:with:colons"},
		{"baretoken", "", "baretoken"},
		{":emptyusername", "", "emptyusername"},
	}
	for _, tt := range tests {
		gotUsername, gotPassword := splitUsernamePassword(tt.token)
		if gotUsername != tt.wantUsername {
			t.Errorf("splitUsernamePassword(%q) username = %q, want %q", tt.token, gotUsername, tt.wantUsername)
		}
		if gotPassword != tt.wantPassword {
			t.Errorf("splitUsernamePassword(%q) password = %q, want %q", tt.token, gotPassword, tt.wantPassword)
		}
	}
}
