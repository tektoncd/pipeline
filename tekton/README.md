# Tekton cli Repo CI/CD

We dogfood our project by using Tekton Pipelines to build, test and
release the Tekton Pipelines cli!

This directory contains the
[`Tasks`](https://github.com/tektoncd/pipeline/blob/master/docs/tasks.md) and
[`Pipelines`](https://github.com/tektoncd/pipeline/blob/master/docs/pipelines.md)
that we (will) use for the release and the pull-requests.

TODO(tektoncd/pipeline#538): In tektoncd/pipeline#538 or tektoncd/pipeline#537 we will update
[Prow](https://github.com/tektoncd/pipeline/blob/master/CONTRIBUTING.md#pull-request-process)
to invoke these `Pipelines` automatically, but for now we will have to invoke
them manually.

## Release Pipeline

The release pipeline uses the
[`golang`](https://github.com/tektoncd/catalog/tree/master/golang)
Tasks from the
[`tektoncd/catalog`](https://github.com/tektoncd/catalog). To add them
to your cluster:

```
kubectl apply -f https://raw.githubusercontent.com/tektoncd/catalog/master/golang/lint.yaml
kubectl apply -f https://raw.githubusercontent.com/tektoncd/catalog/master/golang/build.yaml
kubectl apply -f https://raw.githubusercontent.com/tektoncd/catalog/master/golang/tests.yaml
```

It also uses the [`goreleaser`](./goreleaser.yml) Task from this
repository.

```
kubectl apply -f ./goreleaser.yml
```

Next is the actual release pipeline. It is defined in
[`release-pipeline.yml`](./release-pipeline.yml).

```
kubectl apply -f ./release-pipeline.yml
```

Note that the [`goreleaser.yml`](./goreleaser.yml) `Task` needs a
secret for the GitHub token, named `bot-token-github`.

TODO(vdemeester): It is hardcoded for now as Tekton Pipeline doesn't
support variable interpolation in the environment. This needs to be
fixed upstream.

```
kubectl create secret generic bot-token-github --from-literal=bot-token=${GITHUB_TOKEN}
```


### Running

To run these `Pipelines` and `Tasks`, you must have Tekton Pipelines installed
(in your own kubernetes cluster) either via
[an official release](https://github.com/tektoncd/pipeline/blob/master/docs/install.md)
or
[from `HEAD`](https://github.com/tektoncd/pipeline/blob/master/DEVELOPMENT.md#install-pipeline).

- [`release-pipeline-run.yml`](./release-pipeline-run.yml) — this
  runs the `ci-release-pipeline` on `tektoncd/cli` master branch. The
  way [`goreleaser`](https://goreleaser.com) works, it will build the
  latest tag.
