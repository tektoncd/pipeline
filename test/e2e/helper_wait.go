package e2e

import (
	"fmt"
	"log"
	"sync"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

//Wait For Task Run Resource to be completed
func WaitForTaskRunToComplete(c *Clients, trname string, namespace string) {
	log.Printf("Waiting for TaskRun %s in namespace %s to complete", trname, namespace)
	if err := WaitForTaskRunState(c, trname, func(tr *v1alpha1.TaskRun) (bool, error) {
		cond := tr.Status.GetCondition(apis.ConditionSucceeded)
		if cond != nil {
			if cond.Status == corev1.ConditionTrue || cond.Status == corev1.ConditionFalse {
				return true, nil
			} else if cond.Status != corev1.ConditionUnknown {
				return false, xerrors.Errorf("taskRun %s failed ", trname)
			}
		}
		return false, nil
	}, "TaskRunSuccess"); err != nil {
		log.Fatalf("Error waiting for TaskRun %s to finish: %s", trname, err)
	}
}

//Wait For Task Run Resource to be completed
func WaitForTaskRunToBeStarted(c *Clients, trname string, namespace string) {
	log.Printf("Waiting for TaskRun %s in namespace %s to be started", trname, namespace)
	if err := WaitForTaskRunState(c, trname, func(tr *v1alpha1.TaskRun) (bool, error) {
		cond := tr.Status.GetCondition(apis.ConditionSucceeded)
		if cond != nil {
			if cond.Status == corev1.ConditionTrue || cond.Status == corev1.ConditionFalse {
				return true, xerrors.Errorf("taskRun %s already finished!", trname)
			} else if cond.Status == corev1.ConditionUnknown && cond.Reason == "Running" || cond.Reason != "Pending" {
				return true, nil
			}
		}
		return false, nil
	}, "TaskRunStartedSuccessfully"); err != nil {
		log.Fatalf("Error waiting for TaskRun %s to start: %s", trname, err)
	}

}

// Waits for PipelineRun to be started
func WaitForPipelineRunToStart(c *Clients, prname string, namespace string) {

	log.Printf("Waiting for Pipelinerun %s in namespace %s to be started", prname, namespace)
	if err := WaitForPipelineRunState(c, prname, timeout, func(pr *v1alpha1.PipelineRun) (bool, error) {
		c := pr.Status.GetCondition(apis.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue || c.Status == corev1.ConditionFalse {
				return true, xerrors.Errorf("pipelineRun %s already finished!", prname)
			} else if c.Status == corev1.ConditionUnknown && (c.Reason == "Running" || c.Reason != "Pending") {
				return true, nil
			}
		}
		return false, nil
	}, "PipelineRunRunning"); err != nil {
		log.Fatalf("Error waiting for PipelineRun %s to be running: %s", prname, err)
	}
}

//Wait for Pipeline Run to complete
func WaitForPipelineRunToComplete(c *Clients, prname string, namespace string) {
	log.Printf("Waiting for Pipelinerun %s in namespace %s to be started", prname, namespace)
	if err := WaitForPipelineRunState(c, prname, timeout, func(pr *v1alpha1.PipelineRun) (bool, error) {
		c := pr.Status.GetCondition(apis.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue || c.Status == corev1.ConditionFalse {
				return true, xerrors.Errorf("pipelineRun %s already finished!", prname)
			} else if c.Status == corev1.ConditionUnknown && (c.Reason == "Running" || c.Reason == "Pending") {
				return true, nil
			}
		}
		return false, nil
	}, "PipelineRunRunning"); err != nil {
		log.Fatalf("Error waiting for PipelineRun %s to be running: %s", prname, err)
	}

	taskrunList, err := c.TaskRunClient.List(metav1.ListOptions{LabelSelector: fmt.Sprintf("tekton.dev/pipelineRun=%s", prname)})

	if err != nil {
		log.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", prname, err)
	}

	log.Printf("Waiting for TaskRuns from PipelineRun %s in namespace %s to be running", prname, namespace)
	errChan := make(chan error, len(taskrunList.Items))
	defer close(errChan)

	for _, taskrunItem := range taskrunList.Items {
		go func(name string) {
			err := WaitForTaskRunState(c, name, func(tr *v1alpha1.TaskRun) (bool, error) {
				c := tr.Status.GetCondition(apis.ConditionSucceeded)
				if c != nil {
					if c.Status == corev1.ConditionTrue || c.Status == corev1.ConditionFalse {
						return true, xerrors.Errorf("taskRun %s already finished!", name)
					} else if c.Status == corev1.ConditionUnknown && (c.Reason == "Running" || c.Reason == "Pending") {
						return true, nil
					}
				}
				return false, nil
			}, "TaskRunRunning")
			errChan <- err
		}(taskrunItem.Name)
	}

	for i := 1; i <= len(taskrunList.Items); i++ {
		if <-errChan != nil {
			log.Fatalf("Error waiting for TaskRun %s to be running: %s", taskrunList.Items[i-1].Name, err)
		}
	}

	if _, err := c.PipelineRunClient.Get(prname, metav1.GetOptions{}); err != nil {
		log.Fatalf("Failed to get PipelineRun `%s`: %s", prname, err)
	}

	log.Printf("Waiting for PipelineRun %s in namespace %s to be Completed", prname, namespace)
	if err := WaitForPipelineRunState(c, prname, timeout, func(pr *v1alpha1.PipelineRun) (bool, error) {
		c := pr.Status.GetCondition(apis.ConditionSucceeded)
		if c != nil {
			if c.Status == corev1.ConditionTrue || c.Status == corev1.ConditionFalse {
				return true, nil
			} else if c.Status != corev1.ConditionUnknown {
				return false, xerrors.Errorf("pipeline Run %s failed ", prname)
			}
		}
		return false, nil
	}, "PipelineRunSuccess"); err != nil {
		log.Fatalf("Error waiting for PipelineRun %s to finish: %s", prname, err)
	}

	log.Printf("Waiting for TaskRuns from PipelineRun %s in namespace %s to be completed", prname, namespace)
	var wg sync.WaitGroup
	for _, taskrunItem := range taskrunList.Items {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			err := WaitForTaskRunState(c, name, func(tr *v1alpha1.TaskRun) (bool, error) {
				cond := tr.Status.GetCondition(apis.ConditionSucceeded)
				if cond != nil {
					if cond.Status == corev1.ConditionTrue || cond.Status == corev1.ConditionFalse {
						return true, nil
					} else if cond.Status != corev1.ConditionUnknown {
						return false, xerrors.Errorf("Task Run %s failed ", name)
					}
				}
				return false, nil
			}, "TaskRunSuccess")
			if err != nil {
				log.Fatalf("Error waiting for TaskRun %s to err: %s", name, err)
			}
		}(taskrunItem.Name)
	}
	wg.Wait()

	if _, err := c.PipelineRunClient.Get(prname, metav1.GetOptions{}); err != nil {
		log.Fatalf("Failed to get PipelineRun `%s`: %s", prname, err)
	}
}
