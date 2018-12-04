package test

import (
	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Task(name, namespace string, ops ...func(*v1alpha1.Task)) *v1alpha1.Task {
	t := &v1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}

	for _, op := range ops {
		op(t)
	}

	return t
}

func TaskSpec(ops ...func(*v1alpha1.TaskSpec)) func(*v1alpha1.Task) {
	return func(t *v1alpha1.Task) {
		spec := &t.Spec
		for _, op := range ops {
			op(spec)
		}
		t.Spec = *spec
	}
}

func Step(name, image string, ops ...func(*corev1.Container)) func(*v1alpha1.TaskSpec) {
	return func(spec *v1alpha1.TaskSpec) {
		if spec.Steps == nil {
			spec.Steps = []corev1.Container{}
		}
		step := &corev1.Container{
			Name:  name,
			Image: image,
		}
		for _, op := range ops {
			op(step)
		}
		spec.Steps = append(spec.Steps, *step)
	}
}

func Command(args ...string) func(*corev1.Container) {
	return func(container *corev1.Container) {
		container.Command = args
	}
}

func TaskRun(name, namespace string, ops ...func(*v1alpha1.TaskRun)) *v1alpha1.TaskRun {
	tr := &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: v1alpha1.TaskRunSpec{
			Trigger: v1alpha1.TaskTrigger{
				TriggerRef: v1alpha1.TaskTriggerRef{
					Type: v1alpha1.TaskTriggerTypeManual,
				},
			},
		},
	}

	for _, op := range ops {
		op(tr)
	}

	return tr
}

func TaskRunSpec(name string, ops ...func(*v1alpha1.TaskRunSpec)) func(*v1alpha1.TaskRun) {
	return func(tr *v1alpha1.TaskRun) {
		spec := &tr.Spec
		spec.TaskRef = &v1alpha1.TaskRef{
			Name: name,
		}
		for _, op := range ops {
			op(spec)
		}
		tr.Spec = *spec
	}
}

func TaskTrigger(name string, triggerType v1alpha1.TaskTriggerType) func(*v1alpha1.TaskRunSpec) {
	return func(trs *v1alpha1.TaskRunSpec) {
		trs.Trigger = v1alpha1.TaskTrigger{
			TriggerRef: v1alpha1.TaskTriggerRef{
				Name: name,
				Type: triggerType,
			},
		}
	}
}

func TaskRunServiceAccount(sa string) func(*v1alpha1.TaskRunSpec) {
	return func(trs *v1alpha1.TaskRunSpec) {
		trs.ServiceAccount = sa
	}
}

func Pipeline(name, namespace string, ops ...func(*v1alpha1.Pipeline)) *v1alpha1.Pipeline {
	p := &v1alpha1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}

	for _, op := range ops {
		op(p)
	}

	return p
}

func PipelineSpec(ops ...func(*v1alpha1.PipelineSpec)) func(*v1alpha1.Pipeline) {
	return func(p *v1alpha1.Pipeline) {
		ps := &p.Spec

		for _, op := range ops {
			op(ps)
		}

		p.Spec = *ps
	}
}

func PipelineTask(name, taskName string) func(*v1alpha1.PipelineSpec) {
	return func(ps *v1alpha1.PipelineSpec) {
		ps.Tasks = append(ps.Tasks, v1alpha1.PipelineTask{
			Name: name,
			TaskRef: v1alpha1.TaskRef{
				Name: taskName,
			},
		})
	}
}

func PipelineRun(name, namespace string, ops ...func(*v1alpha1.PipelineRun)) *v1alpha1.PipelineRun {
	pr := &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineTriggerRef: v1alpha1.PipelineTriggerRef{
				Type: v1alpha1.PipelineTriggerTypeManual,
			},
		},
	}

	for _, op := range ops {
		op(pr)
	}

	return pr
}

func PipelineRunSpec(name string, ops ...func(*v1alpha1.PipelineRunSpec)) func(*v1alpha1.PipelineRun) {
	return func(pr *v1alpha1.PipelineRun) {
		prs := &pr.Spec

		prs.PipelineRef.Name = name

		for _, op := range ops {
			op(prs)
		}

		pr.Spec = *prs
	}
}

func PipelineRunServiceAccount(sa string) func(*v1alpha1.PipelineRunSpec) {
	return func(prs *v1alpha1.PipelineRunSpec) {
		prs.ServiceAccount = sa
	}
}
