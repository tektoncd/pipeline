package e2e

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"go.opencensus.io/trace"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	watch "k8s.io/apimachinery/pkg/watch"
	"knative.dev/pkg/apis"
	knativetest "knative.dev/pkg/test"
)

type TaskStateFn func(r *v1alpha1.Task) (bool, error)

// TaskRunStateFn is a condition function on TaskRun used polling functions
type TaskRunStateFn func(r *v1alpha1.TaskRun) (bool, error)

// PipelineRunStateFn is a condition function on TaskRun used polling functions
type PipelineRunStateFn func(pr *v1alpha1.PipelineRun) (bool, error)

type PodRunStateFn func(r *corev1.Pod) (bool, error)

// WaitForTaskRunState polls the status of the TaskRun called name from client every
// interval until inState returns `true` indicating it is done, returns an
// error or timeout. desc will be used to name the metric that is emitted to
// track how long it took for name to get into the state checked by inState.

func WaitForTaskRunState(c *Clients, name string, inState TaskRunStateFn, desc string) error {
	metricName := fmt.Sprintf("WaitForTaskRunState/%s/%s", name, desc)
	_, span := trace.StartSpan(context.Background(), metricName)
	defer span.End()

	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		r, err := c.TaskRunClient.Get(name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}
		return inState(r)
	})
}

// WaitForPodState polls the status of the Pod called name from client every
// interval until inState returns `true` indicating it is done, returns an
// error or timeout. desc will be used to name the metric that is emitted to
// track how long it took for name to get into the state checked by inState.
func WaitForPodState(c *Clients, name string, namespace string, inState func(r *corev1.Pod) (bool, error), desc string) error {
	metricName := fmt.Sprintf("WaitForPodState/%s/%s", name, desc)
	_, span := trace.StartSpan(context.Background(), metricName)
	defer span.End()

	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		r, err := c.KubeClient.Kube.CoreV1().Pods(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}

		return inState(r)
	})
}

func WaitForPodStateKube(c *knativetest.KubeClient, namespace string, inState PodRunStateFn, desc string) error {
	podlist, err := c.Kube.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, v := range podlist.Items {
		metricName := fmt.Sprintf("WaitForPodState/%s/%s", v.Name, desc)
		_, span := trace.StartSpan(context.Background(), metricName)
		defer span.End()

		err1 := wait.PollImmediate(interval, timeout, func() (bool, error) {
			fmt.Println("v.Name", v.Name)
			r, err := c.Kube.CoreV1().Pods(namespace).Get(v.Name, metav1.GetOptions{})
			if r.Status.Phase == "Running" || r.Status.Phase == "Succeeded" && err != nil {
				fmt.Printf("Pods are Running !! in namespace %s podName %s \n", namespace, v.Name)
				return true, err
			}
			return inState(r)
		})
		if err1 != nil {
			log.Fatal(err1.Error())
		}
	}

	return err

}

func WaitForPodStatus(kubeClient *knativetest.KubeClient, namespace string) {

	watch, err := kubeClient.Kube.CoreV1().Pods(namespace).Watch(metav1.ListOptions{})
	if err != nil {
		log.Fatal(err.Error())
	}
	respond := make(chan string)

	go PodStatus(respond, watch)
	defer close(respond)
	select {
	case queryResp := <-respond:
		if queryResp == "Done" {
			log.Println("Status of resources are up and running ")
		}
	case <-time.After(30 * time.Second):
		log.Fatalln("Status of resources are not up and running ")
	}

}

func PodStatus(respond chan<- string, watch watch.Interface) {
	count := 0
	for event := range watch.ResultChan() {
		p, ok := event.Object.(*v1.Pod)
		if !ok {
			log.Fatal("unexpected type")
		}
		fmt.Printf(" %s (status) -> %s \n", p.GenerateName, p.Status.Phase)
		if p.Status.Phase == "Running" || p.Status.Phase == "Succeeded" {
			count++
			if count == 2 {
				break
			}
		} else {
			continue
		}
	}
	respond <- "Done"

}

// WaitForPipelineRunState polls the status of the PipelineRun called name from client every
// interval until inState returns `true` indicating it is done, returns an
// error or timeout. desc will be used to name the metric that is emitted to
// track how long it took for name to get into the state checked by inState.
func WaitForPipelineRunState(c *Clients, name string, polltimeout time.Duration, inState PipelineRunStateFn, desc string) error {
	metricName := fmt.Sprintf("WaitForPipelineRunState/%s/%s", name, desc)
	_, span := trace.StartSpan(context.Background(), metricName)
	defer span.End()

	return wait.PollImmediate(interval, polltimeout, func() (bool, error) {
		r, err := c.PipelineRunClient.Get(name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}
		return inState(r)
	})
}

// WaitForServiceExternalIPState polls the status of the a k8s Service called name from client every
// interval until an external ip is assigned indicating it is done, returns an
// error or timeout. desc will be used to name the metric that is emitted to
// track how long it took for name to get into the state checked by inState.
func WaitForServiceExternalIPState(c *Clients, namespace, name string, inState func(s *corev1.Service) (bool, error), desc string) error {
	metricName := fmt.Sprintf("WaitForServiceExternalIPState/%s/%s", name, desc)
	_, span := trace.StartSpan(context.Background(), metricName)
	defer span.End()

	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		r, err := c.KubeClient.Kube.CoreV1().Services(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}
		return inState(r)
	})
}

// TaskRunSucceed provides a poll condition function that checks if the TaskRun
// has successfully completed.
func TaskRunSucceed(name string) TaskRunStateFn {
	return func(tr *v1alpha1.TaskRun) (bool, error) {
		c := tr.Status.GetCondition(apis.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue {
				return true, nil
			} else if c.Status == corev1.ConditionFalse {
				return true, xerrors.Errorf("task run %s failed!", name)
			}
		}
		return false, nil
	}
}

func PodRunSucceed(name string) PodRunStateFn {

	return func(r *corev1.Pod) (bool, error) {

		c := r.Status.Phase

		if c != "Pending" {
			if c == "Running" || c == "Succeeded" {
				fmt.Println("pods running !! ")
				return true, nil
			}
			fmt.Println("pods not running!!")
			return true, xerrors.Errorf("Pod run in namespace  %s failed!", name)
		}

		return false, nil
	}
}

// TaskRunFailed provides a poll condition function that checks if the TaskRun
// has failed.
func TaskRunFailed(name string) TaskRunStateFn {
	return func(tr *v1alpha1.TaskRun) (bool, error) {
		c := tr.Status.GetCondition(apis.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue {
				return true, xerrors.Errorf("task run %s succeeded!", name)
			} else if c.Status == corev1.ConditionFalse {
				return true, nil
			}
		}
		return false, nil
	}
}

// PipelineRunSucceed provides a poll condition function that checks if the PipelineRun
// has successfully completed.
func PipelineRunSucceed(name string) PipelineRunStateFn {
	return func(pr *v1alpha1.PipelineRun) (bool, error) {
		c := pr.Status.GetCondition(apis.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue {
				return true, nil
			} else if c.Status == corev1.ConditionFalse {
				return true, xerrors.Errorf("pipeline run %s failed!", name)
			}
		}
		return false, nil
	}
}

// PipelineRunFailed provides a poll condition function that checks if the PipelineRun
// has failed.
func PipelineRunFailed(name string) PipelineRunStateFn {
	return func(tr *v1alpha1.PipelineRun) (bool, error) {
		c := tr.Status.GetCondition(apis.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue {
				return true, xerrors.Errorf("task run %s succeeded!", name)
			} else if c.Status == corev1.ConditionFalse {
				return true, nil
			}
		}
		return false, nil
	}
}
