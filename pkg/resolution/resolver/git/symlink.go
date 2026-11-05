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
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
)

// maxSymlinkHops bounds symlink-chain resolution in resolveViaGitObjects so
// a cycle fails fast instead of looping forever. Matches Linux's own
// MAXSYMLINKS: a smaller, "this is probably a mistake" limit would reject
// a legitimate deep chain that today resolves fine through a full clone's
// filesystem-based filepath.EvalSymlinks, which is a behaviour change this
// partial-clone code path has no reason to introduce.
const maxSymlinkHops = 40

// maxSymlinkTargetLength bounds the raw byte length of any single symlink
// target this code will read, as a defensive limit against a maliciously
// huge blob -- this resolver walks repositories it does not control.
const maxSymlinkTargetLength = 4096

const symlinkMode = "120000"

// gitTreeEntry is one line of `git ls-tree`, with the path itself
// discarded: the caller always already knows the literal path it asked
// for, so there's nothing to recover by re-parsing it.
type gitTreeEntry struct {
	mode string
	typ  string
	sha  string
}

// resolveViaGitObjects resolves givenPath against HEAD's tree by reading
// git's object database directly, rather than the filesystem. It exists
// for exactly one situation: a partial, sparse clone (see clonePartial)
// checks out only the directory containing pathInRepo, so an in-repo
// symlink whose target -- at the leaf or at any intermediate path
// component -- falls outside that directory is left dangling on disk.
// filepath.EvalSymlinks then fails with a plain "not found", indistinguishable
// from the file genuinely not existing, which is what sent getFileContent
// here. See the PR #10665 review discussion this resolves.
//
// Resolution walks one path component at a time against a stack of
// already-validated real-tree components (resolved), rather than joining
// a symlink's target against its directory and lexically collapsing the
// result with something like path.Clean. That distinction matters: given
//
//	links/p.yaml   -> ../shared/alias/../task.yaml
//	shared/alias   -> ../actual/subdir
//
// POSIX symlink resolution resolves "shared/alias" to "actual/subdir"
// *before* applying the trailing "..", landing on "actual/task.yaml".
// Lexically collapsing "shared/alias/../task.yaml" first, without knowing
// "alias" is a symlink, collapses the "alias/.." pair away and lands on
// the wrong file, "shared/task.yaml", instead -- silently, with no error.
// Walking one component at a time avoids this: "alias" is looked up
// *before* the following ".." is ever consulted, so hitting the symlink
// there defers the ".." until after alias's own target is spliced in.
//
// Every git ls-tree lookup here is answered entirely from the clone's
// local object database: tree objects are always fetched in full, even
// under --filter=blob:none, so walking components costs no network round
// trips. Only `git cat-file -p` for a symlink target or the final leaf
// blob can trigger a lazy fetch -- at most one round trip per symlink hop,
// and never for a file that isn't actually on the chain.
func (repo *repository) resolveViaGitObjects(ctx context.Context, givenPath string) ([]byte, error) {
	remaining, err := splitRepoPath(givenPath)
	if err != nil {
		return nil, errors.New("file does not exist")
	}

	var resolved []string
	hops := 0

	for {
		if len(remaining) == 0 {
			return nil, fmt.Errorf("path %q did not resolve to a file", givenPath)
		}
		component, rest := remaining[0], remaining[1:]

		switch component {
		case "":
			remaining = rest
			continue
		case ".":
			remaining = rest
			continue
		case "..":
			if len(resolved) == 0 {
				return nil, fmt.Errorf("path %q climbs above the repository root", givenPath)
			}
			resolved = resolved[:len(resolved)-1]
			remaining = rest
			continue
		}

		// ":(literal)" disables git's pathspec magic (e.g. a target that
		// happens to read like ":(top)foo" or contain other ":(...)"
		// syntax) so the lookup can only ever match the literal path a
		// filesystem would see -- never a different file git's pattern
		// matching happens to also consider equivalent.
		pathspec := ":(literal)" + strings.Join(append(append([]string{}, resolved...), component), "/")

		entry, ok, err := repo.lsTreeEntry(ctx, pathspec)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errors.New("file does not exist")
		}

		if entry.mode == symlinkMode {
			hops++
			if hops > maxSymlinkHops {
				return nil, fmt.Errorf("too many levels of symlinks resolving %q", givenPath)
			}

			targetBytes, err := repo.catFileBlob(ctx, entry.sha)
			if err != nil {
				return nil, err
			}
			targetComponents, err := symlinkTargetComponents(pathspec, targetBytes)
			if err != nil {
				return nil, err
			}

			// The symlink's own directory (resolved, not yet including
			// this component -- symlinks aren't real trees, so component
			// is never pushed onto resolved) stays as the base; splice
			// the symlink's raw target components in before whatever
			// followed the symlink in the original lookup. Any ".." in
			// the target is deliberately left unresolved here -- it's
			// processed by this same loop, against whatever resolved
			// stack is current when the loop reaches it.
			targetComponents = append(targetComponents, rest...)
			remaining = targetComponents
			continue
		}

		if len(rest) == 0 {
			if entry.typ != "blob" {
				return nil, fmt.Errorf("path %q is a %s, not a file", givenPath, entry.typ)
			}
			return repo.catFileBlob(ctx, entry.sha)
		}

		if entry.typ != "tree" {
			return nil, fmt.Errorf("path %q: %q is not a directory", givenPath, strings.Join(append(resolved, component), "/"))
		}
		resolved = append(resolved, component)
		remaining = rest
	}
}

// splitRepoPath splits a resolver-supplied, repository-relative path into
// raw components. pathInRepo is a git tree path, always "/"-separated
// regardless of host OS (clonePartial already treats it this way, via
// Go's path package rather than path/filepath) -- there is no backslash
// normalization here, since this resolver only ever runs on Linux, and
// rewriting a literal backslash in a filename would corrupt a legal Unix
// path rather than normalize a Windows one.
func splitRepoPath(p string) ([]string, error) {
	if p == "" {
		return nil, errors.New("empty path")
	}
	if strings.HasPrefix(p, "/") {
		return nil, fmt.Errorf("path %q must be relative to the repository root", p)
	}
	return strings.Split(p, "/"), nil
}

// symlinkTargetComponents splits a symlink's raw blob content into path
// components, without trimming or otherwise altering the bytes: a target
// legitimately ending in whitespace names a different file than one that
// doesn't, and `git cat-file -p` returns a blob's exact content with
// nothing added. ".." and "." within the target are intentionally left
// unresolved here -- see resolveViaGitObjects' loop, which is what
// resolves them, in the correct order relative to any symlink nested
// inside the target.
func symlinkTargetComponents(symlinkPathspec string, rawTarget []byte) ([]string, error) {
	if len(rawTarget) == 0 {
		return nil, fmt.Errorf("symlink %q has an empty target", symlinkPathspec)
	}
	if len(rawTarget) > maxSymlinkTargetLength {
		return nil, fmt.Errorf("symlink %q target exceeds %d bytes", symlinkPathspec, maxSymlinkTargetLength)
	}
	if bytes.ContainsRune(rawTarget, 0) {
		return nil, fmt.Errorf("symlink %q target contains a NUL byte", symlinkPathspec)
	}
	target := string(rawTarget)
	if strings.HasPrefix(target, "/") {
		return nil, fmt.Errorf("symlink %q has an absolute target %q, which is not supported", symlinkPathspec, target)
	}
	return strings.Split(target, "/"), nil
}

// lsTreeEntry looks up a single literal pathspec against HEAD. ok is
// false, with no error, when the path simply doesn't exist at that exact
// location -- which is also what happens when an intermediate component
// is a symlink rather than a tree, since the pathspec then names nothing
// in the literal tree structure.
func (repo *repository) lsTreeEntry(ctx context.Context, pathspec string) (entry gitTreeEntry, ok bool, err error) {
	out, err := repo.execGit(ctx, "ls-tree", "HEAD", "--", pathspec)
	if err != nil {
		return gitTreeEntry{}, false, err
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		return gitTreeEntry{}, false, nil
	}

	fields := strings.Fields(line)
	if len(fields) < 3 {
		return gitTreeEntry{}, false, fmt.Errorf("unexpected 'git ls-tree' output for %q: %q", pathspec, line)
	}
	return gitTreeEntry{mode: fields[0], typ: fields[1], sha: fields[2]}, true, nil
}

func (repo *repository) catFileBlob(ctx context.Context, sha string) ([]byte, error) {
	return repo.execGit(ctx, "cat-file", "-p", "--", sha)
}
