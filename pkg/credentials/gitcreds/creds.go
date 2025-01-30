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

	credmatcher "github.com/tektoncd/pipeline/pkg/credentials/matcher"
	credwriter "github.com/tektoncd/pipeline/pkg/credentials/writer"
	corev1 "k8s.io/api/core/v1"
)

const (
	annotationPrefix = "tekton.dev/git-"
	basicAuthFlag    = "basic-git"
	sshFlag          = "ssh-git"
)

var (
	basicConfig basicGitConfig
	sshConfig   sshGitConfig
)

// AddFlags adds CLI flags that gitcreds supports to a given flag.FlagSet.
func AddFlags(flagSet *flag.FlagSet) {
	flags(flagSet)
}

func flags(fs *flag.FlagSet) {
	basicConfig = basicGitConfig{entries: make(map[string]basicEntry)}
	fs.Var(&basicConfig, basicAuthFlag, "List of secret=url pairs.")

	sshConfig = sshGitConfig{entries: make(map[string][]sshEntry)}
	fs.Var(&sshConfig, sshFlag, "List of secret=url pairs.")
}

type gitBuilder struct{}

// NewBuilder returns a new builder for Git credentials.
func NewBuilder() interface {
	credmatcher.Matcher
	credwriter.Writer
} {
	return &gitBuilder{}
}

// MatchingAnnotations extracts flags for the credential helper
// from the supplied secret and returns a slice (of length 0 or
// greater) of applicable domains.
func (*gitBuilder) MatchingAnnotations(secret *corev1.Secret) []string {
	var flags []string
	switch secret.Type {
	case corev1.SecretTypeBasicAuth:
		for _, v := range credwriter.SortAnnotations(secret.Annotations, annotationPrefix) {
			flags = append(flags, fmt.Sprintf("-%s=%s=%s", basicAuthFlag, secret.Name, v))
		}

	case corev1.SecretTypeSSHAuth:
		for _, v := range credwriter.SortAnnotations(secret.Annotations, annotationPrefix) {
			flags = append(flags, fmt.Sprintf("-%s=%s=%s", sshFlag, secret.Name, v))
		}

	case corev1.SecretTypeOpaque, corev1.SecretTypeServiceAccountToken, corev1.SecretTypeDockercfg, corev1.SecretTypeDockerConfigJson, corev1.SecretTypeTLS, corev1.SecretTypeBootstrapToken:
		return flags

	default:
		return flags
	}
	return flags
}

func (*gitBuilder) Write(directory string) error {
	if err := basicConfig.Write(directory); err != nil {
		return err
	}
	return sshConfig.Write(directory)
}
