The cluster admin can create TaskRuns with context set:

```
$ k create -f demo/taskrun.yaml
taskrun.tekton.dev/clone-kaniko-build-push-run-fetch-source-9qxjg created
```

Create two service accounts that will be used for this demo:

```
$ k create -f demo/serviceaccount.yaml
serviceaccount/can-create-context created
serviceaccount/cannot-create-context created
clusterrolebinding.rbac.authorization.k8s.io/demo-clusterrolebinding created
clusterrolebinding.rbac.authorization.k8s.io/demo-clusterrolebinding2 created
```

The service account "can-create-context" has a clusterrolebinding that allows it to create
any Tekton objects, plus set the "context" field:

```
$ k create -f demo/taskrun.yaml --as=system:serviceaccount:default:can-create-context
taskrun.tekton.dev/clone-kaniko-build-push-run-fetch-source-t9t94 created
```

The service account "cannot-create-context" can create any Tekton objects but cannot set the context field:

```
$ k create -f demo/taskrun.yaml --as=system:serviceaccount:default:cannot-create-context
Error from server (BadRequest): error when creating "demo/taskrun.yaml": admission webhook "validation.webhook.pipeline.tekton.dev" denied the request: validation failed: not permitted: ,
```