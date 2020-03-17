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

package remote

import (
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
)

func TestOCIResolver(t *testing.T) {
	// Set up a fake registry to push an image to.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Create the image using an example task.
	task := tb.Task("hello-world", "", tb.TaskType(), tb.TaskSpec(tb.Step("ubuntu", tb.StepCommand("echo 'Hello'"))))
	imgRef, err := test.CreateTaskImage(u.Host, task)
	if err != nil {
		t.Errorf("unexpected error pushing task image %w", err)
	}

	// Now we can call our resolver and see if the spec returned is the same.
	resolver := OCIResolver{
		imageReference: imgRef,
		keychain:       authn.DefaultKeychain,
	}

	ta, err := resolver.GetTask("hello-world")
	if err != nil {
		t.Errorf("failed to fetch task hello-world: %w", err)
	}

	if diff := cmp.Diff(ta, task); diff != "" {
		t.Error(diff)
	}
}
