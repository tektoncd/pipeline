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

package builder

import (
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

// PipelineOp is an operation which modify a Pipeline struct.
type PipelineOp func(*v1alpha1.Pipeline)

// PipelineSpecOp is an operation which modify a PipelineSpec struct.
type PipelineSpecOp func(*v1alpha1.PipelineSpec)

// PipelineTaskOp is an operation which modify a PipelineTask struct.
type PipelineTaskOp func(*v1alpha1.PipelineTask)

// PipelineRunOp is an operation which modify a PipelineRun struct.
type PipelineRunOp func(*v1alpha1.PipelineRun)

// PipelineRunSpecOp is an operation which modify a PipelineRunSpec struct.
type PipelineRunSpecOp func(*v1alpha1.PipelineRunSpec)

// PipelineResourceOp is an operation which modify a PipelineResource struct.
type PipelineResourceOp func(*v1alpha1.PipelineResource)

// PipelineResourceBindingOp is an operation which modify a PipelineResourceBinding struct.
type PipelineResourceBindingOp func(*v1alpha1.PipelineResourceBinding)

// PipelineResourceSpecOp is an operation which modify a PipelineResourceSpec struct.
type PipelineResourceSpecOp func(*v1alpha1.PipelineResourceSpec)

// PipelineTaskInputResourceOp is an operation which modifies a PipelineTaskInputResource.
type PipelineTaskInputResourceOp func(*v1alpha1.PipelineTaskInputResource)

// PipelineRunStatusOp is an operation which modifies a PipelineRunStatus
type PipelineRunStatusOp func(*v1alpha1.PipelineRunStatus)

// PipelineTaskConditionOp is an operation which modifies a PipelineTaskCondition
type PipelineTaskConditionOp func(condition *v1alpha1.PipelineTaskCondition)

// Pipeline creates a Pipeline with default values.
// Any number of Pipeline modifier can be passed to transform it.
func Pipeline(name string, ops ...PipelineOp) *v1alpha1.Pipeline {
	p := &v1alpha1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	for _, op := range ops {
		op(p)
	}

	return p
}

// PipelineType will add a TypeMeta to the pipeline's definition.
func PipelineType(t *v1alpha1.Pipeline) {
	t.TypeMeta = metav1.TypeMeta{
		Kind:       "Pipeline",
		APIVersion: "tekton.dev/v1alpha1",
	}
}

// PipelineNamespace sets the namespace on the Pipeline
func PipelineNamespace(namespace string) PipelineOp {
	return func(t *v1alpha1.Pipeline) {
		t.ObjectMeta.Namespace = namespace
	}
}

// PipelineSpec sets the PipelineSpec to the Pipeline.
// Any number of PipelineSpec modifier can be passed to transform it.
func PipelineSpec(ops ...PipelineSpecOp) PipelineOp {
	return func(p *v1alpha1.Pipeline) {
		ps := &p.Spec

		for _, op := range ops {
			op(ps)
		}

		p.Spec = *ps
	}
}

// PipelineCreationTimestamp sets the creation time of the pipeline
func PipelineCreationTimestamp(t time.Time) PipelineOp {
	return func(p *v1alpha1.Pipeline) {
		p.CreationTimestamp = metav1.Time{Time: t}
	}
}

// PipelineDescription sets the description of the pipeline
func PipelineDescription(desc string) PipelineSpecOp {
	return func(ps *v1alpha1.PipelineSpec) {
		ps.Description = desc
	}
}

// PipelineRunCancelled sets the status to cancel to the TaskRunSpec.
func PipelineRunCancelled(spec *v1alpha1.PipelineRunSpec) {
	spec.Status = v1alpha1.PipelineRunSpecStatusCancelled
}

// PipelineDeclaredResource adds a resource declaration to the Pipeline Spec,
// with the specified name and type.
func PipelineDeclaredResource(name string, t v1alpha1.PipelineResourceType) PipelineSpecOp {
	return func(ps *v1alpha1.PipelineSpec) {
		r := v1alpha1.PipelineDeclaredResource{
			Name: name,
			Type: t,
		}
		ps.Resources = append(ps.Resources, r)
	}
}

// PipelineParamSpec adds a param, with specified name and type, to the PipelineSpec.
// Any number of PipelineParamSpec modifiers can be passed to transform it.
func PipelineParamSpec(name string, pt v1alpha1.ParamType, ops ...ParamSpecOp) PipelineSpecOp {
	return func(ps *v1alpha1.PipelineSpec) {
		pp := &v1alpha1.ParamSpec{Name: name, Type: pt}
		for _, op := range ops {
			op(pp)
		}
		ps.Params = append(ps.Params, *pp)
	}
}

// PipelineTask adds a PipelineTask, with specified name and task name, to the PipelineSpec.
// Any number of PipelineTask modifier can be passed to transform it.
func PipelineTask(name, taskName string, ops ...PipelineTaskOp) PipelineSpecOp {
	return func(ps *v1alpha1.PipelineSpec) {
		pTask := &v1alpha1.PipelineTask{
			Name: name,
		}
		if taskName != "" {
			pTask.TaskRef = &v1alpha1.TaskRef{
				Name: taskName,
			}
		}
		for _, op := range ops {
			op(pTask)
		}
		ps.Tasks = append(ps.Tasks, *pTask)
	}
}

// PipelineResult adds a PipelineResult, with specified name, value and description, to the PipelineSpec.
func PipelineResult(name, value, description string) PipelineSpecOp {
	return func(ps *v1alpha1.PipelineSpec) {
		pResult := &v1beta1.PipelineResult{
			Name:        name,
			Value:       value,
			Description: description,
		}
		ps.Results = append(ps.Results, *pResult)
	}
}

// PipelineRunResult adds a PipelineResultStatus, with specified name, value and description, to the PipelineRunStatusSpec.
func PipelineRunResult(name, value string) PipelineRunStatusOp {
	return func(s *v1alpha1.PipelineRunStatus) {
		pResult := &v1beta1.PipelineRunResult{
			Name:  name,
			Value: value,
		}
		s.PipelineResults = append(s.PipelineResults, *pResult)
	}
}

// PipelineTaskSpec sets the TaskSpec on a PipelineTask.
func PipelineTaskSpec(spec *v1alpha1.TaskSpec) PipelineTaskOp {
	return func(pt *v1alpha1.PipelineTask) {
		pt.TaskSpec = spec
	}
}

// Retries sets the number of retries on a PipelineTask.
func Retries(retries int) PipelineTaskOp {
	return func(pt *v1alpha1.PipelineTask) {
		pt.Retries = retries
	}
}

// RunAfter will update the provided Pipeline Task to indicate that it
// should be run after the provided list of Pipeline Task names.
func RunAfter(tasks ...string) PipelineTaskOp {
	return func(pt *v1alpha1.PipelineTask) {
		pt.RunAfter = tasks
	}
}

// PipelineTaskRefKind sets the TaskKind to the PipelineTaskRef.
func PipelineTaskRefKind(kind v1alpha1.TaskKind) PipelineTaskOp {
	return func(pt *v1alpha1.PipelineTask) {
		pt.TaskRef.Kind = kind
	}
}

// PipelineTaskParam adds a ResourceParam, with specified name and value, to the PipelineTask.
func PipelineTaskParam(name string, value string, additionalValues ...string) PipelineTaskOp {
	return func(pt *v1alpha1.PipelineTask) {
		pt.Params = append(pt.Params, v1alpha1.Param{
			Name:  name,
			Value: *v1beta1.NewArrayOrString(value, additionalValues...),
		})
	}
}

// From will update the provided PipelineTaskInputResource to indicate that it
// should come from tasks.
func From(tasks ...string) PipelineTaskInputResourceOp {
	return func(r *v1alpha1.PipelineTaskInputResource) {
		r.From = tasks
	}
}

// PipelineTaskInputResource adds an input resource to the PipelineTask with the specified
// name, pointing at the declared resource.
// Any number of PipelineTaskInputResource modifies can be passed to transform it.
func PipelineTaskInputResource(name, resource string, ops ...PipelineTaskInputResourceOp) PipelineTaskOp {
	return func(pt *v1alpha1.PipelineTask) {
		r := v1alpha1.PipelineTaskInputResource{
			Name:     name,
			Resource: resource,
		}
		for _, op := range ops {
			op(&r)
		}
		if pt.Resources == nil {
			pt.Resources = &v1alpha1.PipelineTaskResources{}
		}
		pt.Resources.Inputs = append(pt.Resources.Inputs, r)
	}
}

// PipelineTaskOutputResource adds an output resource to the PipelineTask with the specified
// name, pointing at the declared resource.
func PipelineTaskOutputResource(name, resource string) PipelineTaskOp {
	return func(pt *v1alpha1.PipelineTask) {
		r := v1alpha1.PipelineTaskOutputResource{
			Name:     name,
			Resource: resource,
		}
		if pt.Resources == nil {
			pt.Resources = &v1alpha1.PipelineTaskResources{}
		}
		pt.Resources.Outputs = append(pt.Resources.Outputs, r)
	}
}

// PipelineTaskCondition adds a condition to the PipelineTask with the
// specified conditionRef. Any number of PipelineTaskCondition modifiers can be passed
// to transform it
func PipelineTaskCondition(conditionRef string, ops ...PipelineTaskConditionOp) PipelineTaskOp {
	return func(pt *v1alpha1.PipelineTask) {
		c := &v1alpha1.PipelineTaskCondition{
			ConditionRef: conditionRef,
		}
		for _, op := range ops {
			op(c)
		}
		pt.Conditions = append(pt.Conditions, *c)
	}
}

// PipelineTaskConditionParam adds a parameter to a PipelineTaskCondition
func PipelineTaskConditionParam(name, val string) PipelineTaskConditionOp {
	return func(condition *v1alpha1.PipelineTaskCondition) {
		if condition.Params == nil {
			condition.Params = []v1alpha1.Param{}
		}
		condition.Params = append(condition.Params, v1alpha1.Param{
			Name:  name,
			Value: *v1beta1.NewArrayOrString(val),
		})
	}
}

// PipelineTaskConditionResource adds a resource to a PipelineTaskCondition
func PipelineTaskConditionResource(name, resource string, from ...string) PipelineTaskConditionOp {
	return func(condition *v1alpha1.PipelineTaskCondition) {
		if condition.Resources == nil {
			condition.Resources = []v1alpha1.PipelineTaskInputResource{}
		}
		condition.Resources = append(condition.Resources, v1alpha1.PipelineTaskInputResource{
			Name:     name,
			Resource: resource,
			From:     from,
		})
	}
}

// PipelineTaskWorkspaceBinding adds a workspace with the specified name, workspace and subpath on a PipelineTask.
func PipelineTaskWorkspaceBinding(name, workspace, subPath string) PipelineTaskOp {
	return func(pt *v1alpha1.PipelineTask) {
		pt.Workspaces = append(pt.Workspaces, v1alpha1.WorkspacePipelineTaskBinding{
			Name:      name,
			Workspace: workspace,
			SubPath:   subPath,
		})
	}
}

// PipelineTaskTimeout sets the timeout for the PipelineTask.
func PipelineTaskTimeout(duration time.Duration) PipelineTaskOp {
	return func(pt *v1alpha1.PipelineTask) {
		pt.Timeout = &metav1.Duration{Duration: duration}
	}
}

// PipelineRun creates a PipelineRun with default values.
// Any number of PipelineRun modifier can be passed to transform it.
func PipelineRun(name string, ops ...PipelineRunOp) *v1alpha1.PipelineRun {
	pr := &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.PipelineRunSpec{},
	}

	for _, op := range ops {
		op(pr)
	}

	return pr
}

// PipelineRunNamespace sets the namespace on a PipelineRun.
func PipelineRunNamespace(namespace string) PipelineRunOp {
	return func(t *v1alpha1.PipelineRun) {
		t.ObjectMeta.Namespace = namespace
	}
}

// PipelineRunSpec sets the PipelineRunSpec, references Pipeline with specified name, to the PipelineRun.
// Any number of PipelineRunSpec modifier can be passed to transform it.
func PipelineRunSpec(name string, ops ...PipelineRunSpecOp) PipelineRunOp {
	return func(pr *v1alpha1.PipelineRun) {
		prs := &pr.Spec

		prs.PipelineRef = &v1alpha1.PipelineRef{
			Name: name,
		}
		// Set a default timeout
		prs.Timeout = &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute}

		for _, op := range ops {
			op(prs)
		}

		pr.Spec = *prs
	}
}

// PipelineRunLabel adds a label to the PipelineRun.
func PipelineRunLabel(key, value string) PipelineRunOp {
	return func(pr *v1alpha1.PipelineRun) {
		if pr.ObjectMeta.Labels == nil {
			pr.ObjectMeta.Labels = map[string]string{}
		}
		pr.ObjectMeta.Labels[key] = value
	}
}

// PipelineRunAnnotation adds a annotation to the PipelineRun.
func PipelineRunAnnotation(key, value string) PipelineRunOp {
	return func(pr *v1alpha1.PipelineRun) {
		if pr.ObjectMeta.Annotations == nil {
			pr.ObjectMeta.Annotations = map[string]string{}
		}
		pr.ObjectMeta.Annotations[key] = value
	}
}

// PipelineRunResourceBinding adds bindings from actual instances to a Pipeline's declared resources.
func PipelineRunResourceBinding(name string, ops ...PipelineResourceBindingOp) PipelineRunSpecOp {
	return func(prs *v1alpha1.PipelineRunSpec) {
		r := &v1alpha1.PipelineResourceBinding{
			Name: name,
		}
		for _, op := range ops {
			op(r)
		}
		prs.Resources = append(prs.Resources, *r)
	}
}

// PipelineResourceBindingRef set the ResourceRef name to the Resource called Name.
func PipelineResourceBindingRef(name string) PipelineResourceBindingOp {
	return func(b *v1alpha1.PipelineResourceBinding) {
		b.ResourceRef = &v1alpha1.PipelineResourceRef{
			Name: name,
		}
	}
}

// PipelineResourceBindingResourceSpec set the PipelineResourceResourceSpec to the PipelineResourceBinding.
func PipelineResourceBindingResourceSpec(spec *v1alpha1.PipelineResourceSpec) PipelineResourceBindingOp {
	return func(b *v1alpha1.PipelineResourceBinding) {
		b.ResourceSpec = spec
	}
}

// PipelineRunServiceAccountName sets the service account to the PipelineRunSpec.
func PipelineRunServiceAccountName(sa string) PipelineRunSpecOp {
	return func(prs *v1alpha1.PipelineRunSpec) {
		prs.ServiceAccountName = sa
	}
}

// PipelineRunServiceAccountNameTask configures the service account for given Task in PipelineRun.
func PipelineRunServiceAccountNameTask(taskName, sa string) PipelineRunSpecOp {
	return func(prs *v1alpha1.PipelineRunSpec) {
		prs.ServiceAccountNames = append(prs.ServiceAccountNames, v1alpha1.PipelineRunSpecServiceAccountName{
			TaskName:           taskName,
			ServiceAccountName: sa,
		})
	}
}

// PipelineRunParam add a param, with specified name and value, to the PipelineRunSpec.
func PipelineRunParam(name string, value string, additionalValues ...string) PipelineRunSpecOp {
	return func(prs *v1alpha1.PipelineRunSpec) {
		prs.Params = append(prs.Params, v1alpha1.Param{
			Name:  name,
			Value: *v1beta1.NewArrayOrString(value, additionalValues...),
		})
	}
}

// PipelineRunTimeout sets the timeout to the PipelineRunSpec.
func PipelineRunTimeout(duration time.Duration) PipelineRunSpecOp {
	return func(prs *v1alpha1.PipelineRunSpec) {
		prs.Timeout = &metav1.Duration{Duration: duration}
	}
}

// PipelineRunNilTimeout sets the timeout to nil on the PipelineRunSpec
func PipelineRunNilTimeout(prs *v1alpha1.PipelineRunSpec) {
	prs.Timeout = nil
}

// PipelineRunNodeSelector sets the Node selector to the PipelineRunSpec.
func PipelineRunNodeSelector(values map[string]string) PipelineRunSpecOp {
	return func(prs *v1alpha1.PipelineRunSpec) {
		if prs.PodTemplate == nil {
			prs.PodTemplate = &v1alpha1.PodTemplate{}
		}
		prs.PodTemplate.NodeSelector = values
	}
}

// PipelineRunPipelineSpec adds a PipelineSpec to the PipelineRunSpec.
// Any number of PipelineSpec modifiers can be passed to transform it.
func PipelineRunPipelineSpec(ops ...PipelineSpecOp) PipelineRunSpecOp {
	return func(prs *v1alpha1.PipelineRunSpec) {
		ps := &v1alpha1.PipelineSpec{}
		prs.PipelineRef = nil
		for _, op := range ops {
			op(ps)
		}
		prs.PipelineSpec = ps
	}
}

// PipelineRunStatus sets the PipelineRunStatus to the PipelineRun.
// Any number of PipelineRunStatus modifier can be passed to transform it.
func PipelineRunStatus(ops ...PipelineRunStatusOp) PipelineRunOp {
	return func(pr *v1alpha1.PipelineRun) {
		s := &v1alpha1.PipelineRunStatus{}
		for _, op := range ops {
			op(s)
		}
		pr.Status = *s
	}
}

// PipelineRunStatusCondition adds a StatusCondition to the TaskRunStatus.
func PipelineRunStatusCondition(condition apis.Condition) PipelineRunStatusOp {
	return func(s *v1alpha1.PipelineRunStatus) {
		s.Conditions = append(s.Conditions, condition)
	}
}

// PipelineRunStartTime sets the start time to the PipelineRunStatus.
func PipelineRunStartTime(startTime time.Time) PipelineRunStatusOp {
	return func(s *v1alpha1.PipelineRunStatus) {
		s.StartTime = &metav1.Time{Time: startTime}
	}
}

// PipelineRunCompletionTime sets the completion time  to the PipelineRunStatus.
func PipelineRunCompletionTime(t time.Time) PipelineRunStatusOp {
	return func(s *v1alpha1.PipelineRunStatus) {
		s.CompletionTime = &metav1.Time{Time: t}
	}
}

// PipelineRunTaskRunsStatus sets the status of TaskRun to the PipelineRunStatus.
func PipelineRunTaskRunsStatus(taskRunName string, status *v1alpha1.PipelineRunTaskRunStatus) PipelineRunStatusOp {
	return func(s *v1alpha1.PipelineRunStatus) {
		if s.TaskRuns == nil {
			s.TaskRuns = make(map[string]*v1alpha1.PipelineRunTaskRunStatus)
		}
		s.TaskRuns[taskRunName] = status
	}
}

// PipelineResource creates a PipelineResource with default values.
// Any number of PipelineResource modifier can be passed to transform it.
func PipelineResource(name string, ops ...PipelineResourceOp) *v1alpha1.PipelineResource {
	resource := &v1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	for _, op := range ops {
		op(resource)
	}
	return resource
}

// PipelineResourceNamespace sets the namespace on a PipelineResource.
func PipelineResourceNamespace(namespace string) PipelineResourceOp {
	return func(t *v1alpha1.PipelineResource) {
		t.ObjectMeta.Namespace = namespace
	}
}

// PipelineResourceSpec set the PipelineResourceSpec, with specified type, to the PipelineResource.
// Any number of PipelineResourceSpec modifier can be passed to transform it.
func PipelineResourceSpec(resourceType v1alpha1.PipelineResourceType, ops ...PipelineResourceSpecOp) PipelineResourceOp {
	return func(r *v1alpha1.PipelineResource) {
		spec := &r.Spec
		spec.Type = resourceType
		for _, op := range ops {
			op(spec)
		}
		r.Spec = *spec
	}
}

// PipelineResourceDescription sets the description of the pipeline resource
func PipelineResourceDescription(desc string) PipelineResourceSpecOp {
	return func(spec *v1alpha1.PipelineResourceSpec) {
		spec.Description = desc
	}
}

// PipelineResourceSpecParam adds a ResourceParam, with specified name and value, to the PipelineResourceSpec.
func PipelineResourceSpecParam(name, value string) PipelineResourceSpecOp {
	return func(spec *v1alpha1.PipelineResourceSpec) {
		spec.Params = append(spec.Params, v1alpha1.ResourceParam{
			Name:  name,
			Value: value,
		})
	}
}

// PipelineResourceSpecSecretParam adds a SecretParam, with specified fieldname, secretKey and secretName, to the PipelineResourceSpec.
func PipelineResourceSpecSecretParam(fieldname, secretName, secretKey string) PipelineResourceSpecOp {
	return func(spec *v1alpha1.PipelineResourceSpec) {
		spec.SecretParams = append(spec.SecretParams, v1alpha1.SecretParam{
			FieldName:  fieldname,
			SecretKey:  secretKey,
			SecretName: secretName,
		})
	}
}

// PipelineWorkspaceDeclaration adds a Workspace to the workspaces listed in the pipeline spec.
func PipelineWorkspaceDeclaration(names ...string) PipelineSpecOp {
	return func(spec *v1alpha1.PipelineSpec) {
		for _, name := range names {
			spec.Workspaces = append(spec.Workspaces, v1alpha1.PipelineWorkspaceDeclaration{Name: name})
		}
	}
}

// PipelineRunWorkspaceBindingEmptyDir adds an EmptyDir Workspace to the workspaces of a pipelinerun spec.
func PipelineRunWorkspaceBindingEmptyDir(name string) PipelineRunSpecOp {
	return func(spec *v1alpha1.PipelineRunSpec) {
		spec.Workspaces = append(spec.Workspaces, v1alpha1.WorkspaceBinding{
			Name:     name,
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		})
	}
}

// PipelineRunWorkspaceBindingVolumeClaimTemplate adds an VolumeClaimTemplate Workspace to the workspaces of a pipelineRun spec.
func PipelineRunWorkspaceBindingVolumeClaimTemplate(name string, claimName string, subPath string) PipelineRunSpecOp {
	return func(spec *v1alpha1.PipelineRunSpec) {
		spec.Workspaces = append(spec.Workspaces, v1alpha1.WorkspaceBinding{
			Name:    name,
			SubPath: subPath,
			VolumeClaimTemplate: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: claimName,
				},
				Spec: corev1.PersistentVolumeClaimSpec{},
			},
		})
	}
}
