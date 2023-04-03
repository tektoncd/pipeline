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

package gitcreds

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/credentials"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBasicFlagHandling(t *testing.T) {
	credentials.VolumePath = t.TempDir()
	dir := credentials.VolumeName("foo")
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, corev1.BasicAuthUsernameKey), []byte("bar"), 0777); err != nil {
		t.Fatalf("os.WriteFile(username) = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, corev1.BasicAuthPasswordKey), []byte("baz"), 0777); err != nil {
		t.Fatalf("os.WriteFile(password) = %v", err)
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	AddFlags(fs)
	err := fs.Parse([]string{
		"-basic-git=foo=https://github.com",
	})
	if err != nil {
		t.Fatalf("flag.CommandLine.Parse() = %v", err)
	}

	t.Setenv("HOME", credentials.VolumePath)
	if err := NewBuilder().Write(credentials.VolumePath); err != nil {
		t.Fatalf("Write() = %v", err)
	}

	b, err := os.ReadFile(filepath.Join(credentials.VolumePath, ".gitconfig"))
	if err != nil {
		t.Fatalf("os.ReadFile(.gitconfig) = %v", err)
	}

	expectedGitConfig := `[credential]
	helper = store
[credential "https://github.com"]
	username = bar
`
	if string(b) != expectedGitConfig {
		t.Errorf("got: %v, wanted: %v", string(b), expectedGitConfig)
	}

	b, err = os.ReadFile(filepath.Join(credentials.VolumePath, ".git-credentials"))
	if err != nil {
		t.Fatalf("os.ReadFile(.git-credentials) = %v", err)
	}

	expectedGitCredentials := `https://bar:baz@github.com
`
	if string(b) != expectedGitCredentials {
		t.Errorf("got: %v, wanted: %v", string(b), expectedGitCredentials)
	}
}

func TestBasicFlagHandlingTwice(t *testing.T) {
	credentials.VolumePath = t.TempDir()
	fooDir := credentials.VolumeName("foo")
	if err := os.MkdirAll(fooDir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", fooDir, err)
	}
	if err := os.WriteFile(filepath.Join(fooDir, corev1.BasicAuthUsernameKey), []byte("asdf"), 0777); err != nil {
		t.Fatalf("os.WriteFile(username) = %v", err)
	}
	if err := os.WriteFile(filepath.Join(fooDir, corev1.BasicAuthPasswordKey), []byte("blah"), 0777); err != nil {
		t.Fatalf("os.WriteFile(password) = %v", err)
	}
	barDir := credentials.VolumeName("bar")
	if err := os.MkdirAll(barDir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", barDir, err)
	}
	if err := os.WriteFile(filepath.Join(barDir, corev1.BasicAuthUsernameKey), []byte("bleh"), 0777); err != nil {
		t.Fatalf("os.WriteFile(username) = %v", err)
	}
	if err := os.WriteFile(filepath.Join(barDir, corev1.BasicAuthPasswordKey), []byte("belch"), 0777); err != nil {
		t.Fatalf("os.WriteFile(password) = %v", err)
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	AddFlags(fs)
	err := fs.Parse([]string{
		"-basic-git=foo=https://github.com",
		"-basic-git=bar=https://gitlab.com",
	})
	if err != nil {
		t.Fatalf("flag.CommandLine.Parse() = %v", err)
	}

	t.Setenv("HOME", credentials.VolumePath)
	if err := NewBuilder().Write(credentials.VolumePath); err != nil {
		t.Fatalf("Write() = %v", err)
	}

	b, err := os.ReadFile(filepath.Join(credentials.VolumePath, ".gitconfig"))
	if err != nil {
		t.Fatalf("os.ReadFile(.gitconfig) = %v", err)
	}

	expectedGitConfig := `[credential]
	helper = store
[credential "https://github.com"]
	username = asdf
[credential "https://gitlab.com"]
	username = bleh
`
	if string(b) != expectedGitConfig {
		t.Errorf("got: %v, wanted: %v", string(b), expectedGitConfig)
	}

	b, err = os.ReadFile(filepath.Join(credentials.VolumePath, ".git-credentials"))
	if err != nil {
		t.Fatalf("os.ReadFile(.git-credentials) = %v", err)
	}

	expectedGitCredentials := `https://asdf:blah@github.com
https://bleh:belch@gitlab.com
`
	if string(b) != expectedGitCredentials {
		t.Errorf("got: %v, wanted: %v", string(b), expectedGitCredentials)
	}
}

func TestBasicFlagHandlingMissingFiles(t *testing.T) {
	credentials.VolumePath = t.TempDir()
	dir := credentials.VolumeName("not-found")
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", dir, err)
	}
	// No username / password files yields an error.

	cfg := basicGitConfig{entries: make(map[string]basicEntry)}
	if err := cfg.Set("not-found=https://github.com"); err == nil {
		t.Error("Set(); got success, wanted error.")
	}
}

func TestBasicFlagHandlingURLCollision(t *testing.T) {
	credentials.VolumePath = t.TempDir()
	dir := credentials.VolumeName("foo")
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, corev1.BasicAuthUsernameKey), []byte("bar"), 0777); err != nil {
		t.Fatalf("os.WriteFile(username) = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, corev1.BasicAuthPasswordKey), []byte("baz"), 0777); err != nil {
		t.Fatalf("os.WriteFile(password) = %v", err)
	}

	cfg := basicGitConfig{entries: make(map[string]basicEntry)}
	if err := cfg.Set("foo=https://github.com"); err != nil {
		t.Fatalf("First Set() = %v", err)
	}
	if err := cfg.Set("bar=https://github.com"); err == nil {
		t.Error("Second Set(); got success, wanted error.")
	}
}

func TestSSHFlagHandling(t *testing.T) {
	credentials.VolumePath = t.TempDir()
	dir := credentials.VolumeName("foo")
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, corev1.SSHAuthPrivateKey), []byte("bar"), 0777); err != nil {
		t.Fatalf("os.WriteFile(ssh-privatekey) = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "known_hosts"), []byte("ssh-rsa blah"), 0777); err != nil {
		t.Fatalf("os.WriteFile(known_hosts) = %v", err)
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	AddFlags(fs)
	err := fs.Parse([]string{
		"-ssh-git=foo=github.com",
	})
	if err != nil {
		t.Fatalf("flag.CommandLine.Parse() = %v", err)
	}

	t.Setenv("HOME", credentials.VolumePath)
	if err := NewBuilder().Write(credentials.VolumePath); err != nil {
		t.Fatalf("Write() = %v", err)
	}

	b, err := os.ReadFile(filepath.Join(credentials.VolumePath, ".ssh", "config"))
	if err != nil {
		t.Fatalf("os.ReadFile(.ssh/config) = %v", err)
	}

	expectedSSHConfig := fmt.Sprintf(`Host github.com
    HostName github.com
    Port 22
    IdentityFile %s/.ssh/id_foo
`, credentials.VolumePath)
	if d := cmp.Diff(expectedSSHConfig, string(b)); d != "" {
		t.Errorf("ssh_config diff %s", diff.PrintWantGot(d))
	}

	b, err = os.ReadFile(filepath.Join(credentials.VolumePath, ".ssh", "known_hosts"))
	if err != nil {
		t.Fatalf("os.ReadFile(.ssh/known_hosts) = %v", err)
	}
	expectedSSHKnownHosts := `ssh-rsa blah`
	if string(b) != expectedSSHKnownHosts {
		t.Errorf("got: %v, wanted: %v", string(b), expectedSSHKnownHosts)
	}

	b, err = os.ReadFile(filepath.Join(credentials.VolumePath, ".ssh", "id_foo"))
	if err != nil {
		t.Fatalf("os.ReadFile(.ssh/id_foo) = %v", err)
	}

	expectedIDFoo := `bar`
	if string(b) != expectedIDFoo {
		t.Errorf("got: %v, wanted: %v", string(b), expectedIDFoo)
	}
}

func TestSSHFlagHandlingThrice(t *testing.T) {
	credentials.VolumePath = t.TempDir()
	fooDir := credentials.VolumeName("foo")
	if err := os.MkdirAll(fooDir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", fooDir, err)
	}
	if err := os.WriteFile(filepath.Join(fooDir, corev1.SSHAuthPrivateKey), []byte("asdf"), 0777); err != nil {
		t.Fatalf("os.WriteFile(ssh-privatekey) = %v", err)
	}
	if err := os.WriteFile(filepath.Join(fooDir, "known_hosts"), []byte("ssh-rsa aaaa"), 0777); err != nil {
		t.Fatalf("os.WriteFile(known_hosts) = %v", err)
	}
	barDir := credentials.VolumeName("bar")
	if err := os.MkdirAll(barDir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", barDir, err)
	}
	if err := os.WriteFile(filepath.Join(barDir, corev1.SSHAuthPrivateKey), []byte("bleh"), 0777); err != nil {
		t.Fatalf("os.WriteFile(ssh-privatekey) = %v", err)
	}
	if err := os.WriteFile(filepath.Join(barDir, "known_hosts"), []byte("ssh-rsa bbbb"), 0777); err != nil {
		t.Fatalf("os.WriteFile(known_hosts) = %v", err)
	}
	bazDir := credentials.VolumeName("baz")
	if err := os.MkdirAll(bazDir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", bazDir, err)
	}
	if err := os.WriteFile(filepath.Join(bazDir, corev1.SSHAuthPrivateKey), []byte("derp"), 0777); err != nil {
		t.Fatalf("os.WriteFile(ssh-privatekey) = %v", err)
	}
	if err := os.WriteFile(filepath.Join(bazDir, "known_hosts"), []byte("ssh-rsa cccc"), 0777); err != nil {
		t.Fatalf("os.WriteFile(known_hosts) = %v", err)
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	AddFlags(fs)
	err := fs.Parse([]string{
		// Two secrets target github.com, and both will end up in the
		// ssh config.
		"-ssh-git=foo=github.com",
		"-ssh-git=bar=github.com",
		"-ssh-git=baz=gitlab.example.com:2222",
	})
	if err != nil {
		t.Fatalf("flag.CommandLine.Parse() = %v", err)
	}

	t.Setenv("HOME", credentials.VolumePath)
	if err := NewBuilder().Write(credentials.VolumePath); err != nil {
		t.Fatalf("Write() = %v", err)
	}

	b, err := os.ReadFile(filepath.Join(credentials.VolumePath, ".ssh", "config"))
	if err != nil {
		t.Fatalf("os.ReadFile(.ssh/config) = %v", err)
	}

	expectedSSHConfig := fmt.Sprintf(`Host github.com
    HostName github.com
    Port 22
    IdentityFile %s/.ssh/id_foo
    IdentityFile %s/.ssh/id_bar
Host gitlab.example.com
    HostName gitlab.example.com
    Port 2222
    IdentityFile %s/.ssh/id_baz
`, credentials.VolumePath, credentials.VolumePath, credentials.VolumePath)
	if d := cmp.Diff(expectedSSHConfig, string(b)); d != "" {
		t.Errorf("ssh_config diff %s", diff.PrintWantGot(d))
	}

	b, err = os.ReadFile(filepath.Join(credentials.VolumePath, ".ssh", "known_hosts"))
	if err != nil {
		t.Fatalf("os.ReadFile(.ssh/known_hosts) = %v", err)
	}
	expectedSSHKnownHosts := `ssh-rsa aaaa
ssh-rsa bbbb
ssh-rsa cccc`
	if d := cmp.Diff(expectedSSHKnownHosts, string(b)); d != "" {
		t.Errorf("known_hosts diff %s", diff.PrintWantGot(d))
	}

	b, err = os.ReadFile(filepath.Join(credentials.VolumePath, ".ssh", "id_foo"))
	if err != nil {
		t.Fatalf("os.ReadFile(.ssh/id_foo) = %v", err)
	}

	expectedIDFoo := `asdf`
	if string(b) != expectedIDFoo {
		t.Errorf("got: %v, wanted: %v", string(b), expectedIDFoo)
	}

	b, err = os.ReadFile(filepath.Join(credentials.VolumePath, ".ssh", "id_bar"))
	if err != nil {
		t.Fatalf("os.ReadFile(.ssh/id_bar) = %v", err)
	}

	expectedIDBar := `bleh`
	if string(b) != expectedIDBar {
		t.Errorf("got: %v, wanted: %v", string(b), expectedIDBar)
	}

	b, err = os.ReadFile(filepath.Join(credentials.VolumePath, ".ssh", "id_baz"))
	if err != nil {
		t.Fatalf("os.ReadFile(.ssh/id_baz) = %v", err)
	}

	expectedIDBaz := `derp`
	if string(b) != expectedIDBaz {
		t.Errorf("got: %v, wanted: %v", string(b), expectedIDBaz)
	}
}

func TestSSHFlagHandlingMissingFiles(t *testing.T) {
	credentials.VolumePath = t.TempDir()
	dir := credentials.VolumeName("not-found")
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", dir, err)
	}
	// No ssh-privatekey files yields an error.

	cfg := sshGitConfig{entries: make(map[string][]sshEntry)}
	if err := cfg.Set("not-found=github.com"); err == nil {
		t.Error("Set(); got success, wanted error.")
	}
}

func TestBasicMalformedValues(t *testing.T) {
	tests := []string{
		"bar=baz=blah",
		"bar",
	}
	for _, test := range tests {
		cfg := basicGitConfig{}
		if err := cfg.Set(test); err == nil {
			t.Errorf("Set(%v); got success, wanted error.", test)
		}
	}
}

func TestSshMalformedValues(t *testing.T) {
	tests := []string{
		"bar=baz=blah",
		"bar",
	}
	for _, test := range tests {
		cfg := sshGitConfig{}
		if err := cfg.Set(test); err == nil {
			t.Errorf("Set(%v); got success, wanted error.", test)
		}
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
		wantFlag: []string{fmt.Sprintf("-%s=git=basickeys", basicAuthFlag)},
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
		wantFlag: []string{fmt.Sprintf("-%s=ssh=keys", sshFlag)},
	}, {
		secret: &corev1.Secret{
			Type: corev1.SecretTypeSSHAuth,
			ObjectMeta: metav1.ObjectMeta{
				Name: "ssh",
				Annotations: map[string]string{
					fmt.Sprintf("%s.testkeys1", annotationPrefix): "keys1",
					fmt.Sprintf("%s.testkeys2", annotationPrefix): "keys2",
					fmt.Sprintf("%s.testkeys3", annotationPrefix): "keys3",
				},
			},
		},
		wantFlag: []string{fmt.Sprintf("-%s=ssh=keys1", sshFlag), fmt.Sprintf("-%s=ssh=keys2", sshFlag), fmt.Sprintf("-%s=ssh=keys3", sshFlag)},
	}, {
		secret: &corev1.Secret{Type: corev1.SecretTypeOpaque},
	}, {
		secret: &corev1.Secret{Type: corev1.SecretTypeServiceAccountToken},
	}, {
		secret: &corev1.Secret{Type: corev1.SecretTypeDockercfg},
	}, {
		secret: &corev1.Secret{Type: corev1.SecretTypeDockerConfigJson},
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
			t.Errorf("MatchingAnnotations() Mismatch of flags; wanted: %v got: %v", ts.wantFlag, gotFlag)
		}
	}
}

func TestBasicBackslashInUsername(t *testing.T) {
	credentials.VolumePath = t.TempDir()
	dir := credentials.VolumeName("foo")
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatalf("os.MkdirAll(%s) = %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, corev1.BasicAuthUsernameKey), []byte(`foo\bar\banana`), 0777); err != nil {
		t.Fatalf("os.WriteFile(username) = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, corev1.BasicAuthPasswordKey), []byte("baz"), 0777); err != nil {
		t.Fatalf("os.WriteFile(password) = %v", err)
	}

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	AddFlags(fs)
	err := fs.Parse([]string{
		"-basic-git=foo=https://github.com",
	})
	if err != nil {
		t.Fatalf("flag.CommandLine.Parse() = %v", err)
	}

	t.Setenv("HOME", credentials.VolumePath)
	if err := NewBuilder().Write(credentials.VolumePath); err != nil {
		t.Fatalf("Write() = %v", err)
	}

	b, err := os.ReadFile(filepath.Join(credentials.VolumePath, ".gitconfig"))
	if err != nil {
		t.Fatalf("os.ReadFile(.gitconfig) = %v", err)
	}

	expectedGitConfig := `[credential]
	helper = store
[credential "https://github.com"]
	username = foo\\bar\\banana
`
	if string(b) != expectedGitConfig {
		t.Errorf("got: %v, wanted: %v", string(b), expectedGitConfig)
	}

	b, err = os.ReadFile(filepath.Join(credentials.VolumePath, ".git-credentials"))
	if err != nil {
		t.Fatalf("os.ReadFile(.git-credentials) = %v", err)
	}

	expectedGitCredentials := `https://foo%5Cbar%5Cbanana:baz@github.com
`
	if string(b) != expectedGitCredentials {
		t.Errorf("got: %v, wanted: %v", string(b), expectedGitCredentials)
	}
}
