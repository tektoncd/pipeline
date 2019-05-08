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
	"bufio"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	k8s "k8s.io/client-go/kubernetes"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type PodLogStreamer interface {
	Stream(name string, opts *corev1.PodLogOptions) (io.ReadCloser, error)
}

type LogStreamFunc func(pods typedv1.PodInterface) PodLogStreamer

type PodsStream struct {
	pods typedv1.PodInterface
}

type LogFetcher struct {
	kube     k8s.Interface
	streamer LogStreamFunc
}

type PodLogs struct {
	pod     string
	ns      string
	fetcher *LogFetcher
}

type Streams struct {
	Out io.Writer
	Err io.Writer
}

//Stream Creates a stream object which allows reading the logs
func (p *PodsStream) Stream(name string, opts *corev1.PodLogOptions) (io.ReadCloser, error) {
	return p.pods.GetLogs(name, opts).Stream()
}

func defaultStreamer(pods typedv1.PodInterface) PodLogStreamer {
	return &PodsStream{pods}
}

//DefaultLogFetcher Returns a new instance of LogFetcher
func DefaultLogFetcher(kubeClient k8s.Interface) *LogFetcher {
	return &LogFetcher{
		kube:     kubeClient,
		streamer: defaultStreamer,
	}
}

//NewPodLogs Returns a new instance of PodLog
func NewPodLogs(podName string, namespace string, logfetcher *LogFetcher) *PodLogs {
	return &PodLogs{
		pod:     podName,
		ns:      namespace,
		fetcher: logfetcher,
	}
}

//Fetch streams the logs for given pod container.
//Format callback function allows an API consumer to format the log statement.
// e.g. prefix some string before log statement
func (p *PodLogs) Fetch(s Streams, container string, format func(s string) string) error {
	var (
		kube     = p.fetcher.kube
		streamer = p.fetcher.streamer
		pod      = p.pod
	)

	pods := kube.CoreV1().
		Pods(p.ns)

	if pods == nil {
		return fmt.Errorf("Error in getting pods \n")
	}

	opts := corev1.PodLogOptions{
		Follow:    false,
		Container: container,
	}

	logStream, err := streamer(pods).
		Stream(pod, &opts)
	if err != nil {
		return fmt.Errorf("Error in getting logs for the %s : %s \n", pod, err)
	}
	defer logStream.Close()

	if format == nil {
		format = func(s string) string {
			return s
		}
	}

	r := bufio.NewReader(logStream)
	for {
		line, err := r.ReadString('\n')
		if err == io.EOF {
			if len(line) > 0 {
				fmt.Fprint(s.Out, format(line))
			}
			return nil
		}

		if err != nil {
			return err
		}

		fmt.Fprint(s.Out, format(line))
	}

	return nil
}
