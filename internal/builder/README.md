# PLEASE AVOID USING THESE PACKAGES

See https://github.com/tektoncd/pipeline/issues/3178 for more information

Instead of using these packages to define a Pipeline:

```go
import tb "github.com/tektoncd/pipeline/internal/builder/v1beta1"

p := tb.Pipeline("my-pipeline", tb.PipelineNamespace("my-namespace"), tb.PipelineSpec(
	tb.PipelineDescription("Example Pipeline"),
	tb.PipelineParamSpec("first-param", v1beta1.ParamTypeString, tb.ParamSpecDefault("default-value"), tb.ParamSpecDescription("default description")),
	tb.PipelineTask("my-task", "task-name",
		tb.PipelineTaskParam("stringparam", "value"),
	),
))
```

Just use the Go structs directly:

```go
import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

p := &v1beta1.Pipeline{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "my-pipeline",
		Namespace: "my-namespace",
	},
	Spec: v1beta1.PipelineSpec{
		Description: "Test Pipeline",
		Params: []v1beta1.ParamSpec{{
			Name:        "first-param",
			Type:        v1beta1.ParamTypeString,
			Default: v1beta1.ArrayOrString{
				Type:        v1beta1.ParamTypeString,
				StringValue: "default-value",
			},
			Description: "default description",
		}},
		Tasks: []v1beta1.PipelineTask{{
			Name:    "my-task",
			TaskRef: &v1beta1.TaskRef{
				Name:   "task-name",
				Params: []v1beta1.Param{{
					Name: "stringparam",
					Value: v1beta1.ArrayOrString{
						Type:        v1beta1.ParamTypeString,
						StringValue: "value",
					},
				}},
			},
		}},
	},
}
```

It's more typing, but it's more consistent with other Go code, all fields are
clearly named, and nobody has to write and test and maintain the wrapper
functions.
