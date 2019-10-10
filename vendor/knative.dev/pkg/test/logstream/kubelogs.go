/*
Copyright 2019 The Knative Authors

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

package logstream

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/pkg/ptr"
	"knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

type kubelogs struct {
	namespace string

	once sync.Once
	m    sync.RWMutex
	keys map[string]logger
	err  error
}

type logger func(string, ...interface{})

var _ streamer = (*kubelogs)(nil)

// timeFormat defines a simple timestamp with millisecond granularity
const timeFormat = "15:04:05.000"

func (k *kubelogs) init(t *testing.T) {
	k.keys = make(map[string]logger)

	kc, err := test.NewKubeClient(test.Flags.Kubeconfig, test.Flags.Cluster)
	if err != nil {
		t.Errorf("Error loading client config: %v", err)
	}

	// List the pods in the given namespace.
	pl, err := kc.Kube.CoreV1().Pods(k.namespace).List(metav1.ListOptions{})
	if err != nil {
		t.Errorf("Error listing pods: %v", err)
	}

	eg := errgroup.Group{}
	for _, pod := range pl.Items {
		// Grab data from all containers in the pods.  We need this in case
		// an envoy sidecar is injected for mesh installs.  This should be
		// equivalent to --all-containers.
		for _, container := range pod.Spec.Containers {
			// Required for capture below.
			pod, container := pod, container
			eg.Go(func() error {
				options := &corev1.PodLogOptions{
					Container: container.Name,
					// Follow directs the api server to continuously stream logs back.
					Follow: true,
					// Only return new logs (this value is being used for "epsilon").
					SinceSeconds: ptr.Int64(1),
				}

				req := kc.Kube.CoreV1().Pods(k.namespace).GetLogs(pod.Name, options)
				stream, err := req.Stream()
				if err != nil {
					return err
				}
				defer stream.Close()
				// Read this container's stream.
				for scanner := bufio.NewScanner(stream); scanner.Scan(); {
					k.handleLine(scanner.Text())
				}
				return fmt.Errorf("logstream completed prematurely for: %s/%s", pod.Name, container.Name)
			})
		}
	}

	// Monitor the error group in the background and surface an error on the kubelogs
	// in case anything had an active stream open.
	go func() {
		if err := eg.Wait(); err != nil {
			k.m.Lock()
			defer k.m.Unlock()
			k.err = err
		}
	}()
}

func (k *kubelogs) handleLine(l string) {
	// This holds the standard structure of our logs.
	var line struct {
		Level      string    `json:"level"`
		Timestamp  time.Time `json:"ts"`
		Controller string    `json:"knative.dev/controller"`
		Caller     string    `json:"caller"`
		Key        string    `json:"knative.dev/key"`
		Message    string    `json:"msg"`
		Error      string    `json:"error"`

		// TODO(mattmoor): Parse out more context.
	}
	if err := json.Unmarshal([]byte(l), &line); err != nil {
		// Ignore malformed lines.
		return
	}
	if line.Key == "" {
		return
	}

	k.m.RLock()
	defer k.m.RUnlock()

	for name, logf := range k.keys {
		// TODO(mattmoor): Do a slightly smarter match.
		if !strings.Contains(line.Key, name) {
			continue
		}

		// We also get logs not from controllers (activator, autoscaler).
		// So replace controller string in them with their callsite.
		site := line.Controller
		if site == "" {
			site = line.Caller
		}
		// E 15:04:05.000 [route-controller] [default/testroute-xyz] this is my message
		msg := fmt.Sprintf("%s %s [%s] [%s] %s",
			strings.ToUpper(string(line.Level[0])),
			line.Timestamp.Format(timeFormat),
			site,
			line.Key,
			line.Message)

		if line.Error != "" {
			msg += " err=" + line.Error
		}

		logf(msg)
	}
}

// Start implements streamer
func (k *kubelogs) Start(t *testing.T) Canceler {
	k.once.Do(func() { k.init(t) })

	name := helpers.ObjectPrefixForTest(t)

	// Register a key
	k.m.Lock()
	defer k.m.Unlock()
	k.keys[name] = t.Logf

	// Return a function that unregisters that key.
	return func() {
		k.m.Lock()
		defer k.m.Unlock()
		delete(k.keys, name)

		if k.err != nil {
			t.Errorf("error during logstream: %v", k.err)
		}
	}
}
