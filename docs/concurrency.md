## Concurrency control primitives

The "ConcurrencyControl" CRD is used for higher-level systems to control concurrency of PipelineRuns.
To use concurrency controls, first create an instance of a ConcurrencyControl, for example:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: ConcurrencyControl
metadata:
  name: my-concurrency-control
spec:
  params:
  - name: param-1
  - name: param-2
  key: $(params.param-1)-$(params.param-2)
  strategy: Cancel
```

The only currently supported strategy is "Cancel", which will patch other PipelineRuns with the same
concurrency key in the same namespace as Canceled.
The parameters defined by the ConcurrencyControl should match the parameters declared in the resource
it applies to (currently, only PipelineRuns are supported).
The ConcurrencyControl must be in the same namespace as any PipelineRuns using it.

Next, create your PipelineRun with the label "tekton.dev/concurrency": "insert-name-of-concurrency-control-here".

The PipelineRun controller will apply a concurrency key label to your PipelineRun.
For example, for the following PipelineRun:

```yaml
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  labels:
    tekton.dev/concurrency: my-concurrency-control
spec:
  params:
  - name: param-1
    value: foo
  - name: param-2
    value: bar
```

the controller will apply the label "tekton.dev/concurrency-key": "foo-bar".
If any PipelineRuns in the same namespace have the same concurrency key, they will be canceled
before the existing PipelineRun begins running.
 