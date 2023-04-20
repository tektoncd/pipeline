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

package credentials

import (
	"os"
	"path/filepath"
	"testing"
)

const credContents string = "hello, world!"

func TestTryCopyCredDir(t *testing.T) {
	dir := t.TempDir()

	fakeCredDir := filepath.Join(dir, ".docker")
	err := os.Mkdir(fakeCredDir, 0700)
	if err != nil {
		t.Fatalf("unexpected error creating fake credential directory: %v", err)
	}
	credFilename := "important-credential.json"
	writeFakeCred(t, fakeCredDir, credFilename, credContents)
	destination := filepath.Join(dir, ".docker-copy")

	copiedFile := filepath.Join(destination, credFilename)
	if err := tryCopyCred(fakeCredDir, destination); err != nil {
		t.Fatalf("error creating copy of credential directory: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(destination, credFilename)); err != nil {
		t.Fatalf("error accessing copied credential: %v", err)
	}
	b, err := os.ReadFile(copiedFile)
	if err != nil {
		t.Fatalf("unexpected error opening copied file: %v", err)
	}
	if string(b) != credContents {
		t.Fatalf("mismatching file contents, expected %q received %q", credContents, string(b))
	}
}

func TestTryCopyCredFile(t *testing.T) {
	dir := t.TempDir()
	fakeCredFile := writeFakeCred(t, dir, ".git-credentials", credContents)
	destination := filepath.Join(dir, ".git-credentials-copy")

	if err := tryCopyCred(fakeCredFile, destination); err != nil {
		t.Fatalf("error creating copy of credential file: %v", err)
	}
	if _, err := os.Lstat(destination); err != nil {
		t.Fatalf("error accessing copied credential: %v", err)
	}
	b, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("unexpected error opening copied file: %v", err)
	}
	if string(b) != credContents {
		t.Fatalf("mismatching file contents, expected %q received %q", credContents, string(b))
	}
}

func TestTryCopyCredFileMissing(t *testing.T) {
	dir := t.TempDir()
	fakeCredFile := filepath.Join(dir, "foo")
	destination := filepath.Join(dir, "foo-copy")

	if err := tryCopyCred(fakeCredFile, destination); err != nil {
		t.Fatalf("error creating copy of credential file: %v", err)
	}
	if _, err := os.Lstat(destination); err != nil && !os.IsNotExist(err) {
		t.Fatalf("error accessing copied credential: %v", err)
	}
	_, err := os.ReadFile(destination)
	if !os.IsNotExist(err) {
		t.Fatalf("destination file exists but should not have been copied: %v", err)
	}
}

func writeFakeCred(t *testing.T, dir, name, contents string) string {
	t.Helper()
	flags := os.O_RDWR | os.O_CREATE | os.O_TRUNC
	path := filepath.Join(dir, name)
	cred, err := os.OpenFile(path, flags, 0600)
	if err != nil {
		t.Fatalf("unexpected error writing fake credential: %v", err)
	}
	_, _ = cred.Write([]byte(credContents))
	_ = cred.Close()
	return path
}
