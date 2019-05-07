// Copyright © 2019 The Knative Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cli

import (
	"os"
	"path/filepath"

	"github.com/jonboulle/clockwork"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"k8s.io/client-go/tools/clientcmd"
)

type TektonParams struct {
	kubeConfigPath string
	clientset      *versioned.Clientset

	namespace string
}

// ensure that TektonParams complies with cli.Params interface
var _ Params = (*TektonParams)(nil)

func (p *TektonParams) SetKubeConfigPath(path string) {
	p.kubeConfigPath = path
}

func (p *TektonParams) Clientset() (versioned.Interface, error) {
	if p.clientset != nil {
		return p.clientset, nil
	}

	kcPath, err := resolveKubeConfigPath(p.kubeConfigPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve kubeconfig path")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kcPath)
	if err != nil {
		return nil, errors.Wrapf(err, "Parsing kubeconfig failed")
	}

	cs, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	p.clientset = cs
	return p.clientset, nil
}

func resolveKubeConfigPath(kcPath string) (string, error) {
	if kcPath != "" {
		return kcPath, nil
	}

	if kubeEnvConf, ok := os.LookupEnv("KUBECONFIG"); ok {
		return kubeEnvConf, nil
	}

	// deduce from ~/.kube/config
	home, err := homedir.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kube", "config"), nil
}

func (p *TektonParams) SetNamespace(ns string) {
	p.namespace = ns
}

func (p *TektonParams) Namespace() string {
	return p.namespace
}


func (p *TektonParams) Time() clockwork.Clock {
	return clockwork.NewRealClock()
}