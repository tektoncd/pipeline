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

func TestGitea_GetFileContent(t *testing.T) {
	fileContent := []byte("apiVersion: tekton.dev/v1\nkind: Task\n")
	encoded := base64.StdEncoding.EncodeToString(fileContent)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/repos/test_org/test_repo/contents/tasks/build.yaml" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "token test_token" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		json.NewEncoder(w).Encode(map[string]string{
			"content": encoded,
		})
	}))
	defer server.Close()

	client := newGiteaClient(server.URL, "test_token")
	got, err := client.GetFileContent(context.Background(), "test_org", "test_repo", "tasks/build.yaml", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(fileContent) {
		t.Errorf("got %q, want %q", got, fileContent)
	}
}

func TestGitea_GetFileContent_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message": "Not Found"}`))
	}))
	defer server.Close()

	client := newGiteaClient(server.URL, "test_token")
	_, err := client.GetFileContent(context.Background(), "test_org", "test_repo", "missing.yaml", "main")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestGitea_GetCommitSHA(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/repos/test_org/test_repo/git/commits/main" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]string{
			"sha": "abc123def456abc123def456abc123def456abc1",
		})
	}))
	defer server.Close()

	client := newGiteaClient(server.URL, "test_token")
	got, err := client.GetCommitSHA(context.Background(), "test_org", "test_repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "abc123def456abc123def456abc123def456abc1" {
		t.Errorf("got %q, want %q", got, "abc123def456abc123def456abc123def456abc1")
	}
}

func TestGitea_GetCommitSHA_EmptySHA(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"sha": ""})
	}))
	defer server.Close()

	client := newGiteaClient(server.URL, "test_token")
	_, err := client.GetCommitSHA(context.Background(), "test_org", "test_repo", "main")
	if err == nil {
		t.Fatal("expected error for empty sha, got nil")
	}
}

func TestGitea_GetCloneURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/repos/test_org/test_repo" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]string{
			"clone_url": "http://gitea.example.com/test_org/test_repo.git",
		})
	}))
	defer server.Close()

	client := newGiteaClient(server.URL, "test_token")
	got, err := client.GetCloneURL(context.Background(), "test_org", "test_repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "http://gitea.example.com/test_org/test_repo.git" {
		t.Errorf("got %q, want %q", got, "http://gitea.example.com/test_org/test_repo.git")
	}
}

func TestGitea_GetCloneURL_EmptyURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"clone_url": ""})
	}))
	defer server.Close()

	client := newGiteaClient(server.URL, "test_token")
	_, err := client.GetCloneURL(context.Background(), "test_org", "test_repo")
	if err == nil {
		t.Fatal("expected error for empty clone_url, got nil")
	}
}
