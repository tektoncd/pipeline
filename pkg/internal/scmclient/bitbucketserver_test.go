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

func TestBitbucketServer_GetFileContent(t *testing.T) {
	fileContent := []byte("apiVersion: tekton.dev/v1\nkind: Task\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/1.0/projects/test_project/repos/test_repo/raw/tasks/build.yaml" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("at") != "main" {
			t.Errorf("unexpected ref: %s", r.URL.Query().Get("at"))
		}
		// Bearer token, project/repository level
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test_token" {
			t.Errorf("unexpected Authorization header: %s", authHeader)
		}
		w.Write(fileContent)
	}))
	defer server.Close()

	client := newBitbucketServerClient(server.URL, "test_token")
	got, err := client.GetFileContent(context.Background(), "test_project", "test_repo", "tasks/build.yaml", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(fileContent) {
		t.Errorf("got %q, want %q", got, fileContent)
	}
}

func TestBitbucketServer_GetFileContent_RawBytes(t *testing.T) {
	// Verify raw bytes returned directly - no base64 decoding
	rawContent := []byte("raw: content\nno: encoding\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(rawContent)
	}))
	defer server.Close()

	client := newBitbucketServerClient(server.URL, "test_token")
	got, err := client.GetFileContent(context.Background(), "test_project", "test_repo", "file.yaml", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(rawContent) {
		t.Errorf("got %q, want %q", got, rawContent)
	}
}

func TestBitbucketServer_GetFileContent_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"errors": [{"message": "Repository does not exist"}]}`))
	}))
	defer server.Close()

	client := newBitbucketServerClient(server.URL, "test_token")
	_, err := client.GetFileContent(context.Background(), "test_project", "test_repo", "missing.yaml", "main")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestBitbucketServer_GetCommitSHA(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/1.0/projects/test_project/repos/test_repo/commits/main" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		io.WriteString(w, `{"id": "abc123def456abc123def456abc123def456abc1"}`)
	}))
	defer server.Close()

	client := newBitbucketServerClient(server.URL, "test_token")
	got, err := client.GetCommitSHA(context.Background(), "test_project", "test_repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "abc123def456abc123def456abc123def456abc1" {
		t.Errorf("got %q, want %q", got, "abc123def456abc123def456abc123def456abc1")
	}
}

func TestBitbucketServer_GetCommitSHA_EmptyValues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"values": []}`)
	}))
	defer server.Close()

	client := newBitbucketServerClient(server.URL, "test_token")
	_, err := client.GetCommitSHA(context.Background(), "test_project", "test_repo", "main")
	if err == nil {
		t.Fatal("expected error for empty values, got nil")
	}
}

func TestBitbucketServer_GetCommitSHA_EmptyID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"id": ""}`)
	}))
	defer server.Close()

	client := newBitbucketServerClient(server.URL, "test_token")
	_, err := client.GetCommitSHA(context.Background(), "test_project", "test_repo", "main")
	if err == nil {
		t.Fatal("expected error for empty id, got nil")
	}
}

func TestBitbucketServer_GetCloneURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/1.0/projects/test_project/repos/test_repo" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		// Bitbucket Server uses "http" not "https" for the clone link name
		io.WriteString(w, `{
			"links": {
				"clone": [
					{"name": "ssh", "href": "ssh://git@bitbucketserver.example.com/test_proj/test_repo.git"},
					{"name": "http", "href": "https://bitbucketserver.example.com/scm/test_proj/test_repo.git"}
				]
			}
		}`)
	}))
	defer server.Close()

	client := newBitbucketServerClient(server.URL, "test_token")
	got, err := client.GetCloneURL(context.Background(), "test_project", "test_repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://bitbucketserver.example.com/scm/test_proj/test_repo.git" {
		t.Errorf("got %q, want %q", got, "https://bitbucketserver.example.com/scm/test_proj/test_repo.git")
	}
}

func TestBitbucketServer_GetCloneURL_NoHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only SSH link, no http
		io.WriteString(w, `{
			"links": {
				"clone": [
					{"name": "ssh", "href": "ssh://git@bitbucketserver.example.com/test_proj/test_repo.git"}
				]
			}
		}`)
	}))
	defer server.Close()

	client := newBitbucketServerClient(server.URL, "test_token")
	_, err := client.GetCloneURL(context.Background(), "test_project", "test_repo")
	if err == nil {
		t.Fatal("expected error when no http clone URL found, got nil")
	}
	if !strings.Contains(err.Error(), "no http clone URL found") {
		t.Errorf("unexpected error message: %v", err)
	}
}
