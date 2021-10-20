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
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/tektoncd/pipeline/pkg/credentials"
)

const annotationPrefix = "tekton.dev/docker-"

var config basicDocker
var dockerConfig arrayArg
var dockerCfg arrayArg

// AddFlags adds CLI flags that dockercreds supports to a given flag.FlagSet.
func AddFlags(flagSet *flag.FlagSet) {
	flags(flagSet)
}

func flags(fs *flag.FlagSet) {
	config = basicDocker{make(map[string]entry)}
	dockerConfig = arrayArg{[]string{}}
	dockerCfg = arrayArg{[]string{}}
	fs.Var(&config, "basic-docker", "List of secret=url pairs.")
	fs.Var(&dockerConfig, "docker-config", "Docker config.json secret file.")
	fs.Var(&dockerCfg, "docker-cfg", "Docker .dockercfg secret file.")
}

// As the flag is read, this status is populated.
// basicDocker implements flag.Value
type basicDocker struct {
	Entries map[string]entry `json:"auths"`
}

func (dc *basicDocker) String() string {
	if dc == nil {
		// According to flag.Value this can happen.
		return ""
	}
	var urls []string
	for k, v := range dc.Entries {
		urls = append(urls, fmt.Sprintf("%s=%s", v.Secret, k))
	}
	return strings.Join(urls, ",")
}

// Set sets a secret for a URL from a value in the format of "secret=url"
func (dc *basicDocker) Set(value string) error {
	parts := strings.Split(value, "=")
	if len(parts) != 2 {
		return fmt.Errorf("expect entries of the form secret=url, got: %v", value)
	}
	secret := parts[0]
	url := parts[1]

	if _, ok := dc.Entries[url]; ok {
		return fmt.Errorf("multiple entries for url: %v", url)
	}

	e, err := newEntry(secret)
	if err != nil {
		return err
	}
	dc.Entries[url] = *e
	return nil
}

type arrayArg struct {
	Values []string
}

// Set adds a value to the arrayArg's value slice
func (aa *arrayArg) Set(value string) error {
	aa.Values = append(aa.Values, value)
	return nil
}

func (aa *arrayArg) String() string {
	return strings.Join(aa.Values, ",")
}

type configFile struct {
	Auth map[string]entry `json:"auths"`
}

type entry struct {
	Secret   string `json:"-"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Auth     string `json:"auth"`
	Email    string `json:"email,omitempty"`
}

func newEntry(secret string) (*entry, error) {
	secretPath := credentials.VolumeName(secret)

	ub, err := ioutil.ReadFile(filepath.Join(secretPath, corev1.BasicAuthUsernameKey))
	if err != nil {
		return nil, err
	}
	username := string(ub)

	pb, err := ioutil.ReadFile(filepath.Join(secretPath, corev1.BasicAuthPasswordKey))
	if err != nil {
		return nil, err
	}
	password := string(pb)

	return &entry{
		Secret:   secret,
		Username: username,
		Password: password,
		Auth:     base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password))),
		Email:    "not@val.id",
	}, nil
}

type basicDockerBuilder struct{}

// NewBuilder returns a new builder for Docker credentials.
func NewBuilder() credentials.Builder { return &basicDockerBuilder{} }

// MatchingAnnotations extracts flags for the credential helper
// from the supplied secret and returns a slice (of length 0 or
// greater) of applicable domains.
func (*basicDockerBuilder) MatchingAnnotations(secret *corev1.Secret) []string {
	var flags []string
	switch secret.Type {
	case corev1.SecretTypeBasicAuth:
		for _, v := range credentials.SortAnnotations(secret.Annotations, annotationPrefix) {
			flags = append(flags, fmt.Sprintf("-basic-docker=%s=%s", secret.Name, v))
		}
	case corev1.SecretTypeDockerConfigJson:
		flags = append(flags, fmt.Sprintf("-docker-config=%s", secret.Name))
	case corev1.SecretTypeDockercfg:
		flags = append(flags, fmt.Sprintf("-docker-cfg=%s", secret.Name))
	default:
		return flags
	}

	return flags
}

// Write builds a .docker/config.json file from a combination
// of kubernetes docker registry secrets and tekton docker
// secret entries and writes it to the given directory. If
// no entries exist then nothing will be written to disk.
func (*basicDockerBuilder) Write(directory string) error {
	dockerDir := filepath.Join(directory, ".docker")
	basicDocker := filepath.Join(dockerDir, "config.json")
	cf := configFile{Auth: config.Entries}
	auth := map[string]entry{}

	for _, secretName := range dockerCfg.Values {
		dockerConfigAuthMap, err := authsFromDockerCfg(secretName)
		if err != nil {
			return err
		}
		for k, v := range dockerConfigAuthMap {
			auth[k] = v
		}
	}

	for _, secretName := range dockerConfig.Values {
		dockerConfigAuthMap, err := authsFromDockerConfig(secretName)
		if err != nil {
			return err
		}
		for k, v := range dockerConfigAuthMap {
			auth[k] = v
		}
	}
	for k, v := range config.Entries {
		auth[k] = v
	}
	if len(auth) == 0 {
		return nil
	}
	if err := os.MkdirAll(dockerDir, os.ModePerm); err != nil {
		return err
	}

	cf.Auth = auth
	content, err := json.Marshal(cf)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(basicDocker, content, 0600)
}

func authsFromDockerCfg(secret string) (map[string]entry, error) {
	secretPath := credentials.VolumeName(secret)
	m := make(map[string]entry)
	data, err := ioutil.ReadFile(filepath.Join(secretPath, corev1.DockerConfigKey))
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(data, &m)
	return m, err
}

func authsFromDockerConfig(secret string) (map[string]entry, error) {
	secretPath := credentials.VolumeName(secret)
	m := make(map[string]entry)
	c := configFile{}
	data, err := ioutil.ReadFile(filepath.Join(secretPath, corev1.DockerConfigJsonKey))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return m, err
	}
	for k, v := range c.Auth {
		m[k] = v
	}
	return m, nil
}
