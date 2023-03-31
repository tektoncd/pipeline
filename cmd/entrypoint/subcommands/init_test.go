/*
Copyright 2023 The Tekton Authors

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
	"os"
	"path/filepath"
	"testing"
)

func TestEntrypointInit(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "foo.txt")
	dst := filepath.Join(tmp, "bar.txt")
	if err := os.WriteFile(src, []byte("hello world"), 0700); err != nil {
		t.Fatalf("error writing source file: %v", err)
	}

	// Override tektonRoot for testing.
	tektonRoot = tmp

	// Create step directory so that symlinks can be successfully created.
	// This is typically done by volume mounts, so it needs to be done manually
	// in tests.
	stepDir := filepath.Join(tmp, "steps")
	if err := os.Mkdir(stepDir, os.ModePerm); err != nil {
		t.Fatalf("error creating step directory: %v", err)
	}

	steps := []string{"a", "b"}
	if err := entrypointInit(src, dst, steps); err != nil {
		t.Fatalf("stepInit: %v", err)
	}

	info, err := os.Lstat(dst)
	if err != nil {
		t.Fatalf("error statting destination file: %v", err)
	}

	// os.OpenFile is subject to umasks, so the created permissions of the
	// created dst file might be more restrictive than dstPermissions.
	// excludePerm represents the value of permissions we do not want in the
	// resulting file - e.g. if dstPermissions is 0311, excludePerm should be
	// 0466.
	// This is done instead of trying to look up the system umask, since this
	// relies on syscalls that we are not sure will be portable across
	// environments.
	excludePerm := os.ModePerm ^ dstPermissions
	if p := info.Mode().Perm(); p&excludePerm != 0 {
		t.Errorf("expected permissions <= %#o for destination file but found %#o", dstPermissions, p)
	}

	// Map of symlinks to expected /tekton/run folders.
	// Expected format:
	// Key: /tekton/steps/<key>
	// Value: /tekton/run/<value>/status
	wantLinks := map[string]string{
		"a": "0",
		"0": "0",
		"b": "1",
		"1": "1",
	}

	direntry, err := os.ReadDir(stepDir)
	if err != nil {
		t.Fatalf("os.ReadDir: %v", err)
	}
	for _, de := range direntry {
		t.Run(de.Name(), func(t *testing.T) {
			l, err := os.Readlink(filepath.Join(stepDir, de.Name()))
			if err != nil {
				t.Fatal(err)
			}
			want, ok := wantLinks[de.Name()]
			if !ok {
				t.Fatalf("unexpected symlink: %s", de.Name())
			}
			if wantDir := filepath.Join(tmp, "run", want, "status"); l != wantDir {
				t.Errorf("want %s, got %s", wantDir, l)
			}
		})
	}
}
