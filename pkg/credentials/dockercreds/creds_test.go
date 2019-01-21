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

package dockercreds

import (
	"flag"
	"fmt"
	"testing"

	"gotest.tools/assert"
	"gotest.tools/env"
	"gotest.tools/fs"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/knative/build-pipeline/pkg/credentials"
)

func TestFlagHandling(t *testing.T) {
	dir := fs.NewDir(t, "", fs.WithDir("foo",
		fs.WithFile(corev1.BasicAuthUsernameKey, "bar"),
		fs.WithFile(corev1.BasicAuthPasswordKey, "baz"),
	))
	defer dir.Remove()
	credentials.VolumePath = dir.Path()

	flagset := flag.NewFlagSet("test", flag.ContinueOnError)
	flags(flagset)
	assert.NilError(t, flagset.Parse([]string{
		"-basic-docker=foo=https://us.gcr.io",
	}))

	defer env.Patch(t, "HOME", credentials.VolumePath)()
	assert.NilError(t, NewBuilder().Write())

	// Note: "auth" is base64(username + ":" + password)
	expectedConfig := `{"auths":{"https://us.gcr.io":{"username":"bar","password":"baz","auth":"YmFyOmJheg==","email":"not@val.id"}}}`
	expected := fs.Expected(t, fs.WithDir(".docker",
		fs.WithFile("config.json", expectedConfig, fs.MatchAnyFileMode),
	), fs.MatchExtraFiles)
	assert.Assert(t, fs.Equal(credentials.VolumePath, expected))
}

func TestFlagHandlingTwice(t *testing.T) {
	dir := fs.NewDir(t, "", fs.WithDir("foo",
		fs.WithFile(corev1.BasicAuthUsernameKey, "asdf"),
		fs.WithFile(corev1.BasicAuthPasswordKey, "blah"),
	), fs.WithDir("bar",
		fs.WithFile(corev1.BasicAuthUsernameKey, "bleh"),
		fs.WithFile(corev1.BasicAuthPasswordKey, "belch"),
	))
	defer dir.Remove()
	credentials.VolumePath = dir.Path()

	flagset := flag.NewFlagSet("test", flag.ContinueOnError)
	flags(flagset)
	assert.NilError(t, flagset.Parse([]string{
		"-basic-docker=foo=https://us.gcr.io",
		"-basic-docker=bar=https://eu.gcr.io",
	}))
	defer env.Patch(t, "HOME", credentials.VolumePath)()
	assert.NilError(t, NewBuilder().Write())

	// Note: "auth" is base64(username + ":" + password)
	expectedConfig := `{"auths":{"https://eu.gcr.io":{"username":"bleh","password":"belch","auth":"YmxlaDpiZWxjaA==","email":"not@val.id"},"https://us.gcr.io":{"username":"asdf","password":"blah","auth":"YXNkZjpibGFo","email":"not@val.id"}}}`
	expected := fs.Expected(t, fs.WithDir(".docker",
		fs.WithFile("config.json", expectedConfig, fs.MatchAnyFileMode),
	), fs.MatchExtraFiles)
	assert.Assert(t, fs.Equal(credentials.VolumePath, expected))
}

func TestFlagHandlingMissingFiles(t *testing.T) {
	dir := fs.NewDir(t, "", fs.WithDir("not-found"))
	defer dir.Remove()
	credentials.VolumePath = dir.Path()

	// No username / password files yields an error.
	cfg := dockerConfig{make(map[string]entry)}
	if err := cfg.Set("not-found=https://us.gcr.io"); err == nil {
		t.Error("Set(); got success, wanted error.")
	}
}

func TestFlagHandlingURLCollision(t *testing.T) {
	dir := fs.NewDir(t, "", fs.WithDir("foo",
		fs.WithFile(corev1.BasicAuthUsernameKey, "bar"),
		fs.WithFile(corev1.BasicAuthPasswordKey, "baz"),
	))
	defer dir.Remove()
	credentials.VolumePath = dir.Path()

	cfg := dockerConfig{make(map[string]entry)}
	if err := cfg.Set("foo=https://us.gcr.io"); err != nil {
		t.Fatalf("First Set() = %v", err)
	}
	if err := cfg.Set("bar=https://us.gcr.io"); err == nil {
		t.Error("Second Set(); got success, wanted error.")
	}
}

func TestMalformedValueTooMany(t *testing.T) {
	cfg := dockerConfig{make(map[string]entry)}
	if err := cfg.Set("bar=baz=blah"); err == nil {
		t.Error("Second Set(); got success, wanted error.")
	}
}

func TestMalformedValueTooFew(t *testing.T) {
	cfg := dockerConfig{make(map[string]entry)}
	if err := cfg.Set("bar"); err == nil {
		t.Error("Second Set(); got success, wanted error.")
	}
}

func TestMatchingAnnotations(t *testing.T) {
	tests := []struct {
		secret   *corev1.Secret
		wantFlag []string
	}{{
		secret: &corev1.Secret{
			Type: corev1.SecretTypeBasicAuth,
			ObjectMeta: metav1.ObjectMeta{
				Name: "git",
				Annotations: map[string]string{
					fmt.Sprintf("%s.testkeys", annotationPrefix): "basickeys",
				},
			},
		},
		wantFlag: []string{"-basic-docker=git=basickeys"},
	}, {
		secret: &corev1.Secret{
			Type: corev1.SecretTypeSSHAuth,
			ObjectMeta: metav1.ObjectMeta{
				Name: "ssh",
				Annotations: map[string]string{
					fmt.Sprintf("%s.testkeys", annotationPrefix): "keys",
				},
			},
		},
		wantFlag: []string(nil),
	}, {
		secret: &corev1.Secret{
			Type: corev1.SecretTypeBasicAuth,
			ObjectMeta: metav1.ObjectMeta{
				Name: "ssh",
				Annotations: map[string]string{
					fmt.Sprintf("%s.testkeys1", annotationPrefix): "keys1",
					fmt.Sprintf("%s.testkeys2", annotationPrefix): "keys2",
					fmt.Sprintf("%s.testkeys3", annotationPrefix): "keys3",
				},
			},
		},
		wantFlag: []string{"-basic-docker=ssh=keys1",
			"-basic-docker=ssh=keys2",
			"-basic-docker=ssh=keys3",
		},
	}}

	nb := NewBuilder()
	for _, ts := range tests {
		gotFlag := nb.MatchingAnnotations(ts.secret)
		assert.DeepEqual(t, ts.wantFlag, gotFlag)
	}
}
