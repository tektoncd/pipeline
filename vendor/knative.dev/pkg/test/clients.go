/*
Copyright 2018 The Knative Authors

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

// This file contains an object which encapsulates k8s clients which are useful for e2e tests.

package test

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"knative.dev/pkg/test/logging"
	"knative.dev/pkg/test/spoof"
)

// NewSpoofingClient returns a spoofing client to make requests
func NewSpoofingClient(ctx context.Context, client kubernetes.Interface, logf logging.FormatLogger,
	domain string, resolvable bool, opts ...spoof.TransportOption) (*spoof.SpoofingClient, error) {
	return spoof.New(ctx, client, logf, domain, resolvable, Flags.IngressEndpoint,
		Flags.SpoofRequestInterval, Flags.SpoofRequestTimeout, opts...)
}

// BuildClientConfig builds the client config specified by the config path and the cluster name
func BuildClientConfig(kubeConfigPath string, clusterName string) (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := clientcmd.ConfigOverrides{}

	if kubeConfigPath != "" {
		loadingRules.ExplicitPath = kubeConfigPath
	}
	// Override the cluster name if provided.
	if clusterName != "" {
		overrides.Context.Cluster = clusterName
	}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&overrides).ClientConfig()
}

// UpdateConfigMap updates the config map for specified @name with values
func UpdateConfigMap(ctx context.Context, client kubernetes.Interface, name string, configName string, values map[string]string) error {
	configMap, err := client.CoreV1().ConfigMaps(name).Get(ctx, configName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	for key, value := range values {
		configMap.Data[key] = value
	}

	_, err = client.CoreV1().ConfigMaps(name).Update(ctx, configMap, metav1.UpdateOptions{})
	return err
}

// CreatePod will create a Pod
func CreatePod(ctx context.Context, client kubernetes.Interface, pod *corev1.Pod) (*corev1.Pod, error) {
	pods := client.CoreV1().Pods(pod.GetNamespace())
	return pods.Create(ctx, pod, metav1.CreateOptions{})
}

// PodLogs returns Pod logs for given Pod and Container in the namespace
func PodLogs(ctx context.Context, client kubernetes.Interface, podName, containerName, namespace string) ([]byte, error) {
	pods := client.CoreV1().Pods(namespace)
	podList, err := pods.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range podList.Items {
		// Pods are big, so avoid copying.
		pod := &podList.Items[i]
		if strings.Contains(pod.Name, podName) {
			result := pods.GetLogs(pod.Name, &corev1.PodLogOptions{
				Container: containerName,
			}).Do(ctx)
			return result.Raw()
		}
	}
	return nil, fmt.Errorf("could not find logs for %s/%s:%s", namespace, podName, containerName)
}
