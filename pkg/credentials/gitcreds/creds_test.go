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

package gitcreds

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/knative/build-pipeline/pkg/credentials"
	"gotest.tools/assert"
	"gotest.tools/env"
	"gotest.tools/fs"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBasicFlagHandling(t *testing.T) {
	dir := fs.NewDir(t, "", fs.WithDir("foo",
		fs.WithFile(corev1.BasicAuthUsernameKey, "bar"),
		fs.WithFile(corev1.BasicAuthPasswordKey, "baz"),
	))
	defer dir.Remove()
	credentials.VolumePath = dir.Path()

	flagset := flag.NewFlagSet("test", flag.ContinueOnError)
	flags(flagset)
	assert.NilError(t, flagset.Parse([]string{
		"-basic-git=foo=https://github.com",
	}))

	defer env.Patch(t, "HOME", credentials.VolumePath)()
	assert.NilError(t, NewBuilder().Write())

	expectedGitConfig := `[credential]
	helper = store
[credential "https://github.com"]
	username = bar
`
	expectedGitCredentials := `https://bar:baz@github.com
`
	expected := fs.Expected(t,
		fs.WithFile(".gitconfig", expectedGitConfig, fs.MatchAnyFileMode),
		fs.WithFile(".git-credentials", expectedGitCredentials, fs.MatchAnyFileMode),
		fs.MatchExtraFiles,
	)
	assert.Assert(t, fs.Equal(credentials.VolumePath, expected))
}

func TestBasicFlagHandlingTwice(t *testing.T) {
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
	err := flagset.Parse([]string{
		"-basic-git=foo=https://github.com",
		"-basic-git=bar=https://gitlab.com",
	})
	if err != nil {
		t.Fatalf("flag.CommandLine.Parse() = %v", err)
	}

	defer env.Patch(t, "HOME", credentials.VolumePath)()
	assert.NilError(t, NewBuilder().Write())

	expectedGitConfig := `[credential]
	helper = store
[credential "https://github.com"]
	username = asdf
[credential "https://gitlab.com"]
	username = bleh
`
	expectedGitCredentials := `https://asdf:blah@github.com
https://bleh:belch@gitlab.com
`
	expected := fs.Expected(t,
		fs.WithFile(".gitconfig", expectedGitConfig, fs.MatchAnyFileMode),
		fs.WithFile(".git-credentials", expectedGitCredentials, fs.MatchAnyFileMode),
		fs.MatchExtraFiles,
	)
	assert.Assert(t, fs.Equal(credentials.VolumePath, expected))
}

func TestBasicFlagHandlingMissingFiles(t *testing.T) {
	dir := fs.NewDir(t, "", fs.WithDir("not-found"))
	defer dir.Remove()
	credentials.VolumePath = dir.Path()

	// No username / password files yields an error.

	cfg := basicGitConfig{entries: make(map[string]basicEntry)}
	if err := cfg.Set("not-found=https://github.com"); err == nil {
		t.Error("Set(); got success, wanted error.")
	}
}

func TestBasicFlagHandlingURLCollision(t *testing.T) {
	dir := fs.NewDir(t, "", fs.WithDir("foo",
		fs.WithFile(corev1.BasicAuthUsernameKey, "bar"),
		fs.WithFile(corev1.BasicAuthPasswordKey, "baz"),
	))
	defer dir.Remove()
	credentials.VolumePath = dir.Path()

	cfg := basicGitConfig{entries: make(map[string]basicEntry)}
	if err := cfg.Set("foo=https://github.com"); err != nil {
		t.Fatalf("First Set() = %v", err)
	}
	if err := cfg.Set("bar=https://github.com"); err == nil {
		t.Error("Second Set(); got success, wanted error.")
	}
}

func TestSSHFlagHandling(t *testing.T) {
	dir := fs.NewDir(t, "", fs.WithDir("foo",
		fs.WithFile(corev1.SSHAuthPrivateKey, "bar"),
		fs.WithFile("known_hosts", "ssh-rsa blah"),
	))
	defer dir.Remove()
	credentials.VolumePath = dir.Path()

	flagset := flag.NewFlagSet("test", flag.ContinueOnError)
	flags(flagset)
	err := flagset.Parse([]string{
		"-ssh-git=foo=github.com",
	})
	if err != nil {
		t.Fatalf("flag.CommandLine.Parse() = %v", err)
	}

	defer env.Patch(t, "HOME", credentials.VolumePath)()
	assert.NilError(t, NewBuilder().Write())

	expectedSSHConfig := fmt.Sprintf(`Host github.com
    HostName github.com
    IdentityFile %s
`, filepath.Join(os.Getenv("HOME"), ".ssh", "id_foo"))
	expectedSSHKnownHosts := `ssh-rsa blah`
	expectedIDFoo := `bar`
	expected := fs.Expected(t, fs.WithDir(".ssh",
		fs.WithFile("config", expectedSSHConfig, fs.MatchAnyFileMode),
		fs.WithFile("known_hosts", expectedSSHKnownHosts, fs.MatchAnyFileMode),
		fs.WithFile("id_foo", expectedIDFoo, fs.MatchAnyFileMode),
	), fs.MatchExtraFiles)
	assert.Assert(t, fs.Equal(credentials.VolumePath, expected))
}

func TestSSHFlagHandlingTwice(t *testing.T) {
	dir := fs.NewDir(t, "", fs.WithDir("foo",
		fs.WithFile(corev1.SSHAuthPrivateKey, "asdf"),
		fs.WithFile("known_hosts", "ssh-rsa aaaa"),
	), fs.WithDir("bar",
		fs.WithFile(corev1.SSHAuthPrivateKey, "bleh"),
		fs.WithFile("known_hosts", "ssh-rsa bbbb"),
	))
	defer dir.Remove()
	credentials.VolumePath = dir.Path()

	flagset := flag.NewFlagSet("test", flag.ContinueOnError)
	flags(flagset)
	err := flagset.Parse([]string{
		"-ssh-git=foo=github.com",
		"-ssh-git=bar=gitlab.com",
	})
	if err != nil {
		t.Fatalf("flag.CommandLine.Parse() = %v", err)
	}

	defer env.Patch(t, "HOME", credentials.VolumePath)()
	assert.NilError(t, NewBuilder().Write())

	expectedSSHConfig := fmt.Sprintf(`Host github.com
    HostName github.com
    IdentityFile %s
Host gitlab.com
    HostName gitlab.com
    IdentityFile %s
`, filepath.Join(os.Getenv("HOME"), ".ssh", "id_foo"),
		filepath.Join(os.Getenv("HOME"), ".ssh", "id_bar"))
	expectedSSHKnownHosts := `ssh-rsa aaaa
ssh-rsa bbbb`
	expectedIDFoo := `asdf`
	expectedIDBar := `bleh`
	expected := fs.Expected(t, fs.WithDir(".ssh",
		fs.WithFile("config", expectedSSHConfig, fs.MatchAnyFileMode),
		fs.WithFile("known_hosts", expectedSSHKnownHosts, fs.MatchAnyFileMode),
		fs.WithFile("id_foo", expectedIDFoo, fs.MatchAnyFileMode),
		fs.WithFile("id_bar", expectedIDBar, fs.MatchAnyFileMode),
	), fs.MatchExtraFiles)
	assert.Assert(t, fs.Equal(credentials.VolumePath, expected))
}

func TestSSHFlagHandlingMissingFiles(t *testing.T) {
	dir := fs.NewDir(t, "", fs.WithDir("not-found"))
	defer dir.Remove()
	credentials.VolumePath = dir.Path()

	// No ssh-privatekey files yields an error.
	cfg := sshGitConfig{entries: make(map[string]sshEntry)}
	if err := cfg.Set("not-found=github.com"); err == nil {
		t.Error("Set(); got success, wanted error.")
	}
}

func TestSSHFlagHandlingURLCollision(t *testing.T) {
	dir := fs.NewDir(t, "", fs.WithDir("foo",
		fs.WithFile(corev1.SSHAuthPrivateKey, "bar"),
		fs.WithFile("known_hosts", "ssh-rsa blah"),
	))
	defer dir.Remove()
	credentials.VolumePath = dir.Path()

	cfg := sshGitConfig{entries: make(map[string]sshEntry)}
	if err := cfg.Set("foo=github.com"); err != nil {
		t.Fatalf("First Set() = %v", err)
	}
	if err := cfg.Set("bar=github.com"); err == nil {
		t.Error("Second Set(); got success, wanted error.")
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
	}}

	nb := NewBuilder()
	for _, ts := range tests {
		gotFlag := nb.MatchingAnnotations(ts.secret)
		assert.DeepEqual(t, ts.wantFlag, gotFlag)
	}
}
