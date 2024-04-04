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

package dockercreds

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/credentials"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFlagHandling(t *testing.T) {
	credentials.VolumePath = t.TempDir()
	dir := credentials.VolumeName("foo")
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, corev1.BasicAuthUsernameKey), []byte("bar"), 0o777); err != nil {
		t.Fatalf("os.WriteFile(username) = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, corev1.BasicAuthPasswordKey), []byte("baz"), 0o777); err != nil {
		t.Fatalf("os.WriteFile(password) = %v", err)
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	AddFlags(fs)
	err := fs.Parse([]string{
		"-basic-docker=foo=https://us.gcr.io",
	})
	if err != nil {
		t.Fatalf("flag.CommandLine.Parse() = %v", err)
	}

	t.Setenv("HOME", credentials.VolumePath)
	if err := NewBuilder().Write(credentials.VolumePath); err != nil {
		t.Fatalf("Write() = %v", err)
	}

	b, err := os.ReadFile(filepath.Join(credentials.VolumePath, ".docker", "config.json"))
	if err != nil {
		t.Fatalf("os.ReadFile(.docker/config.json) = %v", err)
	}

	// Note: "auth" is base64(username + ":" + password)
	expected := `{"auths":{"https://us.gcr.io":{"username":"bar","password":"baz","auth":"YmFyOmJheg==","email":"not@val.id"}}}`
	if string(b) != expected {
		t.Errorf("got: %v, wanted: %v", string(b), expected)
	}
}

func TestFlagHandlingTwice(t *testing.T) {
	credentials.VolumePath = t.TempDir()
	fooDir := credentials.VolumeName("foo")
	if err := os.MkdirAll(fooDir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", fooDir, err)
	}
	if err := os.WriteFile(filepath.Join(fooDir, corev1.BasicAuthUsernameKey), []byte("asdf"), 0o777); err != nil {
		t.Fatalf("os.WriteFile(username) = %v", err)
	}
	if err := os.WriteFile(filepath.Join(fooDir, corev1.BasicAuthPasswordKey), []byte("blah"), 0o777); err != nil {
		t.Fatalf("os.WriteFile(password) = %v", err)
	}
	barDir := credentials.VolumeName("bar")
	if err := os.MkdirAll(barDir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", barDir, err)
	}
	if err := os.WriteFile(filepath.Join(barDir, corev1.BasicAuthUsernameKey), []byte("bleh"), 0o777); err != nil {
		t.Fatalf("os.WriteFile(username) = %v", err)
	}
	if err := os.WriteFile(filepath.Join(barDir, corev1.BasicAuthPasswordKey), []byte("belch"), 0o777); err != nil {
		t.Fatalf("os.WriteFile(password) = %v", err)
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	AddFlags(fs)
	err := fs.Parse([]string{
		"-basic-docker=foo=https://us.gcr.io",
		"-basic-docker=bar=https://eu.gcr.io",
	})
	if err != nil {
		t.Fatalf("flag.CommandLine.Parse() = %v", err)
	}

	t.Setenv("HOME", credentials.VolumePath)
	if err := NewBuilder().Write(credentials.VolumePath); err != nil {
		t.Fatalf("Write() = %v", err)
	}

	b, err := os.ReadFile(filepath.Join(credentials.VolumePath, ".docker", "config.json"))
	if err != nil {
		t.Fatalf("os.ReadFile(.docker/config.json) = %v", err)
	}

	// Note: "auth" is base64(username + ":" + password)
	expected := `{"auths":{"https://eu.gcr.io":{"username":"bleh","password":"belch","auth":"YmxlaDpiZWxjaA==","email":"not@val.id"},"https://us.gcr.io":{"username":"asdf","password":"blah","auth":"YXNkZjpibGFo","email":"not@val.id"}}}`
	if string(b) != expected {
		t.Errorf("got: %v, wanted: %v", string(b), expected)
	}
}

func TestFlagHandlingMissingFiles(t *testing.T) {
	credentials.VolumePath = t.TempDir()
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
	credentials.VolumePath = t.TempDir()
	dir := credentials.VolumeName("foo")
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, corev1.BasicAuthUsernameKey), []byte("bar"), 0o777); err != nil {
		t.Fatalf("os.WriteFile(username) = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, corev1.BasicAuthPasswordKey), []byte("baz"), 0o777); err != nil {
		t.Fatalf("os.WriteFile(password) = %v", err)
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
					annotationPrefix + ".testkeys": "basickeys",
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
					annotationPrefix + ".testkeys": "keys",
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
					annotationPrefix + ".testkeys1": "keys1",
					annotationPrefix + ".testkeys2": "keys2",
					annotationPrefix + ".testkeys3": "keys3",
				},
			},
		},
		wantFlag: []string{
			"-basic-docker=ssh=keys1",
			"-basic-docker=ssh=keys2",
			"-basic-docker=ssh=keys3",
		},
	}, {
		secret: &corev1.Secret{
			Type:       corev1.SecretTypeDockercfg,
			ObjectMeta: metav1.ObjectMeta{Name: "my-dockercfg"},
		},
		wantFlag: []string{"-docker-cfg=my-dockercfg"},
	}, {
		secret: &corev1.Secret{
			Type:       corev1.SecretTypeDockerConfigJson,
			ObjectMeta: metav1.ObjectMeta{Name: "my-dockerconfigjson"},
		},
		wantFlag: []string{"-docker-config=my-dockerconfigjson"},
	}, {
		secret: &corev1.Secret{Type: corev1.SecretTypeOpaque},
	}, {
		secret: &corev1.Secret{Type: corev1.SecretTypeServiceAccountToken},
	}, {
		secret: &corev1.Secret{Type: corev1.SecretTypeTLS},
	}, {
		secret: &corev1.Secret{Type: corev1.SecretTypeBootstrapToken},
	}, {
		secret: &corev1.Secret{}, // An empty secret should result in no flags.
	}}

	nb := NewBuilder()
	for _, ts := range tests {
		gotFlag := nb.MatchingAnnotations(ts.secret)
		if !cmp.Equal(ts.wantFlag, gotFlag) {
			t.Errorf("Mismatch of flags for %v; want: %v got: %v ", ts.secret.Type, ts.wantFlag, gotFlag)
		}
	}
}

func TestMultipleFlagHandling(t *testing.T) {
	credentials.VolumePath = t.TempDir()
	fooDir := credentials.VolumeName("foo")
	if err := os.MkdirAll(fooDir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", fooDir, err)
	}
	if err := os.WriteFile(filepath.Join(fooDir, corev1.BasicAuthUsernameKey), []byte("bar"), 0o777); err != nil {
		t.Fatalf("os.WriteFile(username) = %v", err)
	}
	if err := os.WriteFile(filepath.Join(fooDir, corev1.BasicAuthPasswordKey), []byte("baz"), 0o777); err != nil {
		t.Fatalf("os.WriteFile(password) = %v", err)
	}

	barDir := credentials.VolumeName("bar")
	if err := os.MkdirAll(barDir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", barDir, err)
	}
	if err := os.WriteFile(filepath.Join(barDir, corev1.DockerConfigJsonKey), []byte(`{"auths":{"https://index.docker.io/v1":{"auth":"fooisbar"}}}`), 0o777); err != nil {
		t.Fatalf("os.WriteFile(username) = %v", err)
	}

	blubbDir := credentials.VolumeName("blubb")
	if err := os.MkdirAll(blubbDir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", blubbDir, err)
	}
	if err := os.WriteFile(filepath.Join(blubbDir, corev1.DockerConfigJsonKey), []byte(`{"auths":{"us.icr.io":{"auth":"fooisblubb"}}}`), 0o777); err != nil {
		t.Fatalf("os.WriteFile(username) = %v", err)
	}

	bazDir := credentials.VolumeName("baz")
	if err := os.MkdirAll(bazDir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", bazDir, err)
	}
	if err := os.WriteFile(filepath.Join(bazDir, corev1.DockerConfigKey), []byte(`{"https://my.registry/v1":{"auth":"fooisbaz"}}`), 0o777); err != nil {
		t.Fatalf("os.WriteFile(username) = %v", err)
	}

	blaDir := credentials.VolumeName("bla")
	if err := os.MkdirAll(blaDir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", blaDir, err)
	}
	if err := os.WriteFile(filepath.Join(blaDir, corev1.DockerConfigKey), []byte(`{"de.icr.io":{"auth":"fooisbla"}}`), 0o777); err != nil {
		t.Fatalf("os.WriteFile(username) = %v", err)
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	AddFlags(fs)
	err := fs.Parse([]string{
		"-basic-docker=foo=https://us.gcr.io",
		"-docker-config=bar",
		"-docker-config=blubb",
		"-docker-cfg=baz",
		"-docker-cfg=bla",
	})
	if err != nil {
		t.Fatalf("flag.CommandLine.Parse() = %v", err)
	}

	t.Setenv("HOME", credentials.VolumePath)
	if err := NewBuilder().Write(credentials.VolumePath); err != nil {
		t.Fatalf("Write() = %v", err)
	}

	b, err := os.ReadFile(filepath.Join(credentials.VolumePath, ".docker", "config.json"))
	if err != nil {
		t.Fatalf("os.ReadFile(.docker/config.json) = %v", err)
	}

	// Note: "auth" is base64(username + ":" + password)
	expected := `{"auths":{"de.icr.io":{"auth":"fooisbla"},"https://index.docker.io/v1":{"auth":"fooisbar"},"https://my.registry/v1":{"auth":"fooisbaz"},"https://us.gcr.io":{"username":"bar","password":"baz","auth":"YmFyOmJheg==","email":"not@val.id"},"us.icr.io":{"auth":"fooisblubb"}}}`
	if string(b) != expected {
		t.Errorf("got: %v, wanted: %v", string(b), expected)
	}
}

// TestNoAuthProvided confirms that providing zero secrets results in no docker
// credential file being written to disk.
func TestNoAuthProvided(t *testing.T) {
	credentials.VolumePath = t.TempDir()
	fooDir := credentials.VolumeName("foo")
	if err := os.MkdirAll(fooDir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", fooDir, err)
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	AddFlags(fs)
	err := fs.Parse([]string{})
	if err != nil {
		t.Fatalf("flag.CommandLine.Parse() = %v", err)
	}
	t.Setenv("HOME", credentials.VolumePath)
	if err := NewBuilder().Write(credentials.VolumePath); err != nil {
		t.Fatalf("Write() = %v", err)
	}
	_, err = os.ReadFile(filepath.Join(credentials.VolumePath, ".docker", "config.json"))
	if err == nil || !os.IsNotExist(err) {
		t.Errorf("expected does not exist error but received: %v", err)
	}
}
