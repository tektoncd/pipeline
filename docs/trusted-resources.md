# Trusted Resources

- [Overview](#overview)
- [Instructions](#Instructions)
 - [Sign Resources](#sign-resources)
 - [Enable Trusted Resources](#enable-trusted-resources)

## Overview

Trusted Resources is a feature which can be used to sign Tekton Resources and verify them. Details of design can be found at [TEP--0091](https://github.com/tektoncd/community/blob/main/teps/0091-trusted-resources.md). This feature is under `alpha` version and support `v1beta1` version of `Task` and `Pipeline`.

Verification failure will mark corresponding taskrun/pipelinerun as Failed status and stop the execution.

**Note:** KMS is not currently supported and will be supported in the following work.


## Instructions

### Sign Resources
For `Sign` cli you may refer to [experimental repo](https://github.com/tektoncd/experimental/tree/main/pipeline/trusted-resources) to sign the resources. We're working to add `sign` and `verify` into [Tekton Cli](https://github.com/tektoncd/cli) as a subcommand.

A signed task example:
```yaml
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  annotations:
    tekton.dev/signature: MEYCIQDM8WHQAn/yKJ6psTsa0BMjbI9IdguR+Zi6sPTVynxv6wIhAMy8JSETHP7A2Ncw7MyA7qp9eLsu/1cCKOjRL1mFXIKV
  creationTimestamp: null
  name: example-task
  namespace: tekton-trusted-resources
spec:
  steps:
  - image: ubuntu
    name: echo
```

### Enable Trusted Resources

#### Enable feature flag

Update the config map:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: feature-flags
  namespace: tekton-pipelines
  labels:
    app.kubernetes.io/instance: default
    app.kubernetes.io/part-of: tekton-pipelines
data:
  resource-verification-mode: "enforce"
```

**Note:** `resource-verification-mode` needs to be set as `enforce` or `warn` to enable resource verification.

`resource-verification-mode` configurations:
 * `enforce`: Failing verification will mark the taskruns/pipelineruns as failed.
 * `warn`: Log warning but don't fail the taskruns/pipelineruns.
 * `skip`: Directly skip the verification.

Or patch the new values:
```bash
kubectl patch configmap feature-flags -n tekton-pipelines -p='{"data":{"resource-verification-mode":"enforce"}}
```

#### Config key at configmap
Note that multiple keys reference should be separated by comma. If the resource can pass any key in the list, it will pass the verification.

We currently hardcode SHA256 as hashfunc for loading public keys as verifiers.

Public key files should be added into secret and mounted into controller volumes. To add keys into secret you may execute:

```shell
kubectl create secret generic verification-secrets \
  --from-file=cosign.pub=./cosign.pub \
    --from-file=cosign.pub=./cosign2.pub \
  -n tekton-pipelines
```

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-trusted-resources
  namespace: tekton-pipelines
  labels:
    app.kubernetes.io/instance: default
    app.kubernetes.io/part-of: tekton-pipelines
data:
  publickeys: "/etc/verification-secrets/cosign.pub, /etc/verification-secrets/cosign2.pub"
```
