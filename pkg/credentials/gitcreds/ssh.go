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
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tektoncd/pipeline/pkg/credentials/common"
	credmatcher "github.com/tektoncd/pipeline/pkg/credentials/matcher"
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

// Set sets a secret for a given URL from a "secret=url" value.
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

// Write puts dc's ssh entries into files in a .ssh directory, under
// the given directory. If dc has no entries then nothing is written.
//
// When an annotation URL includes a repo path (e.g., github.com/org/repo),
// an SSH Host alias is created and a .gitconfig insteadOf entry is written
// so that Git maps the repo URL to the correct SSH key. Host-only entries
// are written first (as fallbacks), repo-specific entries after.
func (dc *sshGitConfig) Write(directory string) error {
	if len(dc.entries) == 0 {
		return nil
	}
	sshDir := filepath.Join(directory, ".ssh")
	if err := os.MkdirAll(sshDir, os.ModePerm); err != nil {
		return err
	}

	sortedOrder := dc.sortedOrder()

	var configEntries []string
	var knownHosts []string
	var gitConfigEntries []string
	for _, k := range sortedOrder {
		host, port, repoPath := parseSSHURL(k)

		var sshHost string
		if repoPath != "" {
			sshHost = sshHostAlias(host, repoPath)
			gitConfigEntries = append(gitConfigEntries, insteadOfEntries(sshHost, host, port, repoPath)...)
		} else {
			sshHost = host
		}

		var configEntry strings.Builder
		fmt.Fprintf(&configEntry, "Host %s\n    HostName %s\n    Port %s\n", sshHost, host, port)
		for _, e := range dc.entries[k] {
			if err := e.Write(sshDir); err != nil {
				return err
			}
			fmt.Fprintf(&configEntry, "    IdentityFile %s\n", e.path(sshDir))
			if e.knownHosts != "" {
				knownHosts = append(knownHosts, e.knownHosts)
			}
		}
		configEntries = append(configEntries, configEntry.String())
	}
	configPath := filepath.Join(sshDir, "config")
	configContent := strings.Join(configEntries, "")
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		return err
	}
	if len(knownHosts) > 0 {
		knownHostsPath := filepath.Join(sshDir, "known_hosts")
		knownHostsContent := strings.Join(knownHosts, "\n")
		if err := os.WriteFile(knownHostsPath, []byte(knownHostsContent), 0600); err != nil {
			return err
		}
	}

	if len(gitConfigEntries) > 0 {
		gitConfigPath := filepath.Join(directory, ".gitconfig")
		gitConfigContent := strings.Join(gitConfigEntries, "")
		f, err := os.OpenFile(gitConfigPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := f.WriteString(gitConfigContent); err != nil {
			return err
		}
	}

	return nil
}

// sortedOrder returns dc.order sorted so that host-only entries come before
// repo-specific entries. Within each group the original insertion order is
// preserved. This ensures host-wide SSH keys act as fallbacks.
func (dc *sshGitConfig) sortedOrder() []string {
	sorted := make([]string, len(dc.order))
	copy(sorted, dc.order)
	sort.SliceStable(sorted, func(i, j int) bool {
		_, _, pi := parseSSHURL(sorted[i])
		_, _, pj := parseSSHURL(sorted[j])
		if (pi == "") != (pj == "") {
			return pi == ""
		}
		return false
	})
	return sorted
}

// parseSSHURL splits a raw SSH annotation value into host, port, and an
// optional repo path. Examples:
//
//	"github.com"              → ("github.com", "22", "")
//	"github.com:2222"         → ("github.com", "2222", "")
//	"github.com/org/repo"     → ("github.com", "22", "/org/repo")
//	"github.com:2222/org/repo"→ ("github.com", "2222", "/org/repo")
func parseSSHURL(raw string) (host, port, path string) {
	const defaultPort = "22"

	if idx := strings.Index(raw, "/"); idx != -1 {
		path = raw[idx:]
		raw = raw[:idx]
	}

	h, p, err := net.SplitHostPort(raw)
	if err == nil {
		host = h
		port = p
	} else {
		host = raw
		port = defaultPort
	}

	return
}

// sshHostAlias builds a deterministic SSH Host alias from the hostname and
// repo path. Double-dash separators prevent collisions when repo names
// contain hyphens (e.g. "org/my-repo" vs "org-my/repo").
// For example ("github.com", "/org/repo") → "github.com--org--repo".
func sshHostAlias(host, repoPath string) string {
	sanitized := strings.TrimPrefix(repoPath, "/")
	sanitized = strings.ReplaceAll(sanitized, "/", "--")
	return host + "--" + sanitized
}

// insteadOfEntries returns .gitconfig url.*.insteadOf blocks that redirect
// Git SSH URLs through the host alias so that the correct SSH key is used.
// Both SCP-style (git@host:path) and SSH URL-style (ssh://host/path) are
// handled. A ".git" suffix is appended to insteadOf values so that Git's
// prefix-based matching does not accidentally rewrite URLs for repos that
// share a common prefix (e.g. org/repo vs org/repo-other).
func insteadOfEntries(alias, host, port, repoPath string) []string {
	const defaultPort = "22"
	pathTrimmed := strings.TrimPrefix(repoPath, "/")

	entries := []string{
		fmt.Sprintf("[url \"git@%s:%s.git\"]\n\tinsteadOf = git@%s:%s.git\n", alias, pathTrimmed, host, pathTrimmed),
	}

	if port != defaultPort {
		entries = append(entries,
			fmt.Sprintf("[url \"ssh://%s/%s.git\"]\n\tinsteadOf = ssh://%s:%s/%s.git\n", alias, pathTrimmed, host, port, pathTrimmed),
		)
	} else {
		entries = append(entries,
			fmt.Sprintf("[url \"ssh://%s/%s.git\"]\n\tinsteadOf = ssh://%s/%s.git\n", alias, pathTrimmed, host, pathTrimmed),
		)
	}

	return entries
}

type sshEntry struct {
	secretName string
	privateKey string
	knownHosts string
}

func (be *sshEntry) path(sshDir string) string {
	return filepath.Join(sshDir, "id_"+be.secretName)
}

func (be *sshEntry) Write(sshDir string) error {
	return os.WriteFile(be.path(sshDir), []byte(be.privateKey), 0600)
}

func newSSHEntry(url, secretName string) (*sshEntry, error) {
	secretPath := credmatcher.VolumeName(secretName)

	pk, err := os.ReadFile(filepath.Join(secretPath, common.SSHAuthPrivateKey))
	if err != nil {
		return nil, err
	}
	privateKey := string(pk)

	knownHosts := ""
	if kh, err := os.ReadFile(filepath.Join(secretPath, sshKnownHosts)); err == nil {
		knownHosts = string(kh)
	}

	return &sshEntry{
		secretName: secretName,
		privateKey: privateKey,
		knownHosts: knownHosts,
	}, nil
}
