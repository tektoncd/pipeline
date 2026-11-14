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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// symlinkAndCommit adds a committed symlink at linkPath (relative to
// repoDir) pointing at target, exactly as createTestRepo's
// writeAndCommitToTestRepo commits a regular file. createTestRepo has no
// symlink support of its own, since os.Symlink -- unlike os.WriteFile --
// needs no content to write, only the two paths.
func symlinkAndCommit(t *testing.T, repoDir, linkPath, target string) {
	t.Helper()
	full := filepath.Join(repoDir, linkPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for symlink %q: %v", linkPath, err)
	}
	if err := os.Symlink(target, full); err != nil {
		t.Fatalf("creating symlink %q -> %q: %v", linkPath, target, err)
	}
	gitCmd := getGitCmd(t, repoDir)
	if out, err := gitCmd("add", "--", linkPath).Output(); err != nil {
		t.Fatalf("git add symlink %q: %q: %v", linkPath, out, err)
	}
	if out, err := gitCmd("commit", "-m", "add symlink "+linkPath).Output(); err != nil {
		t.Fatalf("git commit symlink %q: %q: %v", linkPath, out, err)
	}
}

// countObjectsInPack reports the pack's cumulative in-pack object count
// for repo.directory, used to confirm resolution fetched only the blobs
// on the symlink chain and not, say, every small blob in the repository.
func countObjectsInPack(t *testing.T, dir string) int {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), "git", "-C", dir, "count-objects", "-v").Output()
	if err != nil {
		t.Fatalf("git count-objects: %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if n, ok := strings.CutPrefix(line, "in-pack: "); ok {
			total := 0
			for _, c := range n {
				if c < '0' || c > '9' {
					break
				}
				total = total*10 + int(c-'0')
			}
			return total
		}
	}
	t.Fatalf("no 'in-pack:' line in git count-objects output: %q", out)
	return 0
}

// TestGetFileContent_SymlinkOutsideSparseCone is the exact scenario from
// the PR #10665 review: pathInRepo names a symlink whose target lies
// outside the directory the sparse checkout selected, so the target was
// never materialised and filepath.EvalSymlinks sees only a dangling link.
func TestGetFileContent_SymlinkOutsideSparseCone(t *testing.T) {
	repoDir, _ := createTestRepo(t, []commitForRepo{
		{Dir: "targets", Filename: "pipeline.yaml", Content: "resolved-through-git-objects"},
		{Dir: "decoy", Filename: "unrelated.yaml", Content: "should never be fetched"},
	})
	symlinkAndCommit(t, repoDir, "links/pipeline.yaml", "../targets/pipeline.yaml")

	ctx := t.Context()
	repo, cleanup, err := remote{url: "file://" + repoDir, pathInRepo: "links/pipeline.yaml"}.clone(ctx)
	defer cleanup()
	if err != nil {
		t.Fatalf("failed to clone: %v", err)
	}
	if !repo.partial {
		t.Fatal("expected the file:// clone to take the partial path")
	}
	if err := repo.checkout(ctx, "main"); err != nil {
		t.Fatalf("failed to checkout: %v", err)
	}

	// The symlink itself is present but dangling: its target directory
	// was never pulled into the sparse cone.
	if _, err := os.Lstat(filepath.Join(repo.directory, "links", "pipeline.yaml")); err != nil {
		t.Fatalf("expected the symlink itself to be checked out: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo.directory, "targets")); !os.IsNotExist(err) {
		t.Fatalf("expected 'targets' to be absent from the working tree before resolution, stat returned: %v", err)
	}

	content, err := repo.getFileContent(ctx, "links/pipeline.yaml")
	if err != nil {
		t.Fatalf("expected the dangling symlink to resolve via git objects, got error: %v", err)
	}
	if string(content) != "resolved-through-git-objects" {
		t.Fatalf("expected %q, got %q", "resolved-through-git-objects", string(content))
	}
}

// TestGetFileContent_ChainedSymlinksOutsideSparseCone extends the review's
// scenario to a two-hop chain, each hop landing in a different directory.
func TestGetFileContent_ChainedSymlinksOutsideSparseCone(t *testing.T) {
	repoDir, _ := createTestRepo(t, []commitForRepo{
		{Dir: "shared", Filename: "base.yaml", Content: "final-content"},
		{Dir: "decoy", Filename: "unrelated.yaml", Content: "should never be fetched"},
	})
	symlinkAndCommit(t, repoDir, "deep/nested/mid.yaml", "../../shared/base.yaml")
	symlinkAndCommit(t, repoDir, "pipelines/build.yaml", "../deep/nested/mid.yaml")

	// createTestRepo's plain (non-bare) repository otherwise answers
	// "warning: filtering not recognized by server, ignoring" to a
	// blob:none clone and serves every blob anyway: repo.partial would
	// still end up true (sparse-checkout itself succeeds regardless),
	// but the object-count assertion below would hold trivially, true
	// even if resolution fetched nothing new because everything had
	// already arrived at clone time. Enabling the capability on the
	// server side is what actually makes --filter=blob:none deferral
	// happen, which is the behaviour this test means to verify.
	if out, err := exec.CommandContext(t.Context(), "git", "-C", repoDir, "config", "uploadpack.allowFilter", "true").CombinedOutput(); err != nil {
		t.Fatalf("git config uploadpack.allowFilter: %q: %v", out, err)
	}

	ctx := t.Context()
	repo, cleanup, err := remote{url: "file://" + repoDir, pathInRepo: "pipelines/build.yaml"}.clone(ctx)
	defer cleanup()
	if err != nil {
		t.Fatalf("failed to clone: %v", err)
	}
	if !repo.partial {
		t.Fatal("expected the file:// clone to take the partial path")
	}
	if err := repo.checkout(ctx, "main"); err != nil {
		t.Fatalf("failed to checkout: %v", err)
	}

	before := countObjectsInPack(t, repo.directory)
	content, err := repo.getFileContent(ctx, "pipelines/build.yaml")
	if err != nil {
		t.Fatalf("expected the chained symlink to resolve via git objects, got error: %v", err)
	}
	if string(content) != "final-content" {
		t.Fatalf("expected %q, got %q", "final-content", string(content))
	}
	after := countObjectsInPack(t, repo.directory)

	// The first symlink is already checked out. Only the second symlink
	// and final target should be fetched; zero would mean filtering was
	// ineffective, and three would include the unrelated decoy.
	if got := after - before; got != 2 {
		t.Fatalf("expected exactly 2 additional objects fetched resolving a 2-hop chain, got %d", got)
	}
}

// TestGetFileContent_DirectorySymlinkOutsideSparseCone covers the case the
// review explicitly flagged as needing separate handling: an intermediate
// path *component* -- not just the leaf -- is a symlink to a directory,
// so `git ls-tree HEAD -- links/pipeline.yaml` finds nothing at all (the
// literal tree has no "links" subtree, only a "links" symlink blob).
func TestGetFileContent_DirectorySymlinkOutsideSparseCone(t *testing.T) {
	repoDir, _ := createTestRepo(t, []commitForRepo{
		{Dir: "real-shared", Filename: "pipeline.yaml", Content: "via-dirlink"},
	})
	symlinkAndCommit(t, repoDir, "links", "real-shared")

	ctx := t.Context()
	repo, cleanup, err := remote{url: "file://" + repoDir, pathInRepo: "links/pipeline.yaml"}.clone(ctx)
	defer cleanup()
	if err != nil {
		t.Fatalf("failed to clone: %v", err)
	}
	if !repo.partial {
		t.Fatal("expected the file:// clone to take the partial path")
	}
	if err := repo.checkout(ctx, "main"); err != nil {
		t.Fatalf("failed to checkout: %v", err)
	}

	content, err := repo.getFileContent(ctx, "links/pipeline.yaml")
	if err != nil {
		t.Fatalf("expected a directory symlink intermediate component to resolve, got error: %v", err)
	}
	if string(content) != "via-dirlink" {
		t.Fatalf("expected %q, got %q", "via-dirlink", string(content))
	}
}

// TestGetFileContent_SymlinkEscape_PartialClone mirrors
// TestGetFileContent_SymlinkEscape_RealGitRepo, but specifically exercises
// resolveViaGitObjects' own escape check, rather than
// the pre-existing filesystem-based one. The target is deliberately a
// path that doesn't exist anywhere on the test machine: a target like
// "../../../../../../etc/passwd" would make filepath.EvalSymlinks succeed
// (since /etc/passwd is a real file), taking the *old* code path and its
// *different* error message -- which would make this test pass for the
// wrong reason, without ever reaching the new partial-clone resolution
// this file adds.
func TestGetFileContent_SymlinkEscape_PartialClone(t *testing.T) {
	repoDir, _ := createTestRepo(t, []commitForRepo{
		{Dir: "links", Filename: "placeholder.yaml", Content: "placeholder"},
	})
	symlinkAndCommit(t, repoDir, "links/escape.yaml", "../../../../../../nonexistent-outside-the-repo")

	ctx := t.Context()
	repo, cleanup, err := remote{url: "file://" + repoDir, pathInRepo: "links/escape.yaml"}.clone(ctx)
	defer cleanup()
	if err != nil {
		t.Fatalf("failed to clone: %v", err)
	}
	if !repo.partial {
		t.Fatal("expected the file:// clone to take the partial path")
	}
	if err := repo.checkout(ctx, "main"); err != nil {
		t.Fatalf("failed to checkout: %v", err)
	}

	content, err := repo.getFileContent(ctx, "links/escape.yaml")
	if err == nil {
		t.Fatalf("expected the escaping symlink to be rejected, got content: %q", string(content))
	}
	if !strings.Contains(err.Error(), "climbs above the repository root") {
		t.Fatalf("expected a 'climbs above the repository root' error, got: %v", err)
	}
}

// TestGetFileContent_SymlinkCycle_PartialClone verifies a symlink cycle
// fails with a bounded error instead of looping forever. The two symlinks
// are deliberately split across two directories, only one of which is in
// the sparse cone: if both were in the same (checked-out) directory, the
// filesystem's own filepath.EvalSymlinks would detect the cycle itself
// (ELOOP, not ENOENT) before ever reaching resolveViaGitObjects, and this
// test would not be exercising the code path it's meant to.
func TestGetFileContent_SymlinkCycle_PartialClone(t *testing.T) {
	repoDir, _ := createTestRepo(t, []commitForRepo{
		{Dir: "pipelines", Filename: "placeholder.yaml", Content: "keep pipelines/ non-empty"},
		{Dir: "other", Filename: "placeholder.yaml", Content: "keep other/ non-empty"},
	})
	symlinkAndCommit(t, repoDir, "pipelines/a.yaml", "../other/b.yaml")
	symlinkAndCommit(t, repoDir, "other/b.yaml", "../pipelines/a.yaml")

	ctx := t.Context()
	repo, cleanup, err := remote{url: "file://" + repoDir, pathInRepo: "pipelines/a.yaml"}.clone(ctx)
	defer cleanup()
	if err != nil {
		t.Fatalf("failed to clone: %v", err)
	}
	if !repo.partial {
		t.Fatal("expected the file:// clone to take the partial path")
	}
	if err := repo.checkout(ctx, "main"); err != nil {
		t.Fatalf("failed to checkout: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo.directory, "other")); !os.IsNotExist(err) {
		t.Fatalf("expected 'other' to be outside the sparse cone, stat returned: %v", err)
	}

	_, err = repo.getFileContent(ctx, "pipelines/a.yaml")
	if err == nil {
		t.Fatal("expected a symlink cycle to be rejected, got nil error")
	}
	if !strings.Contains(err.Error(), "too many levels of symlinks") {
		t.Fatalf("expected a 'too many levels of symlinks' error, got: %v", err)
	}
}

// TestGetFileContent_DotDotThroughUnresolvedSymlink guards against
// resolving a ".." lexically before the symlink component it follows has
// itself been resolved. Given:
//
//	shared/alias -> ../actual         (a directory symlink)
//	links/p.yaml -> ../shared/alias/../task.yaml
//
// true POSIX symlink resolution treats "alias" as transparently replaced
// by its target for path purposes, so the trailing ".." applies *after*
// substitution, to the parent of "actual" -- landing on repo-root
// "task.yaml", not "actual/task.yaml" (confirmed against a real kernel
// via readlink -f; it's a common and easy mistake to assume otherwise).
// Collapsing "shared/alias/../task.yaml" with something like path.Clean
// *before* knowing "alias" is a symlink instead cancels the "alias/.."
// pair textually and lands on "shared/task.yaml" -- a third, different
// file, with no error raised. All three files exist with different
// content specifically so a resolution bug here fails on content, not
// just on an unexpected error.
func TestGetFileContent_DotDotThroughUnresolvedSymlink(t *testing.T) {
	repoDir, _ := createTestRepo(t, []commitForRepo{
		{Dir: "", Filename: "task.yaml", Content: "correct-content-at-repo-root"},
		{Dir: "actual", Filename: "task.yaml", Content: "wrong-content-if-dotdot-applies-before-alias-substitution"},
		{Dir: "shared", Filename: "task.yaml", Content: "wrong-content-if-dotdot-collapses-too-early"},
	})
	symlinkAndCommit(t, repoDir, "shared/alias", "../actual")
	symlinkAndCommit(t, repoDir, "links/p.yaml", "../shared/alias/../task.yaml")

	ctx := t.Context()
	repo, cleanup, err := remote{url: "file://" + repoDir, pathInRepo: "links/p.yaml"}.clone(ctx)
	defer cleanup()
	if err != nil {
		t.Fatalf("failed to clone: %v", err)
	}
	if !repo.partial {
		t.Fatal("expected the file:// clone to take the partial path")
	}
	if err := repo.checkout(ctx, "main"); err != nil {
		t.Fatalf("failed to checkout: %v", err)
	}

	content, err := repo.getFileContent(ctx, "links/p.yaml")
	if err != nil {
		t.Fatalf("expected the symlink chain to resolve, got error: %v", err)
	}
	if string(content) != "correct-content-at-repo-root" {
		t.Fatalf("expected %q (correct POSIX resolution: '..' applies after 'alias' is substituted by its target), got %q",
			"correct-content-at-repo-root", string(content))
	}
}

// TestGetFileContent_SymlinkTargetWithPathspecMagic guards against a
// symlink target that happens to contain git pathspec "magic" syntax
// (e.g. literally starting with ":(top)" or ":(icase)") being matched by
// git's pattern rules instead of treated as the literal filename a real
// filesystem would see. Without the ":(literal)" prefix in lsTreeEntry,
// `git ls-tree HEAD -- ':(top)other/decoy.yaml'` matches "other/decoy.yaml"
// even though no file is literally named ":(top)other/decoy.yaml" --
// silently returning a different file's content than the symlink's
// literal target names.
func TestGetFileContent_SymlinkTargetWithPathspecMagic(t *testing.T) {
	repoDir, _ := createTestRepo(t, []commitForRepo{
		{Dir: "other", Filename: "decoy.yaml", Content: "decoy-must-not-be-matched-via-pathspec-magic"},
	})
	// A target string that is valid pathspec magic syntax (matching
	// "other/decoy.yaml" if interpreted that way) but is not the literal
	// name of anything in the repository.
	symlinkAndCommit(t, repoDir, "links/p.yaml", "../:(top)other/decoy.yaml")

	ctx := t.Context()
	repo, cleanup, err := remote{url: "file://" + repoDir, pathInRepo: "links/p.yaml"}.clone(ctx)
	defer cleanup()
	if err != nil {
		t.Fatalf("failed to clone: %v", err)
	}
	if !repo.partial {
		t.Fatal("expected the file:// clone to take the partial path")
	}
	if err := repo.checkout(ctx, "main"); err != nil {
		t.Fatalf("failed to checkout: %v", err)
	}

	content, err := repo.getFileContent(ctx, "links/p.yaml")
	if err == nil {
		t.Fatalf("expected a non-existent literal target to be rejected, got content: %q", string(content))
	}
	if err.Error() != "file does not exist" {
		t.Fatalf("expected %q, got %q", "file does not exist", err.Error())
	}
}
