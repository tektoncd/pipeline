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
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestDecodeScript(t *testing.T) {
	encoded := "IyEvdXNyL2Jpbi9lbnYgc2gKZWNobyAiSGVsbG8gV29ybGQhIgo="
	decoded := `#!/usr/bin/env sh
echo "Hello World!"
`
	mode := os.FileMode(0600)
	expectedPermissions := os.FileMode(0600)

	tmp := t.TempDir()
	src := filepath.Join(tmp, "script.txt")
	defer func() {
		if err := os.Remove(src); err != nil {
			t.Errorf("temporary script file %q was not cleaned up: %v", src, err)
		}
	}()
	if err := os.WriteFile(src, []byte(encoded), mode); err != nil {
		t.Fatalf("error writing encoded script: %v", err)
	}

	if err := decodeScript(src); err != nil {
		t.Errorf("unexpected error decoding script: %v", err)
	}

	file, err := os.Open(src)
	if err != nil {
		t.Fatalf("unexpected error opening decoded script: %v", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatalf("unexpected error statting decoded script: %v", err)
	}
	mod := info.Mode()
	b, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("unexpected error reading content of decoded script: %v", err)
	}
	if string(b) != decoded {
		t.Errorf("expected decoded value %q received %q", decoded, string(b))
	}
	if mod != expectedPermissions {
		t.Errorf("expected mode %#o received %#o", expectedPermissions, mod)
	}
}

func TestDecodeScriptMissingFileError(t *testing.T) {
	b, mod, err := decodeScriptFromFile("/path/to/non-existent/file")
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected error %q received %q", os.ErrNotExist, err)
	}
	if b != nil || mod != 0 {
		t.Errorf("unexpected non-zero bytes or file mode returned")
	}
}

func TestDecodeScriptInvalidBase64(t *testing.T) {
	invalidData := []byte("!")
	expectedError := base64.CorruptInputError(0)

	src, err := os.CreateTemp("", "decode-script-test-*")
	if err != nil {
		t.Fatalf("error creating temp file: %v", err)
	}
	defer func() {
		if err := os.Remove(src.Name()); err != nil {
			t.Errorf("temporary file %q was not cleaned up: %v", src.Name(), err)
		}
	}()
	if _, err := src.Write(invalidData); err != nil {
		t.Fatalf("error writing invalid base64 data: %v", err)
	}
	src.Close()

	b, mod, err := decodeScriptFromFile(src.Name())
	if b != nil || mod != 0 {
		t.Errorf("unexpected non-zero bytes or file mode returned")
	}
	if !errors.Is(err, expectedError) {
		t.Errorf("expected error %q received %q", expectedError, err)
	}
}
