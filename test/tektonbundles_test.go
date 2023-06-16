//go:build e2e
// +build e2e

/*
Copyright 2020 The Tekton Authors

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

package test

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/pod"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun"
	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
	"sigs.k8s.io/yaml"
)

var bundleFeatureFlags = requireAnyGate(map[string]string{
	"enable-tekton-oci-bundles": "true",
	"enable-api-fields":         "alpha",
})

var resolverFeatureFlags = requireAllGates(map[string]string{
	"enable-bundles-resolver": "true",
	"enable-api-fields":       "beta",
})

// TestTektonBundlesSimpleWorkingExample is an integration test which tests a simple, working Tekton bundle using OCI
// images.
func TestTektonBundlesSimpleWorkingExample(t *testing.T) {
	ctx := context.Background()
	c, namespace := setup(ctx, t, withRegistry, bundleFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	taskName := helpers.ObjectNameForTest(t)
	pipelineName := helpers.ObjectNameForTest(t)
	pipelineRunName := helpers.ObjectNameForTest(t)
	repo := fmt.Sprintf("%s:5000/tektonbundlessimple", getRegistryServiceIP(ctx, t, c, namespace))
	task := parse.MustParseV1beta1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: hello
    image: alpine
    script: 'echo Hello'
`, taskName, namespace))

	pipeline := parse.MustParseV1beta1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: hello-world
    taskRef:
      name: %s
      bundle: %s
`, pipelineName, namespace, taskName, repo))

	setupBundle(ctx, t, c, namespace, repo, task, pipeline)

	// Now generate a PipelineRun to invoke this pipeline and task.
	pr := parse.MustParseV1beta1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  pipelineRef:
    name: %s
    bundle: %s
`, pipelineRunName, pipelineName, repo))
	if _, err := c.V1beta1PipelineRunClient.Create(ctx, pr, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun: %s", err)
	}

	t.Logf("Waiting for PipelineRun in namespace %s to finish", namespace)
	if err := WaitForPipelineRunState(ctx, c, pipelineRunName, timeout, PipelineRunSucceed(pipelineRunName), "PipelineRunCompleted", v1beta1Version); err != nil {
		t.Errorf("Error waiting for PipelineRun to finish with error: %s", err)
	}

	trs, err := c.V1beta1TaskRunClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Errorf("Error retrieving taskrun: %s", err)
	}
	if len(trs.Items) != 1 {
		t.Fatalf("Expected 1 TaskRun but found %d", len(trs.Items))
	}

	tr := trs.Items[0]
	if tr.Status.GetCondition(apis.ConditionSucceeded).IsFalse() {
		t.Errorf("Expected TaskRun to succeed but instead found condition: %s", tr.Status.GetCondition(apis.ConditionSucceeded))
	}

	if tr.Status.PodName == "" {
		t.Fatal("Error getting a PodName (empty)")
	}
	p, err := c.KubeClient.CoreV1().Pods(namespace).Get(ctx, tr.Status.PodName, metav1.GetOptions{})

	if err != nil {
		t.Fatalf("Error getting pod `%s` in namespace `%s`", tr.Status.PodName, namespace)
	}
	for _, stat := range p.Status.ContainerStatuses {
		if strings.Contains(stat.Name, "step-hello") {
			req := c.KubeClient.CoreV1().Pods(namespace).GetLogs(p.Name, &corev1.PodLogOptions{Container: stat.Name})
			logContent, err := req.Do(ctx).Raw()
			if err != nil {
				t.Fatalf("Error getting pod logs for pod `%s` and container `%s` in namespace `%s`", tr.Status.PodName, stat.Name, namespace)
			}
			if !strings.Contains(string(logContent), "Hello") {
				t.Fatalf("Expected logs to say hello but received %v", logContent)
			}
		}
	}
}

// TestTektonBundlesResolver is an integration test which tests a simple, working Tekton bundle using OCI
// images using the remote resolution bundles resolver.
func TestTektonBundlesResolver(t *testing.T) {
	ctx := context.Background()
	c, namespace := setup(ctx, t, withRegistry, resolverFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	taskName := helpers.ObjectNameForTest(t)
	pipelineName := helpers.ObjectNameForTest(t)
	pipelineRunName := helpers.ObjectNameForTest(t)
	repo := fmt.Sprintf("%s:5000/tektonbundlesresolver", getRegistryServiceIP(ctx, t, c, namespace))

	task := parse.MustParseV1beta1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: hello
    image: alpine
    script: 'echo Hello'
`, taskName, namespace))

	pipeline := parse.MustParseV1beta1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: hello-world
    taskRef:
      resolver: bundles
      params:
      - name: bundle
        value: %s
      - name: name
        value: %s
`, pipelineName, namespace, repo, taskName))

	setupBundle(ctx, t, c, namespace, repo, task, pipeline)

	// Now generate a PipelineRun to invoke this pipeline and task.
	pr := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  pipelineRef:
    resolver: bundles
    params:
    - name: bundle
      value: %s
    - name: name
      value: %s
    - name: kind
      value: pipeline
`, pipelineRunName, repo, pipelineName))
	if _, err := c.V1PipelineRunClient.Create(ctx, pr, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun: %s", err)
	}

	t.Logf("Waiting for PipelineRun in namespace %s to finish", namespace)
	if err := WaitForPipelineRunState(ctx, c, pipelineRunName, timeout, PipelineRunSucceed(pipelineRunName), "PipelineRunCompleted", v1Version); err != nil {
		t.Errorf("Error waiting for PipelineRun to finish with error: %s", err)
	}

	trs, err := c.V1TaskRunClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Errorf("Error retrieving taskrun: %s", err)
	}
	if len(trs.Items) != 1 {
		t.Fatalf("Expected 1 TaskRun but found %d", len(trs.Items))
	}

	tr := trs.Items[0]
	if tr.Status.GetCondition(apis.ConditionSucceeded).IsFalse() {
		t.Errorf("Expected TaskRun to succeed but instead found condition: %s", tr.Status.GetCondition(apis.ConditionSucceeded))
	}

	if tr.Status.PodName == "" {
		t.Fatal("Error getting a PodName (empty)")
	}
	p, err := c.KubeClient.CoreV1().Pods(namespace).Get(ctx, tr.Status.PodName, metav1.GetOptions{})

	if err != nil {
		t.Fatalf("Error getting pod `%s` in namespace `%s`", tr.Status.PodName, namespace)
	}
	for _, stat := range p.Status.ContainerStatuses {
		if strings.Contains(stat.Name, "step-hello") {
			req := c.KubeClient.CoreV1().Pods(namespace).GetLogs(p.Name, &corev1.PodLogOptions{Container: stat.Name})
			logContent, err := req.Do(ctx).Raw()
			if err != nil {
				t.Fatalf("Error getting pod logs for pod `%s` and container `%s` in namespace `%s`", tr.Status.PodName, stat.Name, namespace)
			}
			if !strings.Contains(string(logContent), "Hello") {
				t.Fatalf("Expected logs to say hello but received %v", logContent)
			}
		}
	}
}

// TestTektonBundlesUsingRegularImage is an integration test which passes a non-Tekton bundle as a task reference.
func TestTektonBundlesUsingRegularImage(t *testing.T) {
	ctx := context.Background()
	c, namespace := setup(ctx, t, withRegistry, bundleFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	taskName := helpers.ObjectNameForTest(t)
	pipelineName := helpers.ObjectNameForTest(t)
	pipelineRunName := helpers.ObjectNameForTest(t)
	repo := fmt.Sprintf("%s:5000/tektonbundlesregularimage", getRegistryServiceIP(ctx, t, c, namespace))

	pipeline := parse.MustParseV1beta1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: hello-world
    taskRef:
      name: %s
      bundle: registry
`, pipelineName, namespace, taskName))

	setupBundle(ctx, t, c, namespace, repo, nil, pipeline)

	// Now generate a PipelineRun to invoke this pipeline and task.
	pr := parse.MustParseV1beta1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  pipelineRef:
    name: %s
    bundle: %s
`, pipelineRunName, pipelineName, repo))
	if _, err := c.V1beta1PipelineRunClient.Create(ctx, pr, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun: %s", err)
	}

	t.Logf("Waiting for PipelineRun in namespace %s to finish", namespace)
	if err := WaitForPipelineRunState(ctx, c, pipelineRunName, timeout,
		Chain(
			FailedWithReason(pod.ReasonCouldntGetTask, pipelineRunName),
			FailedWithMessage("does not contain a dev.tekton.image.apiVersion annotation", pipelineRunName),
		), "PipelineRunFailed", v1beta1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun to finish with expected error: %s", err)
	}
}

// TestTektonBundlesUsingImproperFormat is an integration test which passes an improperly formatted Tekton bundle as a
// task reference.
func TestTektonBundlesUsingImproperFormat(t *testing.T) {
	ctx := context.Background()
	c, namespace := setup(ctx, t, withRegistry, bundleFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	taskName := helpers.ObjectNameForTest(t)
	pipelineName := helpers.ObjectNameForTest(t)
	pipelineRunName := helpers.ObjectNameForTest(t)
	repo := fmt.Sprintf("%s:5000/tektonbundlesimproperformat", getRegistryServiceIP(ctx, t, c, namespace))

	ref, err := name.ParseReference(repo)
	if err != nil {
		t.Fatalf("Failed to parse %s as an OCI reference: %s", repo, err)
	}

	task := parse.MustParseV1beta1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: hello
    image: alpine
    script: 'echo Hello'
`, taskName, namespace))

	pipeline := parse.MustParseV1beta1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: hello-world
    taskRef:
      name: %s
      bundle: %s
`, pipelineName, namespace, taskName, repo))

	// Write the pipeline into an image to the registry in the proper format. Write the task using incorrect
	// annotations.
	rawTask, err := yaml.Marshal(task)
	if err != nil {
		t.Fatalf("Failed to marshal task to yaml: %s", err)
	}

	rawPipeline, err := yaml.Marshal(pipeline)
	if err != nil {
		t.Fatalf("Failed to marshal task to yaml: %s", err)
	}

	img := empty.Image
	taskLayer, err := tarball.LayerFromReader(bytes.NewBuffer(rawTask))
	if err != nil {
		t.Fatalf("Failed to create oci layer from task: %s", err)
	}
	pipelineLayer, err := tarball.LayerFromReader(bytes.NewBuffer(rawPipeline))
	if err != nil {
		t.Fatalf("Failed to create oci layer from pipeline: %s", err)
	}
	img, err = mutate.Append(img, mutate.Addendum{
		Layer: taskLayer,
		Annotations: map[string]string{
			// intentionally invalid name annotation
			"org.opencontainers.image.title": taskName,
			"dev.tekton.image.kind":          strings.ToLower(task.Kind),
			"dev.tekton.image.apiVersion":    task.APIVersion,
		},
	}, mutate.Addendum{
		Layer: pipelineLayer,
		Annotations: map[string]string{
			"dev.tekton.image.name":       pipelineName,
			"dev.tekton.image.kind":       strings.ToLower(pipeline.Kind),
			"dev.tekton.image.apiVersion": pipeline.APIVersion,
		},
	})
	if err != nil {
		t.Fatalf("Failed to create an oci image from the task and pipeline layers: %s", err)
	}

	// Publish this image to the in-cluster registry.
	publishImg(ctx, t, c, namespace, img, ref)

	// Now generate a PipelineRun to invoke this pipeline and task.
	pr := parse.MustParseV1beta1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  pipelineRef:
    name: %s
    bundle: %s
`, pipelineRunName, pipelineName, repo))
	if _, err := c.V1beta1PipelineRunClient.Create(ctx, pr, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun: %s", err)
	}

	t.Logf("Waiting for PipelineRun in namespace %s to finish", namespace)
	if err := WaitForPipelineRunState(ctx, c, pipelineRunName, timeout,
		Chain(
			FailedWithReason(pipelinerun.ReasonCouldntGetPipeline, pipelineRunName),
			FailedWithMessage("does not contain a dev.tekton.image.name annotation", pipelineRunName),
		), "PipelineRunFailed", v1beta1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun to finish with expected error: %s", err)
	}
}

func tarImageInOCIFormat(namespace string, img v1.Image) ([]byte, error) {
	// Write the image in the OCI layout and then tar it up.
	dir, err := os.MkdirTemp(os.TempDir(), namespace)
	if err != nil {
		return nil, err
	}

	p, err := layout.Write(dir, empty.Index)
	if err != nil {
		return nil, err
	}

	if err := p.AppendImage(img); err != nil {
		return nil, err
	}

	// Now that the layout is correct, package this up as a tarball.
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		// Generate the initial tar header.
		header, err := tar.FileInfoHeader(info, path)
		if err != nil {
			return err
		}
		// Rewrite the filename with a relative path to the root dir.
		header.Name = strings.Replace(path, dir, "", 1)
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		// If not a dir, write file content as is.
		if !info.IsDir() {
			data, err := os.Open(path)
			if err != nil {
				return err
			}
			if _, err := io.Copy(tw, data); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}

	// Pull out the tar bundle into a bytes object.
	return io.ReadAll(&buf)
}

// publishImg will generate a Pod that runs in the namespace to publish an OCI compliant image into the local registry
// that runs in the cluster. We can't speak to the in-cluster registry from the test so we need to run a Pod to do it
// for us.
func publishImg(ctx context.Context, t *testing.T, c *clients, namespace string, img v1.Image, ref name.Reference) {
	t.Helper()
	podName := "publish-tekton-bundle"

	tb, err := tarImageInOCIFormat(namespace, img)
	if err != nil {
		t.Fatalf("Failed to create OCI tar bundle: %s", err)
	}

	// Create a configmap to contain the tarball which we will mount in the pod.
	cmName := namespace + "uploadimage-cm"
	if _, err = c.KubeClient.CoreV1().ConfigMaps(namespace).Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: cmName},
		BinaryData: map[string][]byte{
			"image.tar": tb,
		},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create configmap to upload image: %s", err)
	}

	po, err := c.KubeClient.CoreV1().Pods(namespace).Create(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    namespace,
			GenerateName: podName,
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{{
				Name: "cm",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: cmName}},
				},
			}, {
				Name:         "scratch",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
			}},
			InitContainers: []corev1.Container{{
				Name:    "untar",
				Image:   "busybox",
				Command: []string{"/bin/sh", "-c"},
				Args:    []string{"mkdir -p /var/image && tar xvf /var/cm/image.tar -C /var/image"},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "cm",
					MountPath: "/var/cm",
				}, {
					Name:      "scratch",
					MountPath: "/var/image",
				}},
			}},
			Containers: []corev1.Container{{
				Name:       "skopeo",
				Image:      "gcr.io/tekton-releases/dogfooding/skopeo:latest",
				WorkingDir: "/var",
				Command:    []string{"/bin/sh", "-c"},
				Args:       []string{"skopeo copy --dest-tls-verify=false oci:image docker://" + ref.String()},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "scratch",
					MountPath: "/var/image",
				}},
			}},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create the skopeo pod: %v", err)
	}
	if err := WaitForPodState(ctx, c, po.Name, namespace, func(pod *corev1.Pod) (bool, error) {
		return pod.Status.Phase == "Succeeded", nil
	}, "PodContainersTerminated"); err != nil {
		req := c.KubeClient.CoreV1().Pods(namespace).GetLogs(po.GetName(), &corev1.PodLogOptions{Container: "skopeo"})
		logs, err := req.DoRaw(ctx)
		if err != nil {
			t.Fatalf("Error waiting for Pod %q to terminate: %v", podName, err)
		} else {
			t.Fatalf("Failed to create image. Pod logs are: \n%s", string(logs))
		}
	}
}

// setupBundle creates an empty image, provides a reference to the fake registry and pushes the
// bundled task and pipeline within an image into the registry
func setupBundle(ctx context.Context, t *testing.T, c *clients, namespace, repo string, task *v1beta1.Task, pipeline *v1beta1.Pipeline) {
	t.Helper()
	img := empty.Image
	ref, err := name.ParseReference(repo)
	if err != nil {
		t.Fatalf("Failed to parse %s as an OCI reference: %s", repo, err)
	}

	var rawTask, rawPipeline []byte
	var taskLayer, pipelineLayer v1.Layer
	// Write the task and pipeline into an image to the registry in the proper format.
	if task != nil {
		rawTask, err = yaml.Marshal(task)
		if err != nil {
			t.Fatalf("Failed to marshal task to yaml: %s", err)
		}
		taskLayer, err = tarball.LayerFromReader(bytes.NewBuffer(rawTask))
		if err != nil {
			t.Fatalf("Failed to create oci layer from task: %s", err)
		}
		img, err = mutate.Append(img, mutate.Addendum{
			Layer: taskLayer,
			Annotations: map[string]string{
				"dev.tekton.image.name":       task.Name,
				"dev.tekton.image.kind":       strings.ToLower(task.Kind),
				"dev.tekton.image.apiVersion": task.APIVersion,
			},
		})
	}

	if pipeline != nil {
		rawPipeline, err = yaml.Marshal(pipeline)
		if err != nil {
			t.Fatalf("Failed to marshal task to yaml: %s", err)
		}
		pipelineLayer, err = tarball.LayerFromReader(bytes.NewBuffer(rawPipeline))
		if err != nil {
			t.Fatalf("Failed to create oci layer from pipeline: %s", err)
		}
		img, err = mutate.Append(img, mutate.Addendum{
			Layer: pipelineLayer,
			Annotations: map[string]string{
				"dev.tekton.image.name":       pipeline.Name,
				"dev.tekton.image.kind":       strings.ToLower(pipeline.Kind),
				"dev.tekton.image.apiVersion": pipeline.APIVersion,
			},
		})
	}

	if err != nil {
		t.Fatalf("Failed to create an oci image from the task and pipeline layers: %s", err)
	}

	// Publish this image to the in-cluster registry.
	publishImg(ctx, t, c, namespace, img, ref)
}
