//go:build e2e
// +build e2e

/*
Copyright 2022 The Tekton Authors

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
	"context"
	"crypto"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/pod"
	"github.com/tektoncd/pipeline/test/parse"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/system"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

var (
	neededFeatureFlags = map[string]string{
		// Make sure this is running under alpha integration tests
		"enable-api-fields": "alpha",
	}
	privKey  = "trustedresources-keys/cosign.key"
	pubKey   = "trustedresources-keys/cosign.pub"
	password = "1234"
)

func init() {
	// This is the password for test/trustedresources-keys/cosign.key, only used in this test.
	os.Setenv("PRIVATE_PASSWORD", password)
}

func TestTrustedResourcesVerify_VerificationPolicy_Success(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c, namespace, secretName, signer := setupResourceVerificationConfig(ctx, t, requireAnyGate(neededFeatureFlags))
	knativetest.CleanupOnInterrupt(func() { removeResourceVerificationConfig(ctx, t, c, namespace, secretName) }, t.Logf)
	defer removeResourceVerificationConfig(ctx, t, c, namespace, secretName)

	vp := parse.MustParseVerificationPolicy(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  resources:
    - pattern: ".*"
  authorities:
    - name: key1
      key:
        secretRef:
          name:  %s
          namespace: %s
  mode: enforce
`, helpers.ObjectNameForTest(t), namespace, secretName, namespace))

	if _, err := c.V1alpha1VerificationPolicyClient.Create(ctx, vp, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create VerificationPolicy: %s", err)
	}

	fqImageName := getTestImage(busyboxImage)
	task := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - image: %s
    command: ['/bin/sh']
    args: ['-c', 'echo hello']
`, helpers.ObjectNameForTest(t), namespace, fqImageName))

	signedTask, err := GetSignedV1Task(task, signer, "signedtask")
	if err != nil {
		t.Errorf("error getting signed task: %v", err)
	}
	if _, err := c.V1TaskClient.Create(ctx, signedTask, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}

	pipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: task
    taskRef:
      resolver: cluster
      kind: Task
      params:
      - name: kind
        value: task
      - name: name
        value: %s
      - name: namespace
        value: %s
`, helpers.ObjectNameForTest(t), namespace, signedTask.Name, namespace))

	signedPipeline, err := GetSignedV1Pipeline(pipeline, signer, "signedpipeline")
	if err != nil {
		t.Errorf("error getting signed pipeline: %v", err)
	}

	if _, err := c.V1PipelineClient.Create(ctx, signedPipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline: %s", err)
	}

	pr := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    resolver: cluster
    kind: Pipeline
    params:
    - name: kind
      value: pipeline
    - name: name
      value: %s
    - name: namespace
      value: %s
`, helpers.ObjectNameForTest(t), namespace, signedPipeline.Name, namespace))

	t.Logf("Creating PipelineRun %s", pr.Name)
	if _, err := c.V1PipelineRunClient.Create(ctx, pr, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", pr.Name, err)
	}

	t.Logf("Waiting for PipelineRun in namespace %s to succeed", namespace)
	if err := WaitForPipelineRunState(ctx, c, pr.Name, timeout, PipelineRunSucceed(pr.Name), "PipelineRunSucceed", v1Version); err != nil {
		t.Errorf("Error waiting for PipelineRun to finish: %s", err)
	}

	pr, err = c.V1PipelineRunClient.Get(ctx, pr.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected PipelineRun %s: %s", pr.Name, err)
	}

	if pr.Status.GetCondition(apis.ConditionSucceeded).IsFalse() {
		t.Errorf("Expected PipelineRun to succeed but instead found condition: %s", pr.Status.GetCondition(apis.ConditionSucceeded))
	}
}

func TestTrustedResourcesVerify_VerificationPolicy_Error(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c, namespace, secretName, signer := setupResourceVerificationConfig(ctx, t, requireAnyGate(neededFeatureFlags))
	knativetest.CleanupOnInterrupt(func() { removeResourceVerificationConfig(ctx, t, c, namespace, secretName) }, t.Logf)
	defer removeResourceVerificationConfig(ctx, t, c, namespace, secretName)

	vp := parse.MustParseVerificationPolicy(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  resources:
    - pattern: ".*"
  authorities:
    - name: key1
      key:
        secretRef:
          name:  %s
          namespace: %s
  mode: enforce
`, helpers.ObjectNameForTest(t), namespace, secretName, namespace))

	if _, err := c.V1alpha1VerificationPolicyClient.Create(ctx, vp, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create VerificationPolicy: %s", err)
	}

	fqImageName := getTestImage(busyboxImage)
	task := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - image: %s
    command: ['/bin/sh']
    args: ['-c', 'echo hello']
`, helpers.ObjectNameForTest(t), namespace, fqImageName))

	signedTask, err := GetSignedV1Task(task, signer, "signedtask")
	if err != nil {
		t.Errorf("error getting signed task: %v", err)
	}
	// modify the task to fail the verification
	signedTask.Annotations["foo"] = "bar"
	if _, err := c.V1TaskClient.Create(ctx, signedTask, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}

	pipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: task
    taskRef:
      resolver: cluster
      kind: Task
      params:
      - name: kind
        value: task
      - name: name
        value: %s
      - name: namespace
        value: %s
`, helpers.ObjectNameForTest(t), namespace, signedTask.Name, namespace))

	signedPipeline, err := GetSignedV1Pipeline(pipeline, signer, "signedpipeline")
	if err != nil {
		t.Errorf("error getting signed pipeline: %v", err)
	}

	if _, err := c.V1PipelineClient.Create(ctx, signedPipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline: %s", err)
	}

	pr := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    resolver: cluster
    kind: Pipeline
    params:
    - name: kind
      value: pipeline
    - name: name
      value: %s
    - name: namespace
      value: %s
`, helpers.ObjectNameForTest(t), namespace, signedPipeline.Name, namespace))

	t.Logf("Creating PipelineRun %s", pr.Name)
	if _, err := c.V1PipelineRunClient.Create(ctx, pr, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", pr.Name, err)
	}

	t.Logf("Waiting for PipelineRun in namespace %s to fail", namespace)
	if err := WaitForPipelineRunState(ctx, c, pr.Name, timeout, PipelineRunFailed(pr.Name), "PipelineRunFailed", v1Version); err != nil {
		t.Errorf("Error waiting for PipelineRun to finish: %s", err)
	}

	pr, err = c.V1PipelineRunClient.Get(ctx, pr.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected PipelineRun %s: %s", pr.Name, err)
	}

	if pr.Status.GetCondition(apis.ConditionSucceeded).IsTrue() {
		t.Errorf("Expected PipelineRun to fail but found condition: %s", pr.Status.GetCondition(apis.ConditionSucceeded))
	}
	if pr.Status.Conditions[0].Reason != pod.ReasonResourceVerificationFailed {
		t.Errorf("Expected PipelineRun fail condition is: %s but got: %s", pod.ReasonResourceVerificationFailed, pr.Status.Conditions[0].Reason)
	}
}

func setupResourceVerificationConfig(ctx context.Context, t *testing.T, fn ...func(context.Context, *testing.T, *clients, string)) (*clients, string, string, signature.Signer) {
	t.Helper()
	c, ns := setup(ctx, t, requireAnyGate(neededFeatureFlags))
	secretName, signer := setSecretAndConfig(ctx, t, c.KubeClient, ns)
	return c, ns, secretName, signer
}

func setSecretAndConfig(ctx context.Context, t *testing.T, client kubernetes.Interface, namespace string) (string, signature.Signer) {
	t.Helper()
	// Note that this may not work if we run e2e tests in parallel since this feature flag require all tasks and pipelines
	// to be signed and unsigned resources will fail. i.e. Don't add t.Parallel() for this test.
	configMapData := map[string]string{
		"trusted-resources-verification-no-match-policy": config.FailNoMatchPolicy,
	}
	if err := updateConfigMap(ctx, client, system.Namespace(), config.GetFeatureFlagsConfigName(), configMapData); err != nil {
		t.Fatal(err)
	}

	signer, err := signature.LoadSignerFromPEMFile(privKey, crypto.SHA256, getPass)
	if err != nil {
		t.Errorf("error getting signer from key file: %v", err)
	}

	fileBytes, err := os.ReadFile(filepath.Clean(pubKey))
	if err != nil {
		t.Fatal(err)
	}

	secret := &v1.Secret{Data: map[string][]byte{"cosign.pub": fileBytes}, ObjectMeta: metav1.ObjectMeta{Name: "verification-secrets", Namespace: namespace}}

	client.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	_, err = client.CoreV1().Secrets(namespace).Get(ctx, secret.Name, metav1.GetOptions{})
	if err != nil {
		t.Error(err)
		return "", nil
	}
	return secret.Name, signer
}

func removeResourceVerificationConfig(ctx context.Context, t *testing.T, c *clients, namespace string, secretName string) {
	t.Helper()
	resetSecretAndConfig(ctx, t, c.KubeClient, secretName, namespace)
	tearDown(ctx, t, c, namespace)
}

func resetSecretAndConfig(ctx context.Context, t *testing.T, client kubernetes.Interface, secretName, namespace string) {
	t.Helper()
	configMapData := map[string]string{
		"trusted-resources-verification-no-match-policy": config.IgnoreNoMatchPolicy,
	}
	if err := updateConfigMap(ctx, client, system.Namespace(), config.GetFeatureFlagsConfigName(), configMapData); err != nil {
		t.Fatal(err)
	}
	err := client.CoreV1().Secrets(namespace).Delete(ctx, secretName, metav1.DeleteOptions{})
	if err != nil {
		t.Fatal(err)
	}
}
