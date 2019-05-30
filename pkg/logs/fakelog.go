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

package logs

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	corev1 "k8s.io/api/core/v1"
	k8s "k8s.io/client-go/kubernetes"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type logs []task

type step struct {
	name      string
	container string
	logs      []string
}

type task struct {
	name  string
	pod   string
	steps []step
}

type FakePodStream struct {
	logs logs
	pods typedv1.PodInterface
}

func FakeLogFetcher(kubeClient k8s.Interface, fakeStreamer FakePodStream) *LogFetcher {
	return &LogFetcher{
		kube: kubeClient,
		streamer: func(pods typedv1.PodInterface) PodLogStreamer {
			fakeStreamer.pods = pods
			return &fakeStreamer
		},
	}
}

func (p *FakePodStream) Stream(name string, opts *corev1.PodLogOptions) (io.ReadCloser, error) {
	for _, t := range p.logs {
		if t.pod != name {
			continue
		}

		for _, s := range t.steps {
			if s.container == opts.Container {
				log := strings.Join(s.logs, "\n")
				return ioutil.NopCloser(strings.NewReader(log)), nil
			}
		}
	}

	return nil, fmt.Errorf("Failed to stream container logs \n")
}

func Pipeline(pl ...task) FakePodStream {
	logs := logs{}

	for _, t := range pl {
		logs = append(logs, t)
	}

	return FakePodStream{
		logs: logs,
	}
}

func Task(name string, podName string, steps ...step) task {
	return task{
		name:  name,
		pod:   podName,
		steps: steps,
	}
}

func Step(name string, container string, logs ...string) step {
	return step{
		name:      name,
		container: container,
		logs:      logs,
	}
}

func TaskRun(ts ...task) FakePodStream {
	logs := logs{}

	for _, t := range ts {
		logs = append(logs, t)
	}

	return FakePodStream{
		logs: logs,
	}
}
