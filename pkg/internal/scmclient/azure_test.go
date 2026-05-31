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
	"testing"
)

func TestAzure_GetFileContent(t *testing.T) {
	fileContent := []byte("apiVersion: tekton.dev/v1\nkind: Task\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/test_org/test_project/_apis/git/repositories/test_repo/items"
		if r.URL.Path != expectedPath {
			t.Errorf("unexpected path: %s, want %s", r.URL.Path, expectedPath)
		}
		if r.URL.Query().Get("path") != "tasks/build.yaml" {
			t.Errorf("unexpected path param: %s", r.URL.Query().Get("path"))
		}
		if r.URL.Query().Get("versionDescriptor.version") != "main" {
			t.Errorf("unexpected version: %s", r.URL.Query().Get("versionDescriptor.version"))
		}
		if r.URL.Query().Get("versionDescriptor.versionType") != "branch" {
			t.Errorf("unexpected versionType: %s", r.URL.Query().Get("versionDescriptor.versionType"))
		}
		// Azure PAT: Basic auth with empty username
		username, password, ok := r.BasicAuth()
		if !ok {
			t.Error("expected Basic auth, got none")
		}
		if username != "" {
			t.Errorf("expected empty username, got %q", username)
		}
		if password != "test_token" {
			t.Errorf("unexpected token: %s", password)
		}
		if r.Header.Get("Accept") != "text/plain" {
			t.Errorf("unexpected Accept header: %s", r.Header.Get("Accept"))
		}
		w.Write(fileContent)
	}))
	defer server.Close()

	// org is "org/project", repo is separate
	client := newAzureClient(server.URL, "test_token")
	got, err := client.GetFileContent(context.Background(), "test_org/test_project", "test_repo", "tasks/build.yaml", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(fileContent) {
		t.Errorf("got %q, want %q", got, fileContent)
	}
}

func TestAzure_GetFileContent_RefsHeads(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("versionDescriptor.version") != "main" {
			t.Errorf("unexpected version: %s", r.URL.Query().Get("versionDescriptor.version"))
		}
		if r.URL.Query().Get("versionDescriptor.versionType") != "branch" {
			t.Errorf("unexpected versionType: %s", r.URL.Query().Get("versionDescriptor.versionType"))
		}
		w.Write([]byte("content"))
	}))
	defer server.Close()

	client := newAzureClient(server.URL, "test_token")
	_, err := client.GetFileContent(context.Background(), "test_org/test_project", "test_repo", "file.yaml", "refs/heads/main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAzure_GetFileContent_RefsTags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("versionDescriptor.version") != "v1.0.0" {
			t.Errorf("unexpected version: %s", r.URL.Query().Get("versionDescriptor.version"))
		}
		if r.URL.Query().Get("versionDescriptor.versionType") != "tag" {
			t.Errorf("unexpected versionType: %s", r.URL.Query().Get("versionDescriptor.versionType"))
		}
		w.Write([]byte("content"))
	}))
	defer server.Close()

	client := newAzureClient(server.URL, "test_token")
	_, err := client.GetFileContent(context.Background(), "test_org/test_project", "test_repo", "file.yaml", "refs/tags/v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAzure_GetFileContent_CommitSHA(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("versionDescriptor.versionType") != "commit" {
			t.Errorf("unexpected versionType: %s", r.URL.Query().Get("versionDescriptor.versionType"))
		}
		w.Write([]byte("content"))
	}))
	defer server.Close()

	client := newAzureClient(server.URL, "test_token")
	_, err := client.GetFileContent(context.Background(), "test_org/test_project", "test_repo", "file.yaml", "abc123def456abc123def456abc123def456abc1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAzure_GetFileContent_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message": "Not Found"}`))
	}))
	defer server.Close()

	client := newAzureClient(server.URL, "test_token")
	_, err := client.GetFileContent(context.Background(), "test_org/test_project", "test_repo", "missing.yaml", "main")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestAzure_GetFileContent_InvalidOrgFormat(t *testing.T) {
	client := newAzureClient("https://dev.azure.com", "test_token")
	_, err := client.GetFileContent(context.Background(), "onlyorg", "test_repo", "file.yaml", "main")
	if err == nil {
		t.Fatal("expected error for invalid org format, got nil")
	}
}

func TestAzure_GetCommitSHA(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/test_org/test_project/_apis/git/repositories/test_repo/commits/main"
		if r.URL.Path != expectedPath {
			t.Errorf("unexpected path: %s, want %s", r.URL.Path, expectedPath)
		}
		if r.URL.Query().Get("api-version") != "7.1" {
			t.Errorf("unexpected api-version: %s", r.URL.Query().Get("api-version"))
		}
		io.WriteString(w, `{"commitId": "abc123def456abc123def456abc123def456abc1"}`)
	}))
	defer server.Close()

	client := newAzureClient(server.URL, "test_token")
	got, err := client.GetCommitSHA(context.Background(), "test_org/test_project", "test_repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "abc123def456abc123def456abc123def456abc1" {
		t.Errorf("got %q, want %q", got, "abc123def456abc123def456abc123def456abc1")
	}
}

func TestAzure_GetCommitSHA_EmptyCommitID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"commitId": ""}`)
	}))
	defer server.Close()

	client := newAzureClient(server.URL, "test_token")
	_, err := client.GetCommitSHA(context.Background(), "test_org/test_project", "test_repo", "main")
	if err == nil {
		t.Fatal("expected error for empty commitId, got nil")
	}
}

func TestAzure_GetCloneURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/test_org/test_project/_apis/git/repositories/test_repo"
		if r.URL.Path != expectedPath {
			t.Errorf("unexpected path: %s, want %s", r.URL.Path, expectedPath)
		}
		if r.URL.Query().Get("api-version") != "7.1" {
			t.Errorf("unexpected api-version: %s", r.URL.Query().Get("api-version"))
		}
		io.WriteString(w, `{"remoteUrl": "https://test_org@dev.azure.com/test_org/test_project/_git/test_repo"}`)
	}))
	defer server.Close()

	client := newAzureClient(server.URL, "test_token")
	got, err := client.GetCloneURL(context.Background(), "test_org/test_project", "test_repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://test_org@dev.azure.com/test_org/test_project/_git/test_repo" {
		t.Errorf("got %q, want %q", got, "https://test_org@dev.azure.com/test_org/test_project/_git/test_repo")
	}
}

func TestAzure_GetCloneURL_EmptyRemoteURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"remoteUrl": ""}`)
	}))
	defer server.Close()

	client := newAzureClient(server.URL, "test_token")
	_, err := client.GetCloneURL(context.Background(), "test_org/test_project", "test_repo")
	if err == nil {
		t.Fatal("expected error for empty remoteUrl, got nil")
	}
}

func TestAzure_decodeAzureOrgProject(t *testing.T) {
	tests := []struct {
		org      string
		wantOrg  string
		wantProj string
		wantErr  bool
	}{
		{"test_org/test_project", "test_org", "test_project", false},
		{"test_org", "", "", true},
		{"test_org/", "", "", true},
		{"/test_project", "", "", true},
		{"", "", "", true},
	}
	for _, tt := range tests {
		org, proj, err := decodeAzureOrgProject(tt.org)
		if tt.wantErr {
			if err == nil {
				t.Errorf("decodeAzureOrgProject(%q) expected error, got nil", tt.org)
			}
			continue
		}
		if err != nil {
			t.Errorf("decodeAzureOrgProject(%q) unexpected error: %v", tt.org, err)
			continue
		}
		if org != tt.wantOrg || proj != tt.wantProj {
			t.Errorf("decodeAzureOrgProject(%q) = (%q, %q), want (%q, %q)",
				tt.org, org, proj, tt.wantOrg, tt.wantProj)
		}
	}
}

func TestAzure_generateVersionDescriptor(t *testing.T) {
	tests := []struct {
		ref  string
		want string
	}{
		{"refs/heads/main", "versionDescriptor.version=main&versionDescriptor.versionType=branch"},
		{"refs/tags/v1.0.0", "versionDescriptor.version=v1.0.0&versionDescriptor.versionType=tag"},
		{"abc123def456abc123def456abc123def456abc1", "versionDescriptor.version=abc123def456abc123def456abc123def456abc1&versionDescriptor.versionType=commit"},
		{"main", "versionDescriptor.version=main&versionDescriptor.versionType=branch"},
	}
	for _, tt := range tests {
		got := generateVersionDescriptor(tt.ref)
		if got != tt.want {
			t.Errorf("generateVersionDescriptor(%q) = %q, want %q", tt.ref, got, tt.want)
		}
	}
}
