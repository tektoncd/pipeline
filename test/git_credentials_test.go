//go:build e2e

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
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/sdk/gitea"
	"github.com/goccy/kpoward"
	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

const (
	// Test-specific constants for multi-credential testing
	multiCredRepo1     = "private-repo-1"
	multiCredRepo2     = "private-repo-2"
	multiCredOrg       = "multi-cred-org"
	multiCredUser1     = "user1"
	multiCredUser2     = "user2"
	multiCredPassword1 = "password1_ABC123"
	multiCredPassword2 = "password2_XYZ789"
)

// TestGitCredentials_MultipleReposSameHost tests that multiple repositories on the same
// Git host can use different credentials. This verifies that useHttpPath=true is correctly
// set in .gitconfig when the credential URL includes a path, enabling path-based credential matching.
//
// This test validates the fix for supporting multiple git credentials on the same host by:
// 1. Creating two private repos with different user credentials
// 2. Configuring repo-specific credential secrets with path-based URLs
// 3. Verifying that a TaskRun can clone both repos using the correct credentials
//
// @test:execution=parallel
func TestGitCredentials_MultipleReposSameHost(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Set up gitea and create two private repos with different credentials
	giteaClusterHostname, secretName1, secretName2 := setupGiteaMultiRepo(ctx, t, c, namespace)

	// Create ServiceAccount with both git credential secrets
	saName := helpers.AppendRandomString("multi-cred-sa")
	_, err := c.KubeClient.CoreV1().ServiceAccounts(namespace).Create(ctx, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: namespace,
		},
		Secrets: []corev1.ObjectReference{
			{Name: secretName1},
			{Name: secretName2},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create ServiceAccount: %v", err)
	}

	repo1URL := fmt.Sprintf("http://%s/%s/%s", net.JoinHostPort(giteaClusterHostname, "3000"), multiCredOrg, multiCredRepo1)
	repo2URL := fmt.Sprintf("http://%s/%s/%s", net.JoinHostPort(giteaClusterHostname, "3000"), multiCredOrg, multiCredRepo2)

	trName := helpers.ObjectNameForTest(t)
	tr := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  serviceAccountName: %s
  taskSpec:
    steps:
    - name: clone-both-repos
      image: docker.io/alpine/git:latest
      script: |
        #!/usr/bin/env sh
        set -ex

        echo "=== Checking Git config (should have useHttpPath=true for repo-specific URLs) ==="
        cat ~/.gitconfig || echo "No .gitconfig"

        echo "=== Checking Git credentials ==="
        cat ~/.git-credentials || echo "No .git-credentials"

        echo "=== Cloning repo1 with user1 credentials ==="
        git clone %s /workspace/repo1
        echo "Successfully cloned repo1"
        ls -la /workspace/repo1

        echo "=== Cloning repo2 with user2 credentials ==="
        git clone %s /workspace/repo2
        echo "Successfully cloned repo2"
        ls -la /workspace/repo2

        echo "=== Both repos cloned successfully with different credentials ==="
`,
		trName,
		namespace,
		saName,
		repo1URL,
		repo2URL,
	))

	_, err = c.V1TaskRunClient.Create(ctx, tr, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create TaskRun: %v", err)
	}

	t.Logf("Waiting for TaskRun %s in namespace %s to complete", trName, namespace)
	if err := WaitForTaskRunState(ctx, c, trName, TaskRunSucceed(trName), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for TaskRun %s to finish: %s", trName, err)
	}

	t.Log("TaskRun succeeded - multiple git credentials on same host working correctly")
}

// setupGiteaMultiRepo sets up gitea with two private repositories, each with its own user and credentials.
// It returns the gitea cluster hostname and the names of the two secrets containing the credentials.
func setupGiteaMultiRepo(ctx context.Context, t *testing.T, c *clients, namespace string) (string, string, string) {
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

	// Wait for gitea pod to be running
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

	// Create two users via admin API
	user1JSON := fmt.Sprintf(`{"admin":false,"email":"%s@example.com","full_name":"%s","login_name":"%s","must_change_password":false,"password":"%s","send_notify":false,"source_id":0,"username":"%s"}`,
		multiCredUser1, multiCredUser1, multiCredUser1, multiCredPassword1, multiCredUser1)
	user2JSON := fmt.Sprintf(`{"admin":false,"email":"%s@example.com","full_name":"%s","login_name":"%s","must_change_password":false,"password":"%s","send_notify":false,"source_id":0,"username":"%s"}`,
		multiCredUser2, multiCredUser2, multiCredUser2, multiCredPassword2, multiCredUser2)

	trName := helpers.AppendRandomString("git-creds-setup-users")
	setupUsersTaskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskSpec:
    steps:
    - image: docker.io/alpine/curl
      script: |
        #!/bin/ash
        set -ex
        # Create user1
        curl -X POST "http://gitea_admin:%s@%s:3000/api/v1/admin/users" -H "accept: application/json" -H "Content-Type: application/json" -d '%s'
        # Create user2
        curl -X POST "http://gitea_admin:%s@%s:3000/api/v1/admin/users" -H "accept: application/json" -H "Content-Type: application/json" -d '%s'
        echo "Users created successfully"
`,
		trName, namespace,
		scmGiteaAdminPassword, giteaInternalHostname, user1JSON,
		scmGiteaAdminPassword, giteaInternalHostname, user2JSON))

	if _, err := c.V1TaskRunClient.Create(ctx, setupUsersTaskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create user setup TaskRun: %s", err)
	}

	t.Logf("Waiting for user setup TaskRun in namespace %s to succeed", namespace)
	if err := WaitForTaskRunState(ctx, c, trName, TaskRunSucceed(trName), "TaskRunSucceed", v1Version); err != nil {
		t.Fatalf("Error waiting for user setup TaskRun to finish: %s", err)
	}

	// Create organization and repos, grant access to users via port forwarding
	restCfg, err := knativetest.BuildClientConfig(knativetest.Flags.Kubeconfig, knativetest.Flags.Cluster)
	if err != nil {
		t.Fatalf("failed to create configuration obj: %s", err)
	}

	kpow := kpoward.New(restCfg, "gitea-0", 3000)
	kpow.SetNamespace(namespace)

	if err := kpow.Run(ctx, func(ctx context.Context, localPort uint16) error {
		giteaURL := fmt.Sprintf("http://localhost:%d/", localPort)

		// Login as admin to create org and repos
		adminClient, err := gitea.NewClient(giteaURL, gitea.SetBasicAuth("gitea_admin", scmGiteaAdminPassword))
		if err != nil {
			return fmt.Errorf("failed to create admin Gitea client: %w", err)
		}

		// Create organization
		_, _, err = adminClient.CreateOrg(gitea.CreateOrgOption{
			Name: multiCredOrg,
		})
		if err != nil {
			return fmt.Errorf("failed to create org %s: %w", multiCredOrg, err)
		}

		// Create repo1 (owned by org, accessible to user1)
		_, _, err = adminClient.CreateOrgRepo(multiCredOrg, gitea.CreateRepoOption{
			Name:          multiCredRepo1,
			Private:       true,
			AutoInit:      true,
			DefaultBranch: "main",
		})
		if err != nil {
			return fmt.Errorf("failed to create repo1: %w", err)
		}

		// Create repo2 (owned by org, accessible to user2)
		_, _, err = adminClient.CreateOrgRepo(multiCredOrg, gitea.CreateRepoOption{
			Name:          multiCredRepo2,
			Private:       true,
			AutoInit:      true,
			DefaultBranch: "main",
		})
		if err != nil {
			return fmt.Errorf("failed to create repo2: %w", err)
		}

		// Add user1 as collaborator to repo1 only
		readPerm := gitea.AccessModeRead
		_, err = adminClient.AddCollaborator(multiCredOrg, multiCredRepo1, multiCredUser1, gitea.AddCollaboratorOption{
			Permission: &readPerm,
		})
		if err != nil {
			return fmt.Errorf("failed to add user1 as collaborator to repo1: %w", err)
		}

		// Add user2 as collaborator to repo2 only
		_, err = adminClient.AddCollaborator(multiCredOrg, multiCredRepo2, multiCredUser2, gitea.AddCollaboratorOption{
			Permission: &readPerm,
		})
		if err != nil {
			return fmt.Errorf("failed to add user2 as collaborator to repo2: %w", err)
		}

		return nil
	}); err != nil {
		t.Fatalf("failed to set up gitea org/repos: %v", err)
	}

	// Create kubernetes secrets for git credentials
	// Secret 1: credentials for repo1 with repo-specific URL (will trigger useHttpPath=true)
	secretName1 := helpers.AppendRandomString("git-creds-repo1")
	repo1URL := fmt.Sprintf("http://%s/%s/%s", net.JoinHostPort(giteaInternalHostname, "3000"), multiCredOrg, multiCredRepo1)
	_, err = c.KubeClient.CoreV1().Secrets(namespace).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName1,
			Namespace: namespace,
			Annotations: map[string]string{
				"tekton.dev/git-0": repo1URL,
			},
		},
		Type: corev1.SecretTypeBasicAuth,
		Data: map[string][]byte{
			corev1.BasicAuthUsernameKey: []byte(multiCredUser1),
			corev1.BasicAuthPasswordKey: []byte(multiCredPassword1),
		},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create secret1: %v", err)
	}

	// Secret 2: credentials for repo2 with repo-specific URL (will trigger useHttpPath=true)
	secretName2 := helpers.AppendRandomString("git-creds-repo2")
	repo2URL := fmt.Sprintf("http://%s/%s/%s", net.JoinHostPort(giteaInternalHostname, "3000"), multiCredOrg, multiCredRepo2)
	_, err = c.KubeClient.CoreV1().Secrets(namespace).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName2,
			Namespace: namespace,
			Annotations: map[string]string{
				"tekton.dev/git-0": repo2URL,
			},
		},
		Type: corev1.SecretTypeBasicAuth,
		Data: map[string][]byte{
			corev1.BasicAuthUsernameKey: []byte(multiCredUser2),
			corev1.BasicAuthPasswordKey: []byte(multiCredPassword2),
		},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create secret2: %v", err)
	}

	return giteaInternalHostname, secretName1, secretName2
}
