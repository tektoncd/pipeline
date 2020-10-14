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
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"

	"knative.dev/pkg/ptr"
	"knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

type kubelogs struct {
	namespace string
	kc        *test.KubeClient

	once sync.Once
	m    sync.RWMutex
	keys map[string]logger
}

type logger func(string, ...interface{})

var _ streamer = (*kubelogs)(nil)

// timeFormat defines a simple timestamp with millisecond granularity
const timeFormat = "15:04:05.000"

func (k *kubelogs) startForPod(pod *corev1.Pod) {
	// Grab data from all containers in the pods.  We need this in case
	// an envoy sidecar is injected for mesh installs.  This should be
	// equivalent to --all-containers.
	for _, container := range pod.Spec.Containers {
		// Required for capture below.
		psn, pn, cn := pod.Namespace, pod.Name, container.Name

		handleLine := k.handleLine
		if cn == "chaosduck" {
			// Specialcase logs from chaosduck to be able to easily see when pods
			// have been killed throughout all tests.
			handleLine = k.handleGenericLine
		}

		go func() {
			options := &corev1.PodLogOptions{
				Container: cn,
				// Follow directs the API server to continuously stream logs back.
				Follow: true,
				// Only return new logs (this value is being used for "epsilon").
				SinceSeconds: ptr.Int64(1),
			}

			req := k.kc.Kube.CoreV1().Pods(psn).GetLogs(pn, options)
			stream, err := req.Stream(context.Background())
			if err != nil {
				k.handleGenericLine([]byte(err.Error()), pn)
				return
			}
			defer stream.Close()
			// Read this container's stream.
			scanner := bufio.NewScanner(stream)
			for scanner.Scan() {
				handleLine(scanner.Bytes(), pn)
			}
			// Pods get killed with chaos duck, so logs might end
			// before the test does. So don't report an error here.
		}()
	}
}

func podIsReady(p *corev1.Pod) bool {
	if p.Status.Phase == corev1.PodRunning && p.DeletionTimestamp == nil {
		for _, cond := range p.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
				return true
			}
		}
	}
	return false
}

func (k *kubelogs) watchPods(t test.TLegacy) {
	wi, err := k.kc.Kube.CoreV1().Pods(k.namespace).Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Error("Logstream knative pod watch failed, logs might be missing", "error", err)
		return
	}
	go func() {
		watchedPods := sets.NewString()
		for ev := range wi.ResultChan() {
			p := ev.Object.(*corev1.Pod)
			switch ev.Type {
			case watch.Deleted:
				watchedPods.Delete(p.Name)
			case watch.Added, watch.Modified:
				if watchedPods.Has(p.Name) {
					continue
				}
				if podIsReady(p) {
					watchedPods.Insert(p.Name)
					k.startForPod(p)
					continue
				}
			}
		}
	}()
}

func (k *kubelogs) init(t test.TLegacy) {
	k.keys = make(map[string]logger, 1)

	kc, err := test.NewKubeClient(test.Flags.Kubeconfig, test.Flags.Cluster)
	if err != nil {
		t.Error("Error loading client config", "error", err)
		return
	}
	k.kc = kc

	// watchPods will start logging for existing pods as well.
	k.watchPods(t)
}

func (k *kubelogs) handleLine(l []byte, pod string) {
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
	if err := json.Unmarshal(l, &line); err != nil {
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
		if !strings.Contains(line.Key, "/"+name) {
			continue
		}

		// We also get logs not from controllers (activator, autoscaler).
		// So replace controller string in them with their callsite.
		site := line.Controller
		if site == "" {
			site = line.Caller
		}
		// E 15:04:05.000 webhook-699b7b668d-9smk2 [route-controller] [default/testroute-xyz] this is my message
		msg := fmt.Sprintf("%s %s %s [%s] [%s] %s",
			strings.ToUpper(string(line.Level[0])),
			line.Timestamp.Format(timeFormat),
			pod,
			site,
			line.Key,
			line.Message)

		if line.Error != "" {
			msg += " err=" + line.Error
		}

		logf(msg)
	}
}

// handleGenericLine prints the given logline to all active tests as it cannot be parsed
// and/or doesn't contain any correlation data (like the chaosduck for example).
func (k *kubelogs) handleGenericLine(l []byte, pod string) {
	k.m.RLock()
	defer k.m.RUnlock()

	for _, logf := range k.keys {
		// I 15:04:05.000 webhook-699b7b668d-9smk2 this is my message
		logf("I %s %s %s", time.Now().Format(timeFormat), pod, string(l))
	}
}

// Start implements streamer.
func (k *kubelogs) Start(t test.TLegacy) Canceler {
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
	}
}
