/*
Copyright 2018 The Knative Authors

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

package configmap

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestLoad(t *testing.T) {
	want := map[string]string{
		"foo":    "bar",
		"a.b.c":  "blah",
		".z.y.x": "hidden!",
	}
	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("TempDir() = %v", err)
	}
	defer os.RemoveAll(tmpdir)

	// Kubernetes constructs ConfigMap Volumes in a strange way,
	// this attempts to reflect that so that our testing is more
	// representative of how this will function in the wild.
	//  $/key             -> ..data/key
	//  $/..data          -> {timestamp}
	//  $/..{timestamp}/key == value

	nowUnix := time.Now().Unix()
	tsPart := fmt.Sprintf("..%d", nowUnix)
	tsDir := path.Join(tmpdir, tsPart)
	if err := os.Mkdir(tsDir, 0755); err != nil {
		t.Fatalf("Mkdir() = %v", err)
	}
	dataLink := path.Join(tmpdir, "..data")
	if err := os.Symlink(tsDir, dataLink); err != nil {
		t.Fatalf("Symlink() = %v", err)
	}

	// Write out the files as they should be loaded.
	for k, v := range want {
		// Write the actual file to $/.{timestamp}/key
		if err := ioutil.WriteFile(path.Join(tsDir, k), []byte(v), 0644); err != nil {
			t.Fatalf("WriteFile(..{ts}/%s) = %v", k, err)
		}
		// Symlink $/key => $/..data/key
		if err := os.Symlink(path.Join(dataLink, k), path.Join(tmpdir, k)); err != nil {
			t.Fatalf("Symlink(%s, ..data/%s) = %v", k, k, err)
		}
	}

	got, err := Load(tmpdir)
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Load (-want, +got) = %v", diff)
	}
}

func TestLoadError(t *testing.T) {
	if data, err := Load("/does/not/exist"); err == nil {
		t.Errorf("Load() = %v, want error", data)
	}
}

// TODO(#1401): When Prow runs as a mere mortal, uncomment this.
// func TestReadFileError(t *testing.T) {
// 	written := map[string]string{
// 		"foo":    "bar",
// 		"a.b.c":  "blah",
// 		".z.y.x": "hidden!",
// 	}
// 	tmpdir, err := ioutil.TempDir("", "")
// 	if err != nil {
// 		t.Fatalf("TempDir() = %v", err)
// 	}
// 	defer os.RemoveAll(tmpdir)

// 	// Write out the files as write-only, so we fail reading.
// 	for k, v := range written {
// 		err := ioutil.WriteFile(path.Join(tmpdir, k), []byte(v), 0200)
// 		if err != nil {
// 			t.Fatalf("WriteFile(%q) = %v", k, err)
// 		}
// 	}

// 	if got, err := Load(tmpdir); err == nil {
// 		t.Fatalf("Load() = %v, want error", got)
// 	}
// }

func TestReadSymlinkedFileError(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("TempDir() = %v", err)
	}
	defer os.RemoveAll(tmpdir)

	if err := os.Symlink("not-found", path.Join(tmpdir, "foo")); err != nil {
		t.Fatalf("Symlink() = %v", err)
	}

	if got, err := Load(tmpdir); err == nil {
		t.Fatalf("Load() = %v, want error", got)
	}
}
