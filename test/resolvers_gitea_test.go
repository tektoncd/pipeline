//go:build e2e && gitea

/*
 Copyright 2025 The Tekton Authors

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
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/sdk/gitea"
	"github.com/goccy/kpoward"
	"github.com/jenkins-x/go-scm/scm/factory"
	resolverconfig "github.com/tektoncd/pipeline/pkg/apis/config/resolver"
	gitresolution "github.com/tektoncd/pipeline/pkg/resolution/resolver/git"
	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

// @test:execution=parallel
func TestGitResolver_HTTPAuth(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, gitFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	giteaClusterHostname, tokenSecretName := setupGitea(ctx, t, c, namespace)

	requestUrl := fmt.Sprintf("http://%s/%s/%s", net.JoinHostPort(giteaClusterHostname, "3000"), scmRemoteOrg, scmRemoteRepo)

	trName := helpers.ObjectNameForTest(t)
	tr := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    resolver: git
    params:
    - name: url
      value: %s
    - name: revision
      value: %s
    - name: pathInRepo
      value: %s
    - name: gitToken
      value: %s
    - name: gitTokenKey
      value: %s
`,
		trName,
		namespace,
		requestUrl,
		scmRemoteBranch,
		scmRemoteTaskPath,
		tokenSecretName,
		scmTokenSecretKey,
	))

	_, err := c.V1TaskRunClient.Create(ctx, tr, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create TaskRun: %v", err)
	}

	t.Logf("Waiting for TaskRun %s in namespace %s to complete", trName, namespace)
	if err := WaitForTaskRunState(ctx, c, trName, TaskRunSucceed(trName), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for TaskRun %s to finish: %s", trName, err)
	}
}

// @test:execution=parallel
func TestGitResolver_API(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, gitFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	giteaClusterHostname, tokenSecretName := setupGitea(ctx, t, c, namespace)

	resovlerNS := resolverconfig.ResolversNamespace(systemNamespace)

	originalConfigMap, err := c.KubeClient.CoreV1().ConfigMaps(resovlerNS).Get(ctx, gitresolution.ConfigMapName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get ConfigMap `%s`: %s", gitresolution.ConfigMapName, err)
	}
	originalConfigMapData := originalConfigMap.Data

	t.Logf("Creating ConfigMap %s", gitresolution.ConfigMapName)
	configMapData := map[string]string{
		gitresolution.ServerURLKey:          fmt.Sprint("http://", net.JoinHostPort(giteaClusterHostname, "3000")),
		gitresolution.SCMTypeKey:            "gitea",
		gitresolution.APISecretNameKey:      tokenSecretName,
		gitresolution.APISecretKeyKey:       scmTokenSecretKey,
		gitresolution.APISecretNamespaceKey: namespace,
	}
	if err := updateConfigMap(ctx, c.KubeClient, resovlerNS, gitresolution.ConfigMapName, configMapData); err != nil {
		t.Fatal(err)
	}
	defer resetConfigMap(ctx, t, c, resovlerNS, gitresolution.ConfigMapName, originalConfigMapData)

	trName := helpers.ObjectNameForTest(t)
	tr := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    resolver: git
    params:
    - name: revision
      value: %s
    - name: pathInRepo
      value: %s
    - name: org
      value: %s
    - name: repo
      value: %s
`, trName, namespace, scmRemoteBranch, scmRemoteTaskPath, scmRemoteOrg, scmRemoteRepo))

	_, err = c.V1TaskRunClient.Create(ctx, tr, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create TaskRun: %v", err)
	}

	t.Logf("Waiting for TaskRun %s in namespace %s to complete", trName, namespace)
	if err := WaitForTaskRunState(ctx, c, trName, TaskRunSucceed(trName), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for TaskRun %s to finish: %s", trName, err)
	}
}

// @test:execution=parallel
func TestGitResolver_API_Identifier(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, gitFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	giteaClusterHostname, tokenSecretName := setupGitea(ctx, t, c, namespace)

	resovlerNS := resolverconfig.ResolversNamespace(systemNamespace)

	originalConfigMap, err := c.KubeClient.CoreV1().ConfigMaps(resovlerNS).Get(ctx, gitresolution.ConfigMapName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get ConfigMap `%s`: %s", gitresolution.ConfigMapName, err)
	}
	originalConfigMapData := originalConfigMap.Data

	t.Logf("Creating ConfigMap %s", gitresolution.ConfigMapName)
	configMapData := map[string]string{
		"test." + gitresolution.ServerURLKey:          fmt.Sprint("http://", net.JoinHostPort(giteaClusterHostname, "3000")),
		"test." + gitresolution.SCMTypeKey:            "gitea",
		"test." + gitresolution.APISecretNameKey:      tokenSecretName,
		"test." + gitresolution.APISecretKeyKey:       scmTokenSecretKey,
		"test." + gitresolution.APISecretNamespaceKey: namespace,
	}
	if err := updateConfigMap(ctx, c.KubeClient, resovlerNS, gitresolution.ConfigMapName, configMapData); err != nil {
		t.Fatal(err)
	}
	defer resetConfigMap(ctx, t, c, resovlerNS, gitresolution.ConfigMapName, originalConfigMapData)

	trName := helpers.ObjectNameForTest(t)
	tr := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    resolver: git
    params:
    - name: revision
      value: %s
    - name: pathInRepo
      value: %s
    - name: org
      value: %s
    - name: repo
      value: %s
    - name: configKey
      value: test
`, trName, namespace, scmRemoteBranch, scmRemoteTaskPath, scmRemoteOrg, scmRemoteRepo))

	_, err = c.V1TaskRunClient.Create(ctx, tr, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create TaskRun: %v", err)
	}

	t.Logf("Waiting for TaskRun %s in namespace %s to complete", trName, namespace)
	if err := WaitForTaskRunState(ctx, c, trName, TaskRunSucceed(trName), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for TaskRun %s to finish: %s", trName, err)
	}
}

// setupGitea reads git-resolver/gitea.yaml, replaces "default" namespace references in "namespace: default" and
// svc.cluster.local hostnames with the test namespace, calls kubectl create, and waits for the gitea-0 pod to be up
// and running. At that point, it'll create a test user and token, create a Secret containing that token, create an org
// and repository in gitea, add the test task to that repo, and verify that it's been created properly, returning the
// Gitea service's internal-to-cluster hostname and the token secret name.
// Note that gitea.yaml is generated from https://gitea.com/gitea/helm-chart/, via "helm template gitea gitea-charts/gitea",
// then removing the "gitea-test-connection" pod included in the output, because that never seemed to actually work and
// just confused things.
func setupGitea(ctx context.Context, t *testing.T, c *clients, namespace string) (string, string) {
	t.Helper()
	giteaYaml, err := os.ReadFile(filepath.Join("git-resolver", "gitea.yaml"))
	if err != nil {
		t.Fatalf("failed to read gitea.yaml: %v", err)
	}

	// Replace any "namespace: default"s with the test namespace.
	giteaYaml = defaultNamespaceRE.ReplaceAll(giteaYaml, []byte("namespace: "+namespace))
	// Replace any ".default.svc.cluster"s with ".(test namespace).svc.cluster"s.
	giteaYaml = defaultSvcRE.ReplaceAll(giteaYaml, []byte(fmt.Sprintf(".%s.svc.cluster", namespace)))

	giteaInternalHostname := fmt.Sprintf("gitea-http.%s.svc.cluster.local", namespace)

	kcOutput, err := kubectlCreate(giteaYaml, namespace)
	if err != nil {
		t.Logf("failed 'kubectl create' output: %s", string(kcOutput))
		t.Fatalf("failed to 'kubectl create' for gitea: %v", err)
	}

	// Sleep 5 seconds to make sure the pod gets created, then wait for it to be running. It'll take over 30s, due to
	// waiting for the postgres and memcached pods it depends on to be running.
	time.Sleep(5 * time.Second)
	if err := WaitForPodState(ctx, c, "gitea-0", namespace, func(r *corev1.Pod) (bool, error) {
		if r.Status.Phase == corev1.PodRunning {
			for _, cs := range r.Status.ContainerStatuses {
				return cs.Name == "gitea" && cs.State.Running != nil && cs.Ready, nil
			}
		}
		return false, nil
	}, "PodRunning"); err != nil {
		t.Fatalf("Error waiting for gitea-0 pod to be running: %v", err)
	}

	giteaUserJSON := fmt.Sprintf(`{"admin":true,"email":"%s@example.com","full_name":"%s","login_name":"%s","must_change_password":false,"password":"%s","send_notify":false,"source_id":0,"username":"%s"}`, scmRemoteUser, scmRemoteUser, scmRemoteUser, scmRemoteUserPassword, scmRemoteUser)

	trName := helpers.AppendRandomString("git-resolver-setup-gitea-user")
	giteaConfigTaskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskSpec:
    results:
    - name: token
      type: string
    steps:
    - image: docker.io/alpine/curl
      script: |
        #!/bin/ash
        curl -X POST "http://gitea_admin:%s@%s:3000/api/v1/admin/users" -H "accept: application/json" -H "Content-Type: application/json" -d '%s'
        curl -X PATCH "http://gitea_admin:%s@%s:3000/api/v1/admin/users/tekton-bot" -H "accept: application/json" -H "Content-Type: application/json" -d '%s'
        TOKEN=$(curl -X POST "http://%s:%s@%s:3000/api/v1/users/tekton-bot/tokens" -H "accept: application/json" -H "Content-Type: application/json" -d "{\"name\":\"bot_token_name\"}" | sed 's/.*"sha1":"\([^"]*\)".*/\1/')
         # Make sure we don't add a trailing newline to the result!
         echo -n "$TOKEN" > $(results.token.path)
`,
		trName, namespace,
		scmGiteaAdminPassword, giteaInternalHostname, giteaUserJSON,
		scmGiteaAdminPassword, giteaInternalHostname, giteaUserJSON,
		scmRemoteUser, scmRemoteUserPassword, giteaInternalHostname))

	if _, err := c.V1TaskRunClient.Create(ctx, giteaConfigTaskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create gitea user setup TaskRun: %s", err)
	}

	t.Logf("Waiting for gitea user setup TaskRun in namespace %s to succeed", namespace)
	if err := WaitForTaskRunState(ctx, c, trName, TaskRunSucceed(trName), "TaskRunSucceed", v1Version); err != nil {
		t.Fatalf("Error waiting for gitea user setup TaskRun to finish: %s", err)
	}

	tr, err := c.V1TaskRunClient.Get(ctx, trName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected gitea user setup TaskRun %s: %s", trName, err)
	}

	token := ""
	for _, trr := range tr.Status.Results {
		if trr.Name == "token" {
			token = trr.Value.StringVal
		}
	}
	if token == "" {
		t.Fatalf("didn't find token result on gitea setup TaskRun %s", trName)
	}

	secretName := helpers.AppendRandomString(scmTokenSecretBase)

	_, err = c.KubeClient.CoreV1().Secrets(namespace).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			scmTokenSecretKey: []byte(token),
		},
	}, metav1.CreateOptions{})

	if err != nil {
		t.Fatalf("Failed to create gitea token secret %s: %v", secretName, err)
	}

	remoteTaskBytes, err := os.ReadFile(filepath.Join("git-resolver", "remote-task.yaml"))
	if err != nil {
		t.Fatalf("Failed to read git-resolver/remote-task.yaml: %v", err)
	}

	restCfg, err := knativetest.BuildClientConfig(knativetest.Flags.Kubeconfig, knativetest.Flags.Cluster)
	if err != nil {
		t.Fatalf("failed to create configuration obj from %s for cluster %s: %s", knativetest.Flags.Kubeconfig, knativetest.Flags.Cluster, err)
	}

	// To do API operations in Gitea from outside of the cluster, we need to forward the gitea-0 pod's port 3000 locally.
	// We're using https://github.com/goccy/kpoward so we can do this programmatically.
	kpow := kpoward.New(restCfg, "gitea-0", 3000)
	kpow.SetNamespace(namespace)

	if err := kpow.Run(ctx, func(ctx context.Context, localPort uint16) error {
		// To access the Gitea API to create the organization, repository, and file, we're using the Gitea go-sdk. Initial
		// attempts to do this with the go-scm client (which wraps the Gitea go-sdk) failed to properly create the repo
		// with a default branch, and the task YAML file wasn't created properly either. We still verify that the go-scm
		// client can access the newly created file before finishing.

		giteaURL := fmt.Sprintf("http://localhost:%d/", localPort)

		giteaClient, err := gitea.NewClient(giteaURL, gitea.SetToken(token))
		if err != nil {
			return fmt.Errorf("failed to create Gitea client: %w", err)
		}
		_, _, err = giteaClient.CreateOrg(gitea.CreateOrgOption{
			Name: scmRemoteOrg,
		})
		if err != nil {
			return fmt.Errorf("failed to create %s organization in gitea: %w", scmRemoteOrg, err)
		}

		_, _, err = giteaClient.CreateOrgRepo(scmRemoteOrg, gitea.CreateRepoOption{
			Name:          scmRemoteRepo,
			Private:       true,
			AutoInit:      true,
			DefaultBranch: scmRemoteBranch,
		})
		if err != nil {
			return fmt.Errorf("failed to create %s repository in gitea: %w", scmRemoteRepo, err)
		}

		resp, _, err := giteaClient.CreateFile(scmRemoteOrg, scmRemoteRepo, scmRemoteTaskPath, gitea.CreateFileOptions{
			FileOptions: gitea.FileOptions{
				Message: "create file " + scmRemoteTaskPath,
			},
			Content: base64.StdEncoding.EncodeToString(remoteTaskBytes),
		})
		if err != nil {
			return fmt.Errorf("failed to create remote-task.yaml in gitea: %w", err)
		}
		if resp.Content.Type != "file" {
			return fmt.Errorf("expected new file to have type file, but was %s", resp.Content.Type)
		}

		// Verify file can be fetched by the scm-client.
		scmClient, err := factory.NewClient("gitea", giteaURL, token)
		if err != nil {
			return fmt.Errorf("failed to create go-scm client: %w", err)
		}

		_, _, err = scmClient.Contents.Find(ctx, fmt.Sprintf("%s/%s", scmRemoteOrg, scmRemoteRepo), scmRemoteTaskPath, scmRemoteBranch)
		if err != nil {
			return fmt.Errorf("couldn't fetch file content from gitea: %w", err)
		}

		return nil
	}); err != nil {
		t.Fatalf("failed to set up gitea org/repo/content through port forwarding: %v", err)
	}

	return giteaInternalHostname, secretName
}
