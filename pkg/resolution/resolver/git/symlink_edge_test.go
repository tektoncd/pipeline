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

package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func cloneSymlinkFixture(t *testing.T, source string, partial bool) *repository {
	t.Helper()
	ctx := t.Context()
	if out, err := exec.CommandContext(ctx, "git", "-C", source, "config", "uploadpack.allowFilter", "true").CombinedOutput(); err != nil {
		t.Fatalf("enable filtering: %s: %v", out, err)
	}
	repo := &repository{
		url:        "file://" + source,
		pathInRepo: "links/resource.yaml",
		directory:  t.TempDir(),
	}
	var err error
	if partial {
		err = repo.clonePartial(ctx)
	} else {
		err = repo.cloneFull(ctx)
	}
	if err != nil {
		t.Fatalf("clone (partial=%v): %v", partial, err)
	}
	if err := repo.checkout(ctx, "main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if partial {
		if _, err := os.Stat(filepath.Join(repo.directory, "targets")); !os.IsNotExist(err) {
			t.Fatalf("expected targets outside sparse checkout, got %v", err)
		}
	}
	return repo
}

// Compare the object walker with the existing full-clone filesystem path.
// Distinct decoy content makes normalization bugs fail on content, not just errors.
func TestGetFileContent_SymlinkParity(t *testing.T) {
	cases := []struct {
		name    string
		target  string
		want    string
		wantErr bool
	}{
		{name: "trailing space", target: "../targets/task.yaml ", want: "trailing-space"},
		{name: "literal backslash", target: `../targets/task\name.yaml`, want: "backslash"},
		{name: "pathspec syntax exists literally", target: "../:(top)literal/task.yaml", want: "literal-magic"},
		{name: "repeated separators and dot", target: "../targets//./task.yaml", want: "ordinary"},
		{name: "missing before dotdot", target: "../targets/missing/../task.yaml", wantErr: true},
		{name: "file before dotdot", target: "../targets/task.yaml/../task.yaml", wantErr: true},
		{name: "file trailing slash", target: "../targets/task.yaml/", wantErr: true},
		{name: "directory target", target: "../targets", wantErr: true},
		{name: "missing absolute target", target: "/nonexistent-tekton-symlink-target/task.yaml", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source, _ := createTestRepo(t, []commitForRepo{
				{Dir: "targets", Filename: "task.yaml", Content: "ordinary"},
				{Dir: "targets", Filename: "task.yaml ", Content: "trailing-space"},
				{Dir: "targets", Filename: `task\name.yaml`, Content: "backslash"},
				{Dir: ":(top)literal", Filename: "task.yaml", Content: "literal-magic"},
				{Dir: "literal", Filename: "task.yaml", Content: "wrong-magic-match"},
			})
			symlinkAndCommit(t, source, "links/resource.yaml", tc.target)
			for _, partial := range []bool{false, true} {
				t.Run(fmt.Sprintf("partial=%v", partial), func(t *testing.T) {
					repo := cloneSymlinkFixture(t, source, partial)
					content, err := repo.getFileContent(t.Context(), "links/resource.yaml")
					if tc.wantErr {
						if err == nil || len(content) != 0 {
							t.Fatalf("expected error without content, got %q, %v", content, err)
						}
					} else if err != nil || string(content) != tc.want {
						t.Fatalf("want %q, got %q, %v", tc.want, content, err)
					}
				})
			}
		})
	}
}

func TestGetFileContent_SymlinkHopBoundary(t *testing.T) {
	for _, hops := range []int{11, 40, 41} {
		t.Run(fmt.Sprintf("hops=%d", hops), func(t *testing.T) {
			source, _ := createTestRepo(t, []commitForRepo{
				{Dir: "targets", Filename: "task.yaml", Content: "chain-result"},
			})
			// Commit the whole chain together; every intermediate link lies
			// outside the selected links/ directory.
			for i := 1; i < hops; i++ {
				target := "task.yaml"
				if i < hops-1 {
					target = fmt.Sprintf("hop-%d", i+1)
				}
				if err := os.Symlink(target, filepath.Join(source, "targets", fmt.Sprintf("hop-%d", i))); err != nil {
					t.Fatal(err)
				}
			}
			gitCmd := getGitCmd(t, source)
			if out, err := gitCmd("add", "targets").CombinedOutput(); err != nil {
				t.Fatalf("add chain: %s: %v", out, err)
			}
			symlinkAndCommit(t, source, "links/resource.yaml", "../targets/hop-1")
			repo := cloneSymlinkFixture(t, source, true)
			content, err := repo.getFileContent(t.Context(), "links/resource.yaml")
			if hops > maxSymlinkHops {
				if err == nil || !strings.Contains(err.Error(), "too many levels of symlinks") || len(content) != 0 {
					t.Fatalf("expected hop-limit error without content, got %q, %v", content, err)
				}
				return
			}
			if err != nil || string(content) != "chain-result" {
				t.Fatalf("expected valid chain, got %q, %v", content, err)
			}
			full := cloneSymlinkFixture(t, source, false)
			fullContent, err := full.getFileContent(t.Context(), "links/resource.yaml")
			if err != nil || string(fullContent) != string(content) {
				t.Fatalf("full-clone parity failed: %q, %v", fullContent, err)
			}
		})
	}
}

// Invalid blobs cannot all be represented by os.Symlink, so exercise the
// validation boundary directly, including the exact size limit.
func TestSymlinkTargetComponents_Validation(t *testing.T) {
	cases := []struct {
		name    string
		target  []byte
		wantErr bool
	}{
		{name: "empty", target: nil, wantErr: true},
		{name: "NUL", target: []byte("task\x00.yaml"), wantErr: true},
		{name: "absolute", target: []byte("/task.yaml"), wantErr: true},
		{name: "oversized", target: []byte(strings.Repeat("x", maxSymlinkTargetLength+1)), wantErr: true},
		{name: "size boundary", target: []byte(strings.Repeat("x", maxSymlinkTargetLength))},
		{name: "preserve bytes", target: []byte("../targets/task\\name.yaml ")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			components, err := symlinkTargetComponents("links/resource.yaml", tc.target)
			if (err != nil) != tc.wantErr {
				t.Fatalf("wantErr=%v, got %v", tc.wantErr, err)
			}
			if err == nil && strings.Join(components, "/") != string(tc.target) {
				t.Fatalf("target bytes changed: %q", components)
			}
		})
	}
}

func TestGetFileContent_SymlinkObjectFailures(t *testing.T) {
	for _, failure := range []string{"canceled context", "unavailable remote"} {
		t.Run(failure, func(t *testing.T) {
			source, _ := createTestRepo(t, []commitForRepo{
				{Dir: "targets", Filename: "task.yaml", Content: "must-not-be-returned"},
			})
			symlinkAndCommit(t, source, "links/resource.yaml", "../targets/task.yaml")
			repo := cloneSymlinkFixture(t, source, true)
			ctx := t.Context()
			if failure == "canceled context" {
				canceled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = canceled
			} else {
				missingRemote := "file://" + filepath.Join(t.TempDir(), "missing.git")
				if _, err := repo.execGit(ctx, "remote", "set-url", "origin", missingRemote); err != nil {
					t.Fatal(err)
				}
			}
			content, err := repo.getFileContent(ctx, "links/resource.yaml")
			if err == nil || len(content) != 0 {
				t.Fatalf("expected error without content, got %q, %v", content, err)
			}
			if failure == "canceled context" && !errors.Is(err, context.Canceled) {
				t.Fatalf("expected context cancellation, got %v", err)
			}
			if failure == "unavailable remote" && !strings.Contains(err.Error(), "git cat-file error") {
				t.Fatalf("expected lazy blob-fetch failure, got %v", err)
			}
		})
	}
}
