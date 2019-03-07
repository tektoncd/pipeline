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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tektoncd/pipeline/pkg/credentials"
)

func TestFlagHandling(t *testing.T) {
	credentials.VolumePath, _ = ioutil.TempDir("", "")
	dir := credentials.VolumeName("foo")
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", dir, err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, corev1.BasicAuthUsernameKey), []byte("bar"), 0777); err != nil {
		t.Fatalf("ioutil.WriteFile(username) = %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, corev1.BasicAuthPasswordKey), []byte("baz"), 0777); err != nil {
		t.Fatalf("ioutil.WriteFile(password) = %v", err)
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	flags(fs)
	err := fs.Parse([]string{
		"-basic-docker=foo=https://us.gcr.io",
	})
	if err != nil {
		t.Fatalf("flag.CommandLine.Parse() = %v", err)
	}

	os.Setenv("HOME", credentials.VolumePath)
	if err := NewBuilder().Write(); err != nil {
		t.Fatalf("Write() = %v", err)
	}

	b, err := ioutil.ReadFile(filepath.Join(credentials.VolumePath, ".docker", "config.json"))
	if err != nil {
		t.Fatalf("ioutil.ReadFile(.docker/config.json) = %v", err)
	}

	// Note: "auth" is base64(username + ":" + password)
	expected := `{"auths":{"https://us.gcr.io":{"username":"bar","password":"baz","auth":"YmFyOmJheg==","email":"not@val.id"}}}`
	if string(b) != expected {
		t.Errorf("got: %v, wanted: %v", string(b), expected)
	}
}

func TestFlagHandlingTwice(t *testing.T) {
	credentials.VolumePath, _ = ioutil.TempDir("", "")
	fooDir := credentials.VolumeName("foo")
	if err := os.MkdirAll(fooDir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", fooDir, err)
	}
	if err := ioutil.WriteFile(filepath.Join(fooDir, corev1.BasicAuthUsernameKey), []byte("asdf"), 0777); err != nil {
		t.Fatalf("ioutil.WriteFile(username) = %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(fooDir, corev1.BasicAuthPasswordKey), []byte("blah"), 0777); err != nil {
		t.Fatalf("ioutil.WriteFile(password) = %v", err)
	}
	barDir := credentials.VolumeName("bar")
	if err := os.MkdirAll(barDir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", barDir, err)
	}
	if err := ioutil.WriteFile(filepath.Join(barDir, corev1.BasicAuthUsernameKey), []byte("bleh"), 0777); err != nil {
		t.Fatalf("ioutil.WriteFile(username) = %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(barDir, corev1.BasicAuthPasswordKey), []byte("belch"), 0777); err != nil {
		t.Fatalf("ioutil.WriteFile(password) = %v", err)
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	flags(fs)
	err := fs.Parse([]string{
		"-basic-docker=foo=https://us.gcr.io",
		"-basic-docker=bar=https://eu.gcr.io",
	})
	if err != nil {
		t.Fatalf("flag.CommandLine.Parse() = %v", err)
	}

	os.Setenv("HOME", credentials.VolumePath)
	if err := NewBuilder().Write(); err != nil {
		t.Fatalf("Write() = %v", err)
	}

	b, err := ioutil.ReadFile(filepath.Join(credentials.VolumePath, ".docker", "config.json"))
	if err != nil {
		t.Fatalf("ioutil.ReadFile(.docker/config.json) = %v", err)
	}

	// Note: "auth" is base64(username + ":" + password)
	expected := `{"auths":{"https://eu.gcr.io":{"username":"bleh","password":"belch","auth":"YmxlaDpiZWxjaA==","email":"not@val.id"},"https://us.gcr.io":{"username":"asdf","password":"blah","auth":"YXNkZjpibGFo","email":"not@val.id"}}}`
	if string(b) != expected {
		t.Errorf("got: %v, wanted: %v", string(b), expected)
	}
}

func TestFlagHandlingMissingFiles(t *testing.T) {
	credentials.VolumePath, _ = ioutil.TempDir("", "")
	dir := credentials.VolumeName("not-found")
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", dir, err)
	}
	// No username / password files yields an error.

	cfg := basicDocker{make(map[string]entry)}
	if err := cfg.Set("not-found=https://us.gcr.io"); err == nil {
		t.Error("Set(); got success, wanted error.")
	}
}

func TestFlagHandlingURLCollision(t *testing.T) {
	credentials.VolumePath, _ = ioutil.TempDir("", "")
	dir := credentials.VolumeName("foo")
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", dir, err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, corev1.BasicAuthUsernameKey), []byte("bar"), 0777); err != nil {
		t.Fatalf("ioutil.WriteFile(username) = %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, corev1.BasicAuthPasswordKey), []byte("baz"), 0777); err != nil {
		t.Fatalf("ioutil.WriteFile(password) = %v", err)
	}

	cfg := basicDocker{make(map[string]entry)}
	if err := cfg.Set("foo=https://us.gcr.io"); err != nil {
		t.Fatalf("First Set() = %v", err)
	}
	if err := cfg.Set("bar=https://us.gcr.io"); err == nil {
		t.Error("Second Set(); got success, wanted error.")
	}
}

func TestMalformedValueTooMany(t *testing.T) {
	cfg := basicDocker{make(map[string]entry)}
	if err := cfg.Set("bar=baz=blah"); err == nil {
		t.Error("Second Set(); got success, wanted error.")
	}
}

func TestMalformedValueTooFew(t *testing.T) {
	cfg := basicDocker{make(map[string]entry)}
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
		if !cmp.Equal(ts.wantFlag, gotFlag) {
			t.Errorf("MatchingAnnotations() Mismatch of flags; wanted: %v got: %v ", ts.wantFlag, gotFlag)
		}
	}
}

func TestMultipleFlagHandling(t *testing.T) {
	credentials.VolumePath, _ = ioutil.TempDir("", "")
	fooDir := credentials.VolumeName("foo")
	if err := os.MkdirAll(fooDir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", fooDir, err)
	}
	if err := ioutil.WriteFile(filepath.Join(fooDir, corev1.BasicAuthUsernameKey), []byte("bar"), 0777); err != nil {
		t.Fatalf("ioutil.WriteFile(username) = %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(fooDir, corev1.BasicAuthPasswordKey), []byte("baz"), 0777); err != nil {
		t.Fatalf("ioutil.WriteFile(password) = %v", err)
	}

	barDir := credentials.VolumeName("bar")
	if err := os.MkdirAll(barDir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", barDir, err)
	}
	if err := ioutil.WriteFile(filepath.Join(barDir, corev1.DockerConfigJsonKey), []byte(`{"auths":{"https://index.docker.io/v1":{"auth":"fooisbar"}}}`), 0777); err != nil {
		t.Fatalf("ioutil.WriteFile(username) = %v", err)
	}

	bazDir := credentials.VolumeName("baz")
	if err := os.MkdirAll(bazDir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", bazDir, err)
	}
	if err := ioutil.WriteFile(filepath.Join(bazDir, corev1.DockerConfigKey), []byte(`{"https://my.registry/v1":{"auth":"fooisbaz"}}`), 0777); err != nil {
		t.Fatalf("ioutil.WriteFile(username) = %v", err)
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	flags(fs)
	err := fs.Parse([]string{
		"-basic-docker=foo=https://us.gcr.io",
		"-docker-config=bar",
		"-docker-cfg=baz",
	})
	if err != nil {
		t.Fatalf("flag.CommandLine.Parse() = %v", err)
	}

	os.Setenv("HOME", credentials.VolumePath)
	if err := NewBuilder().Write(); err != nil {
		t.Fatalf("Write() = %v", err)
	}

	b, err := ioutil.ReadFile(filepath.Join(credentials.VolumePath, ".docker", "config.json"))
	if err != nil {
		t.Fatalf("ioutil.ReadFile(.docker/config.json) = %v", err)
	}

	// Note: "auth" is base64(username + ":" + password)
	expected := `{"auths":{"https://index.docker.io/v1":{"auth":"fooisbar"},"https://my.registry/v1":{"auth":"fooisbaz"},"https://us.gcr.io":{"username":"bar","password":"baz","auth":"YmFyOmJheg==","email":"not@val.id"}}}`
	if string(b) != expected {
		t.Errorf("got: %v, wanted: %v", string(b), expected)
	}
}
