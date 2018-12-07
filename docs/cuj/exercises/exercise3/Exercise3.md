# Definition

In this exercise, we will define a task which executes `go test` to test your go
code. This task will run `go test` command for your go project.

You can use the go runtime image `gcr.io/google_appengine/golang`

The command used to run all tests inside inside this container image for your
code is:

```shell
docker run -i gcr.io/google_appengine/golang go test <package dir>
```

You can review the documentation for [`Task`](./../../../Concepts.md#task) An
example `Task` definition is [here](./../exercise2/build-push-task.yaml)
