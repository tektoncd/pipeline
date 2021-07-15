/*
Copyright 2020 The Tekton Authors

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
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestCp(t *testing.T) {
	tmp, err := ioutil.TempDir("", "cp-test-*")
	if err != nil {
		t.Fatalf("error creating temp directory: %v", err)
	}
	defer os.RemoveAll(tmp)
	src := filepath.Join(tmp, "foo.txt")
	dst := filepath.Join(tmp, "bar.txt")

	if err = ioutil.WriteFile(src, []byte("hello world"), 0700); err != nil {
		t.Fatalf("error writing source file: %v", err)
	}

	if err := cp(src, dst); err != nil {
		t.Errorf("error copying: %v", err)
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
}

func TestCpMissingFile(t *testing.T) {
	tmp, err := ioutil.TempDir("", "cp-test-*")
	if err != nil {
		t.Fatalf("error creating temp directory: %v", err)
	}
	defer os.RemoveAll(tmp)
	src := filepath.Join(tmp, "doesnt-exist.txt")
	dst := filepath.Join(tmp, "bar.txt")
	err = cp(src, dst)
	if err == nil {
		t.Errorf("unexpected success copying missing file")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf(`expected "file does not exist" error but received %v`, err)
	}
}
