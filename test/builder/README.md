# Builder package for tests

This package holds `Builder` functions that can be used to create struct in
tests with less noise.

One of the most important characteristic of a unit test (and any type of test
really) is **readability**. This means it should be easy to read but most
importantly it should clearly show the intent of the test. The setup (and
cleanup) of the tests should be as small as possible to avoid the noise. Those
builders exists to help with that.

There is two types of functions defined in that package :

    * *Builders*: create and return a struct
    * *Modifiers*: return a function
      that will operate on a given struct. They can be applied to other
      Modifiers or Builders.

Most of the Builder (and Modifier) that accepts Modifiers defines a type
(`TypeOp`) that can be satisfied by existing function in this package, from
other package _or_ inline. An example would be the following.

```go
    // Definition
    type TaskRunOp func(*v1alpha1.TaskRun)
    func TaskRun(name, namespace string, ops ...TaskRunOp) *v1alpha1.TaskRun {
        // […]
    }
    func TaskRunOwnerReference(kind, name string) TaskRunOp {
        return func(t *v1alpha1.TaskRun) {
            // […]
        }
    }
    // Usage
    t := TaskRun("foo", "bar", func(t *v1alpha1.TaskRun){
        // Do something with the Task struct
        // […]
    })
```

The main reason to define the `Op` type, and using it in the methods signatures
is to group Modifier function together. It makes it easier to see what is a
Modifier (or Builder) and on what it operates.

By convention, this package is import with the "tb" as alias. The examples make
that assumption.

## Example

Let's look at a non-exhaustive example.

```go
package builder_test

import (
    "fmt"
    "testing"

    "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
    tb "github.com/tektoncd/pipeline/test/builder"
    corev1 "k8s.io/api/core/v1"
)

func MyTest(t *testing.T) {
    // You can declare re-usable modifiers
    myStep := tb.Step("my-step", "myimage")
    // … and use them in a Task definition
    myTask := tb.Task("my-task", "namespace", tb.TaskSpec(
        tb.Step("simple-step", "myotherimage", tb.Command("/mycmd")),
        myStep,
    ))
    // … and another one.
    myOtherTask := tb.Task("my-other-task", "namespace",
        tb.TaskSpec(myStep,
            tb.TaskInputs(tb.InputsResource("workspace", v1alpha1.PipelineResourceTypeGit)),
        ),
    )
    myClusterTask := tb.ClusterTask("my-task", tb.ClusterTaskSpec(
        tb.Step("simple-step", "myotherimage", tb.Command("/mycmd")),
    ))
    // A simple definition, with a Task reference
    myTaskRun := tb.TaskRun("my-taskrun", "namespace", tb.TaskRunSpec(
        tb.TaskRunTaskRef("my-task"),
    ))
    // … or a more complex one with inline TaskSpec
    myTaskRunWithSpec := tb.TaskRun("my-taskrun-with-spec", "namespace", tb.TaskRunSpec(
        tb.TaskRunInputs(
            tb.TaskRunInputsParam("myarg", "foo"),
            tb.TaskRunInputsResource("workspace", tb.TaskResourceBindingRef("git-resource")),
        ),
        tb.TaskRunTaskSpec(
            tb.TaskInputs(
                tb.InputsResource("workspace", v1alpha1.PipelineResourceTypeGit),
                tb.InputsParam("myarg", tb.ParamDefault("mydefault")),
            ),
            tb.Step("mycontainer", "myimage", tb.Command("/mycmd"),
                tb.Args("--my-arg=${inputs.params.myarg}"),
            ),
        ),
    ))
    // Pipeline
    pipeline := tb.Pipeline("tomatoes", "namespace",
        tb.PipelineSpec(tb.PipelineTask("foo", "banana")),
    )
    // … and PipelineRun
    pipelineRun := tb.PipelineRun("pear", "namespace",
        tb.PipelineRunSpec("tomatoes", tb.PipelineRunServiceAccount("inexistent")),
    )
    // And do something with them
    // […]
    if _, err := c.PipelineClient.Create(pipeline); err != nil {
        t.Fatalf("Failed to create Pipeline `%s`: %s", "tomatoes", err)
    }
    if _, err := c.PipelineRunClient.Create(pipelineRun); err != nil {
        t.Fatalf("Failed to create PipelineRun `%s`: %s", "pear", err)
    }
    // […]
}
```
