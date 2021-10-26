package subcommands

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestStepInit(t *testing.T) {
	tmp, err := ioutil.TempDir("", "step-init-*")
	if err != nil {
		t.Fatalf("error creating temp directory: %v", err)
	}
	defer os.RemoveAll(tmp)

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
	if err := stepInit(steps); err != nil {
		t.Fatalf("stepInit: %v", err)
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
