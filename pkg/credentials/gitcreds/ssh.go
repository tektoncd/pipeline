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
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tektoncd/pipeline/pkg/credentials"
	corev1 "k8s.io/api/core/v1"
)

const sshKnownHosts = "known_hosts"

// As the flag is read, this status is populated.
// sshGitConfig implements flag.Value
type sshGitConfig struct {
	entries map[string][]sshEntry
	// The order we see things, for iterating over the above.
	order []string
}

func (dc *sshGitConfig) String() string {
	if dc == nil {
		// According to flag.Value this can happen.
		return ""
	}
	var urls []string
	for _, k := range dc.order {
		for _, e := range dc.entries[k] {
			urls = append(urls, fmt.Sprintf("%s=%s", e.secretName, k))
		}
	}
	return strings.Join(urls, ",")
}

func (dc *sshGitConfig) Set(value string) error {
	parts := strings.Split(value, "=")
	if len(parts) != 2 {
		return fmt.Errorf("expect entries of the form secret=url, got: %v", value)
	}
	secretName := parts[0]
	url := parts[1]

	e, err := newSSHEntry(url, secretName)
	if err != nil {
		return err
	}
	if _, exists := dc.entries[url]; !exists {
		dc.order = append(dc.order, url)
	}
	dc.entries[url] = append(dc.entries[url], *e)
	return nil
}

func (dc *sshGitConfig) Write() error {
	sshDir := filepath.Join(os.Getenv("HOME"), ".ssh")
	if err := os.MkdirAll(sshDir, os.ModePerm); err != nil {
		return err
	}

	// Walk each of the entries and for each do three things:
	//  1. Write out: ~/.ssh/id_{secretName} with the secret key
	//  2. Compute its part of "~/.ssh/config"
	//  3. Compute its part of "~/.ssh/known_hosts"
	var configEntries []string
	var defaultPort = "22"
	var knownHosts []string
	for _, k := range dc.order {
		var host, port string
		var err error
		if host, port, err = net.SplitHostPort(k); err != nil {
			host = k
			port = defaultPort
		}
		configEntry := fmt.Sprintf(`Host %s
    HostName %s
    Port %s
`, host, host, port)
		for _, e := range dc.entries[k] {
			if err := e.Write(sshDir); err != nil {
				return err
			}
			configEntry += fmt.Sprintf(`    IdentityFile %s
`, e.path(sshDir))
			knownHosts = append(knownHosts, e.knownHosts)
		}
		configEntries = append(configEntries, configEntry)
	}
	configPath := filepath.Join(sshDir, "config")
	configContent := strings.Join(configEntries, "")
	if err := ioutil.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		return err
	}
	knownHostsPath := filepath.Join(sshDir, "known_hosts")
	knownHostsContent := strings.Join(knownHosts, "\n")
	return ioutil.WriteFile(knownHostsPath, []byte(knownHostsContent), 0600)
}

type sshEntry struct {
	secretName string
	privateKey string
	knownHosts string
}

func (be *sshEntry) path(sshDir string) string {
	return filepath.Join(sshDir, "id_"+be.secretName)
}

func sshKeyScan(domain string) ([]byte, error) {
	c := exec.Command("ssh-keyscan", domain)
	var output bytes.Buffer
	c.Stdout = &output
	c.Stderr = &output
	if err := c.Run(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func (be *sshEntry) Write(sshDir string) error {
	return ioutil.WriteFile(be.path(sshDir), []byte(be.privateKey), 0600)
}

func newSSHEntry(u, secretName string) (*sshEntry, error) {
	secretPath := credentials.VolumeName(secretName)

	pk, err := ioutil.ReadFile(filepath.Join(secretPath, corev1.SSHAuthPrivateKey))
	if err != nil {
		return nil, err
	}
	privateKey := string(pk)

	kh, err := ioutil.ReadFile(filepath.Join(secretPath, sshKnownHosts))
	if err != nil {
		kh, err = sshKeyScan(u)
		if err != nil {
			return nil, err
		}
	}
	knownHosts := string(kh)

	return &sshEntry{
		secretName: secretName,
		privateKey: privateKey,
		knownHosts: knownHosts,
	}, nil
}
