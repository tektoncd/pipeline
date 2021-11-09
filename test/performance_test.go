// +build performance

package test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
	knativetest "knative.dev/pkg/test"
)

const (
	qps               = 1
	burst             = qps
	injectionDuration = 1 * time.Minute
)

func TestInject(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	t.Helper()

	cfg := clientConfig(t)
	kubeClient, cs := clientSets(t, cfg)

	rateLimitedCfg := clientConfig(t)
	rateLimitedCfg.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(qps, burst)
	rateLimitedKubeClient, rateLimitedCS := clientSets(t, rateLimitedCfg)

	wg := sync.WaitGroup{}
	timeLimit := time.Now().Add(injectionDuration)
	for i := 1; i < 2*qps+1; i++ {
		wg.Add(1)
		j := i
		go func() {
			defer wg.Done()
			namespace := fmt.Sprintf("user%d", j)
			_, _ = rateLimitedKubeClient.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			}, metav1.CreateOptions{})

			for time.Now().Before(timeLimit) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				task, err := rateLimitedCS.TektonV1beta1().TaskRuns(namespace).Create(ctx, &v1beta1.TaskRun{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "perf-task-",
					},
					Spec: v1beta1.TaskRunSpec{
						ServiceAccountName: "default",
						TaskSpec: &v1beta1.TaskSpec{
							Steps: []v1beta1.Step{
								{
									Container: corev1.Container{
										Name:            "step1",
										Image:           "busybox:1.34.0",
										ImagePullPolicy: corev1.PullIfNotPresent,
									},
									Script: "ls /",
								},
							},
						},
					},
				}, metav1.CreateOptions{})
				if err != nil {
					logrus.Error(err)
				} else {
					logrus.Infof("[%s/%s] taskrun created",
						namespace,
						task.Name,
					)

					wg.Add(1)
					go func() {
						defer wg.Done()
						var podName string
						for {
							task, err := cs.TektonV1beta1().TaskRuns(namespace).Get(context.Background(), task.Name, metav1.GetOptions{})
							if err != nil {
								continue
							}
							if task.Status.PodName != "" {
								pod, _ := kubeClient.CoreV1().Pods(namespace).Get(context.Background(), task.Status.PodName, metav1.GetOptions{})
								if err != nil {
									continue
								}
								logrus.Infof("[%s/%s] controller took %s to create the pod",
									namespace,
									task.Name,
									pod.CreationTimestamp.Time.Sub(task.CreationTimestamp.Time).String())
								podName = task.Status.PodName
								break
							}
							time.Sleep(2 * time.Second)
						}
						for {
							pod, _ := kubeClient.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
							if err != nil {
								continue
							}
							if pod.Status.Phase == corev1.PodSucceeded {
								_ = cs.TektonV1beta1().TaskRuns(namespace).Delete(context.Background(), task.Name, metav1.DeleteOptions{})
								_ = kubeClient.CoreV1().Pods(namespace).Delete(context.Background(), task.Status.PodName, metav1.DeleteOptions{})
								logrus.Infof("[%s/%s] taskrun and pod deleted",
									namespace,
									task.Name,
								)
								return
							}
							time.Sleep(2 * time.Second)
						}
					}()
				}
			}
		}()
	}
	wg.Wait()
}

func clientSets(t *testing.T, cfg *rest.Config) (*kubernetes.Clientset, *versioned.Clientset) {
	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("failed to create kubeclient from config file at %s: %s", knativetest.Flags.Kubeconfig, err)
	}

	cs, err := versioned.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("failed to create pipeline clientset from config file at %s: %s", knativetest.Flags.Kubeconfig, err)
	}
	return kubeClient, cs
}

func clientConfig(t *testing.T) *rest.Config {
	cfg, err := knativetest.BuildClientConfig(knativetest.Flags.Kubeconfig, knativetest.Flags.Cluster)
	if err != nil {
		t.Fatalf("failed to create configuration obj from %s for cluster %s: %s", knativetest.Flags.Kubeconfig, knativetest.Flags.Cluster, err)
	}
	return cfg
}
