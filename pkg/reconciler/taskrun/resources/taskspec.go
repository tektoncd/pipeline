/*
Copyright 2019 The Tekton Authors

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

package resources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetTask is a function used to retrieve Tasks.
type GetTask func(context.Context, string) (v1beta1.TaskObject, error)

// GetTaskRun is a function used to retrieve TaskRuns
type GetTaskRun func(string) (*v1beta1.TaskRun, error)

// GetClusterTask is a function that will retrieve the Task from name and namespace.
type GetClusterTask func(name string) (v1beta1.TaskObject, error)

// GetTaskData will retrieve the Task metadata and Spec associated with the
// provided TaskRun. This can come from a reference Task or from the TaskRun's
// metadata and embedded TaskSpec.
func GetTaskData(ctx context.Context, taskRun *v1beta1.TaskRun, getTask GetTask) (*metav1.ObjectMeta, *v1beta1.TaskSpec, error) {
	taskMeta := metav1.ObjectMeta{}
	taskSpec := v1beta1.TaskSpec{}
	cfg := config.FromContextOrDefaults(ctx)
	switch {
	case taskRun.Spec.TaskRef != nil && taskRun.Spec.TaskRef.Name != "":
		// Get related task for taskrun
		t, err := getTask(ctx, taskRun.Spec.TaskRef.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error when listing tasks for taskRun %s: %w", taskRun.Name, err)
		}
		taskMeta = t.TaskMetadata()
		taskSpec = t.TaskSpec()
		taskSpec.SetDefaults(ctx)
	case taskRun.Spec.TaskSpec != nil:
		taskMeta = taskRun.ObjectMeta
		taskSpec = *taskRun.Spec.TaskSpec
	case cfg.FeatureFlags.EnableAPIFields == config.AlphaAPIFields && taskRun.Spec.TaskRef != nil && taskRun.Spec.TaskRef.Resolver != "":
		task, err := getTask(ctx, taskRun.Name)
		switch {
		case err != nil:
			return nil, nil, err
		case task == nil:
			return nil, nil, errors.New("resolution of remote resource completed successfully but no task was returned")
		default:
			taskMeta = task.TaskMetadata()
			taskSpec = task.TaskSpec()
		}
	default:
		return nil, nil, fmt.Errorf("taskRun %s not providing TaskRef or TaskSpec", taskRun.Name)
	}
	return &taskMeta, &taskSpec, nil
}

// ResolveTaskParamFrom is function used to get value from configmap
// and replace the default value with the obtained value
func ResolveTaskParamFrom(ctx context.Context, k8s kubernetes.Interface, namespace string, ts *v1beta1.TaskSpec) error {
	for i := range ts.Params {
		param := &ts.Params[i]
		if param.ValueFrom != nil && param.ValueFrom.ConfigMapRef != nil {
			if param.Default == nil {
				param.Default = &v1beta1.ParamValue{}
			}
			configRef := param.ValueFrom.ConfigMapRef
			configMap, err := k8s.CoreV1().ConfigMaps(namespace).Get(ctx, configRef.Name, metav1.GetOptions{})
			refData := configMap.Data[configRef.Key]
			if err != nil {
				return err
			}
			switch param.Type {
			case v1beta1.ParamTypeObject:
				param.Default.Type = v1beta1.ParamTypeObject
				var objData map[string]string
				if err := json.Unmarshal([]byte(refData), &objData); err != nil {
					return err
				}
				param.Default.ObjectVal = objData
			case v1beta1.ParamTypeArray:
				param.Default.Type = v1beta1.ParamTypeArray
				var arrData []string
				if err := json.Unmarshal([]byte(refData), &arrData); err != nil {
					return err
				}
				param.Default.ArrayVal = arrData
			default:
				param.Default.Type = v1beta1.ParamTypeString
				param.Default.StringVal = refData
			}
		}
	}
	return nil
}
