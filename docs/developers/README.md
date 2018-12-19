## Developer docs

This document is aimed at helping maintainers/developers of project understand
the complexity.

### How are resources shared between tasks

Pipelineruns uses PVC to share resources between tasks. PVC volume is mounted on
path `/pvc` by Pipelinerun.

- If resource in a task is declared as output then taskun controllers add step
  to copy each output resource to directory path `/pvc/task_name/resource_name`.

- If input resource includes `providedBy` condition then container definition to
  copy from PVC to directory from `/pvc/previous_task/resource_name`.

### How are inputs handled?

Input resources like source code(git) or artifacts are dumped at path
`/workspace/`. Resource definition in task can have custom target directory. If
`targetPath` is mentioned then controllers have to be responsible for adding
container definition to create directories and pulling down the versioned
artifacts into that directory.

### How are outputs handled?

Output resources like source code(git) or artifacts(storage resource) are
expected in directory path `/workspace/output/resource_name`.

- If resource has output "action" like upload to blob storage, then container
  step is added for this action.
- If there is PVC volume present(taskrun holds owner reference to pipelinerun)
  then copy step is added as well.

  - Copy step includes resource being copied to PVC to path
    `/pvc/task_name/resource_name` from `/workspace/output/resource_name` if
    resource is not declared in input resource like below example.

  ```yaml
  kind: Task
  metadata:
  name: get-gcs-task
  namespace: default
  spec:
  outputs:
    resources:
      - name: gcs-workspace
    type: storage
  ```

  - Copy step includes resource being copied to PVC to path
    `/pvc/task_name/resource_name` from `/workspace/random-space/` if resource
    is declared in input resource like below example.

  ```yaml
  kind: Task
  metadata:
  name: get-gcs-task
  namespace: default
  spec:
  inputs:
    resources:
      - name: gcs-workspace
    type: storage
    targetPath: random-space
  outputs:
    resources:
      - name: gcs-workspace
    type: storage
  ```
