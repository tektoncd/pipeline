# Wait Custom Task for Tekton

This folder is largely copied from [experimental/wait-task](https://github.com/tektoncd/experimental/tree/main/wait-task)
for the testing purpose, with resources used for building and releasing the
wait custom task removed.

It provides a [Tekton Custom
Task](https://tekton.dev/docs/pipelines/runs/) that, when run, simply waits a
given amount of time before succeeding, specified by an input parameter named
`duration`. It also supports `timeout` and `retries`.
