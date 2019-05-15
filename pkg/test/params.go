// Copyright © 2019 The Tekton Authors.
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

package test

import (
	"github.com/jonboulle/clockwork"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
)

type Params struct {
	ns, kc string
	Client versioned.Interface
	Clock  clockwork.Clock
}

var _ cli.Params = &Params{}

func (p *Params) SetNamespace(ns string) {
	p.ns = ns
}
func (p *Params) Namespace() string {
	return p.ns
}

func (p *Params) SetKubeConfigPath(path string) {
	p.kc = path
}

func (p *Params) KubeConfigPath() string {
	return p.kc
}

func (p *Params) Clientset() (versioned.Interface, error) {
	return p.Client, nil
}

func (p *Params) Time() clockwork.Clock {
	if p.Clock == nil {
		p.Clock = clockwork.NewFakeClock()
	}
	return p.Clock
}
