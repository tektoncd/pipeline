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

package stepper_test

import (
	"context"
	"github.com/ghodss/yaml"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned/scheme"
	"github.com/tektoncd/pipeline/pkg/remote"
	"github.com/tektoncd/pipeline/pkg/remote/file"
	"github.com/tektoncd/pipeline/pkg/stepper"
	"github.com/tektoncd/pipeline/test/diff"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var (
	// generateTestOutput enable to regenerate the expected output
	generateTestOutput = os.Getenv("REGENERATE_TEST_OUTPUT") == "true"
)

func TestStepper(t *testing.T) {
	sourceDir := filepath.Join("test_data", "tests")
	fs, err := ioutil.ReadDir(sourceDir)
	if err != nil {
		t.Errorf(errors.Wrapf(err, "failed to read source dir %s", sourceDir).Error())
	}

	// make it easy to run a specific test only
	runTestName := os.Getenv("TEST_NAME")
	for _, f := range fs {
		if !f.IsDir() {
			continue
		}
		name := f.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if runTestName != "" && runTestName != name {
			t.Logf("ignoring test %s\n", name)
			continue
		}

		dir := filepath.Join(sourceDir, name)
		path := filepath.Join(dir, "input.yaml")
		expectedPath := filepath.Join(dir, "expected.yaml")
		data, err := ioutil.ReadFile(path)
		if err != nil {
			t.Errorf(errors.Wrapf(err, "failed to read file %s", path).Error())
		}

		obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(data, nil, nil)
		if err != nil {
			t.Errorf(errors.Wrapf(err, "failed to unmarshal file %s", path).Error())
		}

		ctx := context.TODO()
		s := createTestStepper()
		obj, err = s.Resolve(ctx, obj)
		if err != nil {
			t.Errorf(errors.Wrapf(err, "failed to invoke stepper on file %s", path).Error())
		}

		data, err = yaml.Marshal(obj)
		if err != nil {
			t.Errorf(errors.Wrapf(err, "failed to marshal output of stepper on file %s", path).Error())
		}

		if generateTestOutput {
			err = ioutil.WriteFile(expectedPath, data, 0666)
			if err != nil {
				t.Errorf(errors.Wrapf(err, "failed to save file %s", expectedPath).Error())
			}
			continue
		}
		expectedData, err := ioutil.ReadFile(expectedPath)
		if err != nil {
			t.Errorf(errors.Wrapf(err, "failed to load file %s", expectedPath).Error())
		}

		got := strings.TrimSpace(string(data))
		want := strings.TrimSpace(string(expectedData))

		if d := cmp.Diff(want, got); d != "" {
			t.Errorf("path %s diff %s", path, diff.PrintWantGot(d))
			t.Errorf("actual content for %s was: %s", path, got)
		}
	}
}

func createTestStepper() *stepper.Resolver {
	if os.Getenv("STEPPER_USE_GIT") == "true" {
		opts := &stepper.RemoterOptions{}
		return stepper.NewResolver(opts)
	}
	fakeResolver := file.NewResolver(filepath.Join("test_data", "git"))
	remoteResolver := func(ctx context.Context, uses *v1beta1.Uses) (remote.Resolver, error) {
		return fakeResolver, nil
	}
	return &stepper.Resolver{ResolveRemote: remoteResolver}
}
