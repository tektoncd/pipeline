/*
Copyright 2020 The Knative Authors

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
	"reflect"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/ptr"
)

func FromNamespaces(ctx context.Context, c kubernetes.Interface, namespaces []string) Source {
	return &namespaceSource{
		ctx:        ctx,
		kc:         c,
		namespaces: namespaces,
		keys:       make(map[string]Callback, 1),
	}
}

func FromNamespace(ctx context.Context, c kubernetes.Interface, namespace string) Source {
	return &namespaceSource{
		ctx:        ctx,
		kc:         c,
		namespaces: []string{namespace},
		keys:       make(map[string]Callback, 1),
	}
}

type namespaceSource struct {
	namespaces []string
	kc         kubernetes.Interface
	ctx        context.Context

	m        sync.RWMutex
	once     sync.Once
	keys     map[string]Callback
	watchErr error
}

func (s *namespaceSource) StartStream(name string, l Callback) (Canceler, error) {
	s.once.Do(func() { s.watchErr = s.watchPods() })
	if s.watchErr != nil {
		return nil, fmt.Errorf("failed to watch pods in one of the namespace(s) %q: %w", s.namespaces, s.watchErr)
	}

	// Register a key
	s.m.Lock()
	defer s.m.Unlock()
	s.keys[name] = l

	// Return a function that unregisters that key.
	return func() {
		s.m.Lock()
		defer s.m.Unlock()
		delete(s.keys, name)
	}, nil
}

func (s *namespaceSource) watchPods() error {
	for _, ns := range s.namespaces {
		wi, err := s.kc.CoreV1().Pods(ns).Watch(s.ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}

		go func() {
			defer wi.Stop()
			watchedPods := sets.NewString()

			for {
				select {
				case <-s.ctx.Done():
					return
				case ev := <-wi.ResultChan():
					// We have reports of this being randomly nil.
					if ev.Object == nil || reflect.ValueOf(ev.Object).IsNil() {
						continue
					}
					p, ok := ev.Object.(*corev1.Pod)
					if !ok {
						// The Watch interface can return errors via the channel as *metav1.Status.
						// Log those to get notified that loglines might be missing but don't crash.
						s.handleGenericLine([]byte(fmt.Sprintf("unexpected event: %v", p)), "no-pod", "no-container")
						continue
					}
					switch ev.Type {
					case watch.Deleted:
						watchedPods.Delete(p.Name)
					case watch.Added, watch.Modified:
						if !watchedPods.Has(p.Name) && isPodReady(p) {
							watchedPods.Insert(p.Name)
							s.startForPod(p)
						}
					}

				}
			}
		}()
	}

	return nil
}

func (s *namespaceSource) startForPod(pod *corev1.Pod) {
	// Grab data from all containers in the pods.  We need this in case
	// an envoy sidecar is injected for mesh installs.  This should be
	// equivalent to --all-containers.
	for _, container := range pod.Spec.Containers {
		// Required for capture below.
		psn, pn, cn := pod.Namespace, pod.Name, container.Name

		handleLine := s.handleLine
		if wellKnownContainers.Has(cn) {
			// Specialcase logs from chaosduck, queueproxy etc.
			// - ChaosDuck logs enable easy
			//   monitoring of killed pods throughout all tests.
			// - QueueProxy logs enable
			//   debugging troubleshooting data plane request handling issues.
			handleLine = s.handleGenericLine
		}

		go func() {
			options := &corev1.PodLogOptions{
				Container: cn,
				// Follow directs the API server to continuously stream logs back.
				Follow: true,
				// Only return new logs (this value is being used for "epsilon").
				SinceSeconds: ptr.Int64(1),
			}

			req := s.kc.CoreV1().Pods(psn).GetLogs(pn, options)
			stream, err := req.Stream(context.Background())
			if err != nil {
				s.handleGenericLine([]byte(err.Error()), pn, cn)
				return
			}
			defer stream.Close()
			// Read this container's stream.
			for scanner := bufio.NewScanner(stream); scanner.Scan(); {
				handleLine(scanner.Bytes(), pn, cn)
			}
			// Pods get killed with chaos duck, so logs might end
			// before the test does. So don't report an error here.
		}()
	}
}

func isPodReady(p *corev1.Pod) bool {
	if p.Status.Phase == corev1.PodRunning && p.DeletionTimestamp == nil {
		for _, cond := range p.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
				return true
			}
		}
	}
	return false
}

const (
	// timeFormat defines a simple timestamp with millisecond granularity
	timeFormat = "15:04:05.000"
	// ChaosDuck is the well known name for the chaosduck.
	ChaosDuck = "chaosduck"
	// QueueProxy is the well known name for the queueproxy.
	QueueProxy = "queueproxy"
)

// Names of well known containers that do not produce nicely formatted logs that
// could be easily filtered and parsed by handleLine. Logs from these containers
// are captured without filtering.
var wellKnownContainers = sets.NewString(ChaosDuck, QueueProxy)

func (s *namespaceSource) handleLine(l []byte, pod string, _ string) {
	// This holds the standard structure of our logs.
	var line struct {
		Level      string    `json:"severity"`
		Timestamp  time.Time `json:"timestamp"`
		Controller string    `json:"knative.dev/controller"`
		Caller     string    `json:"caller"`
		Key        string    `json:"knative.dev/key"`
		Message    string    `json:"message"`
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

	s.m.RLock()
	defer s.m.RUnlock()

	for name, logf := range s.keys {
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
		func() {
			defer func() {
				if err := recover(); err != nil {
					logf("Invalid log format for pod %s: %s", pod, string(l))
				}
			}()
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
		}()
	}
}

// handleGenericLine prints the given logline to all active tests as it cannot be parsed
// and/or doesn't contain any correlation data (like the chaosduck for example).
func (s *namespaceSource) handleGenericLine(l []byte, pod string, cn string) {
	s.m.RLock()
	defer s.m.RUnlock()

	for _, logf := range s.keys {
		// I 15:04:05.000 webhook-699b7b668d-9smk2 this is my message
		logf("I %s %s %s %s", time.Now().Format(timeFormat), pod, cn, string(l))
	}
}
