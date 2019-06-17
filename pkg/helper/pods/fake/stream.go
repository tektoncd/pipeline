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

package fake

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/tektoncd/cli/pkg/helper/pods/stream"
	corev1 "k8s.io/api/core/v1"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type PodStream struct {
	logs []Log
	pods typedv1.PodInterface
	name string
	opts *corev1.PodLogOptions
}

func (ps *PodStream) Stream() (io.ReadCloser, error) {
	for _, fl := range ps.logs {
		if fl.PodName != ps.name {
			continue
		}

		for _, c := range fl.Containers {
			if c.Name == ps.opts.Container {
				log := strings.Join(c.Logs, "\n")
				return ioutil.NopCloser(strings.NewReader(log)), nil
			}
		}
	}

	return nil, fmt.Errorf("Failed to stream container logs")
}

func Streamer(l []Log) stream.NewStreamerFunc {
	return func(pods typedv1.PodInterface, name string, opts *corev1.PodLogOptions) stream.Streamer {
		return &PodStream{
			logs: l,
			pods: pods,
			name: name,
			opts: opts,
		}
	}
}
