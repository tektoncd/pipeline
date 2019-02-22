/*
Copyright 2017 Google Inc. All Rights Reserved.
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

package logs

import (
	"bufio"
	"context"
	"fmt"
	"io"

	TektonV1alpha1 "github.com/knative/build-pipeline/pkg/client/clientset/versioned/typed/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/test/logs/color"
	"github.com/knative/build-pipeline/test/logs/pod"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"k8s.io/client-go/rest"
)

/*
TailLogs tails the logs for task run object with name `name` in namespace `namespace`
into `out` Writer.

ctx is used to carry cancellation of command. cfg is the kubernetes REST configuration
used to create pipeline client.
*/
func TailLogs(ctx context.Context, cfg *rest.Config, name, namespace string, out io.Writer) error {
	pclient, err := TektonV1alpha1.NewForConfig(cfg)
	if err != nil {
		return err
	}
	tr, err := pclient.TaskRuns(namespace).Get(name, metav1.GetOptions{IncludeUninitialized: true})
	if err != nil {
		return err
	}

	client, err := corev1.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("getting corev1 kubernetes client: %v", err)
	}

	podName := tr.Status.PodName
	pods := client.Pods(namespace)
	watcher := pod.Watcher{
		Pods: pods,
		Name: podName,
	}
	if err := watcher.Start(ctx); err != nil {
		return fmt.Errorf("watching pod: %v", err)
	}

	pod, err := watcher.WaitForPod(ctx, func(p *v1.Pod) bool {
		return len(p.Status.InitContainerStatuses) > 0
	})
	if err != nil {
		return err
	}

	for i, container := range pod.Status.InitContainerStatuses {
		pod, err := watcher.WaitForPod(ctx, func(p *v1.Pod) bool {
			waiting := p.Status.InitContainerStatuses[i].State.Waiting
			if waiting == nil {
				return true
			}

			if waiting.Message != "" {
				fmt.Fprintln(out, color.Red(fmt.Sprintf("[%s] %s", container.Name, waiting.Message)))
			}

			return false
		})
		if err != nil {
			return fmt.Errorf("waiting for container: %v", err)
		}

		container := pod.Status.InitContainerStatuses[i]
		followContainer := container.State.Terminated == nil
		if err := printContainerLogs(ctx, out, pods, podName, container.Name, followContainer, name); err != nil {
			return fmt.Errorf("printing logs: %v", err)
		}

		pod, err = watcher.WaitForPod(ctx, func(p *v1.Pod) bool {
			return p.Status.InitContainerStatuses[i].State.Terminated != nil
		})
		if err != nil {
			return fmt.Errorf("waiting for container termination: %v", err)
		}

		container = pod.Status.InitContainerStatuses[i]
		terminated := container.State.Terminated
		if terminated.ExitCode != 0 {
			message := "Build Failed"
			if terminated.Message != "" {
				message += ": " + terminated.Message
			}

			fmt.Fprintln(out, color.Red(fmt.Sprintf("[%s] %s", container.Name, message)))
			return nil
		}
	}
	return nil
}

func printContainerLogs(ctx context.Context, out io.Writer, pods corev1.PodExpansion, podName, containerName string, follow bool, name string) error {
	rc, err := pods.GetLogs(podName, &v1.PodLogOptions{
		Container: containerName,
		Follow:    follow,
	}).Stream()
	if err != nil {
		return err
	}
	defer rc.Close()

	return streamLogs(ctx, out, containerName, rc, name)
}

func streamLogs(ctx context.Context, out io.Writer, containerName string, rc io.Reader, name string) error {
	prefix := color.Blue(fmt.Sprintf("[%s]", name)) + " " + color.Green(fmt.Sprintf("[%s]", containerName)) + " "

	r := bufio.NewReader(rc)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		line, err := r.ReadBytes('\n')
		if err == io.EOF {
			if len(line) > 0 {
				fmt.Fprintf(out, "%s%s\n", prefix, line)
			}
			return nil
		}
		if err != nil {
			return err
		}

		fmt.Fprintf(out, "%s%s", prefix, line)
	}
}
