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
	"k8s.io/client-go/rest"
	"os"
	"path/filepath"

	"github.com/jonboulle/clockwork"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	DEFAUL_CONFIG_DIR   = ".kube"
	DEFAULT_CONFIG_FILE = "config"
)

type TektonParams struct {
	clients        *Clients
	kubeConfigPath string
	namespace      string
}

// ensure that TektonParams complies with cli.Params interface
var _ Params = (*TektonParams)(nil)

func (p *TektonParams) SetKubeConfigPath(path string) {
	p.kubeConfigPath = path
}

func (p *TektonParams) tektonClient(config *rest.Config) (versioned.Interface, error) {
	cs, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return cs, nil
}

func (p *TektonParams) kubeClient(config *rest.Config) (k8s.Interface, error) {
	k8scs, err := k8s.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to create ks8 client from config")
	}

	return k8scs, nil
}

func (p *TektonParams) Clients() (*Clients, error) {
	if p.clients != nil {
		return p.clients, nil
	}

	config, err := p.config()
	if err != nil {
		return nil, err
	}

	tekton, err := p.tektonClient(config)
	if err != nil {
		return nil, err
	}

	kube, err := p.kubeClient(config)
	if err != nil {
		return nil, err
	}

	p.clients = &Clients{
		Tekton: tekton,
		Kube:   kube,
	}

	return p.clients, nil
}

func (p *TektonParams) config() (*rest.Config, error) {
	kcPath, err := p.resolveKubeConfigPath()

	if err != nil {
		return nil, errors.Wrap(err, "Failed to resolve kubeconfig path")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kcPath)
	if err != nil {
		return nil, errors.Wrapf(err, "Parsing kubeconfig failed")
	}
	return config, err
}

func (p *TektonParams) resolveKubeConfigPath() (string, error) {
	if p.kubeConfigPath != "" {
		return p.kubeConfigPath, nil
	}

	if kubeEnvConf, ok := os.LookupEnv("KUBECONFIG"); ok {
		return kubeEnvConf, nil
	}

	home, err := homedir.Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, DEFAUL_CONFIG_DIR, DEFAULT_CONFIG_FILE), nil
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
