### Pipeline Resources

## Git Resource

Git resource represents a [git](https://git-scm.com/) repository, that containes the source code to be built by the pipeline. Adding the git resource as an input to a task will clone this repository and allow the task to perform the required actions on the contents of the repo.  

Use the following example to understand the syntax and strucutre of a Git Resource

 #### Create a git resource using the `PipelineResource` CRD
 
 ```
apiVersion: pipeline.knative.dev/v1alpha1
kind: PipelineResource
metadata:
  name: wizzbang-git
  namespace: default
spec:
  type: git
  params:
  - name: url
    value: github.com/wizzbangcorp/wizzbang
  - name: Revision
    value: master
 ```

   Params that can be added are the following:

   1. URL: represents the location of the git repository 
   1. Revision: Git [revision](https://git-scm.com/docs/gitrevisions#_specifying_revisions ) (branch, tag, commit SHA or ref) to clone. If no revision is specified, the resource will default to `latest` from `master`

 #### Use the defined git resource in a `Task` definition

```
apiVersion: pipeline.knative.dev/v1alpha1
kind: Task
metadata:
  name: build-push-task
  namespace: default
spec:
    inputs:
        resources:
           - name: wizzbang-git
             type: git
        params:
           - name: PATH_TO_DOCKERFILE
             value: string
    outputs:
        resources:
          - name: builtImage 
    buildSpec:
        template:
            name: kaniko
            arguments:
                - name: DOCKERFILE
                  value: ${PATH_TO_DOCKERFILE}
                - name: REGISTRY
                  value: ${REGISTRY}
``` 

 #### And finally set the version in the `TaskRun` definition

```
apiVersion: pipeline.knative.dev/v1alpha1
kind: TaskRun
metadata:
  name: build-push-task-run
  namespace: default
spec:
    taskRef:
        name: build-push-task
    inputs:
        resourcesVersion:
          - resourceRef:
              name: wizzbang-git
            version: HEAD
    outputs:
        artifacts:
          - name: builtImage
            type: image
``` 

#### Templating

Git Resources (like all Resources) support template expansion into BuildSpecs.
Git Resources support the following keys for replacement:

* name
* url
* type
* revision

These can be referenced in a TaskRun spec like:

```shell
${inputs.resources.NAME.KEY}
```

where NAME is the Resource Name and KEY is the key from the above list.
