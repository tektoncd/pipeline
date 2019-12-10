/*
Copyright 2019 The Tekton Authors

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
package termination

import (
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/logging"
)

func TestExistingFile(t *testing.T) {
	tmpFile, err := ioutil.TempFile(os.TempDir(), "tempFile")
	if err != nil {
		log.Fatal("Cannot create temporary file", err)
	}
	// Remember to clean up the file afterwards
	defer os.Remove(tmpFile.Name())

	logger, _ := logging.NewLogger("", "test termination")
	defer func() {
		_ = logger.Sync()
	}()
	output := []v1alpha1.PipelineResourceResult{{
		Key:   "key1",
		Value: "hello",
	}}

	if err := WriteMessage(tmpFile.Name(), output); err != nil {
		logger.Fatalf("Errot while writing message: %s", err)
	}

	output = []v1alpha1.PipelineResourceResult{{
		Key:   "key2",
		Value: "world",
	}}

	if err := WriteMessage(tmpFile.Name(), output); err != nil {
		logger.Fatalf("Errot while writing message: %s", err)
	}

	if fileContents, err := ioutil.ReadFile(tmpFile.Name()); err != nil {
		logger.Fatalf("Unexpected error reading %v: %v", tmpFile.Name(), err)
	} else {
		want := `[{"name":"","digest":"","key":"key1","value":"hello","resourceRef":{}},{"name":"","digest":"","key":"key2","value":"world","resourceRef":{}}]`
		if d := cmp.Diff(want, string(fileContents)); d != "" {
			t.Fatalf("Diff(-want, got): %s", d)
		}
	}
}
