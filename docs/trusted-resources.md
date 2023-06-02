<!--
---
linkTitle: "Trusted Resources"
weight: 312
---
-->

# Trusted Resources

- [Overview](#overview)
- [Instructions](#Instructions)
 - [Sign Resources](#sign-resources)
 - [Enable Trusted Resources](#enable-trusted-resources)

## Overview

Trusted Resources is a feature which can be used to sign Tekton Resources and verify them. Details of design can be found at [TEP--0091](https://github.com/tektoncd/community/blob/main/teps/0091-trusted-resources.md). This is an alpha feature and supports `v1beta1` and `v1` version of  `Task` and `Pipeline`.

**Note**: Currently, trusted resources only support verifying Tekton resources that come from remote places i.e. git, OCI registry and Artifact Hub. To use [cluster resolver](./cluster-resolver.md) for in-cluster resources, make sure to set all default values for the resources before applied to cluster, because the mutating webhook will update the default fields if not given and fail the verification.

Verification failure will mark corresponding taskrun/pipelinerun as Failed status and stop the execution.

## Instructions

### Sign Resources
We have added `sign` and `verify` into [Tekton Cli](https://github.com/tektoncd/cli) as a subcommand in release [v0.28.0 and later](https://github.com/tektoncd/cli/releases/tag/v0.28.0). Please refer to [cli docs](https://github.com/tektoncd/cli/blob/main/docs/cmd/tkn_task_sign.md) to sign and Tekton resources.

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
  trusted-resources-verification-no-match-policy: "fail"
```

`trusted-resources-verification-no-match-policy` configurations:
 * `ignore`: if no matching policies are found, skip the verification, don't log, and don't fail the taskrun/pipelinerun
 * `warn`:  if no matching policies are found, skip the verification, log a warning, and don't fail the taskrun/pipelinerun
 * `fail`: Fail the taskrun/pipelinerun if no matching policies are found.

 **Notes:**
 * To skip the verification: make sure no policies exist and `trusted-resources-verification-no-match-policy` is set to `warn` or `ignore`.
 * To enable the verification: install [VerificationPolicy](#config-key-at-verificationpolicy) to match the resources.

Or patch the new values:
```bash
kubectl patch configmap feature-flags -n tekton-pipelines -p='{"data":{"trusted-resources-verification-no-match-policy":"fail"}}
```

 #### TaskRun and PipelineRun status update
Trusted resources will update the taskrun's [condition](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties) to indicate if it passes verification or not.

The following tables illustrate how the conditions are impacted by feature flag and verification result. Note that if not `true` or `false` means this case doesn't update the corresponding condition.
**No Matching Policies:**
|                             | `Conditions.TrustedResourcesVerified` | `Conditions.Succeeded` |
|-----------------------------|---------------------------------------|------------------------|
| `no-match-policy`: "ignore" |                                       |                        |
| `no-match-policy`: "warn"   | False                                 |                        |
| `no-match-policy`: "fail"   | False                                 | False                  |

**Matching Policies(no matter what `trusted-resources-verification-no-match-policy` value is):**
|                          | `Conditions.TrustedResourcesVerified` | `Conditions.Succeeded` |
|--------------------------|---------------------------------------|------------------------|
| all policies pass        | True                                  |                        |
| any enforce policy fails | False                                 | False                  |
| only warn policies fail  | False                                 |                        |


A successful sample `TrustedResourcesVerified` condition is:
```yaml
status:
  conditions:
  - lastTransitionTime: "2023-03-01T18:17:05Z"
    message: Trusted resource verification passed
    status: "True"
    type: TrustedResourcesVerified
```

Failed sample `TrustedResourcesVerified` and `Succeeded` conditions are:
```yaml
status:
  conditions:
  - lastTransitionTime: "2023-03-01T18:17:05Z"
    message: resource verification failed # This will be filled with detailed error message.
    status: "False"
    type: TrustedResourcesVerified
  - lastTransitionTime: "2023-03-01T18:17:10Z"
    message: resource verification failed
    status: "False"
    type: Succeeded
```

#### Config key at VerificationPolicy
VerificationPolicy supports SecretRef or encoded public key data.

How does VerificationPolicy work?
You can create multiple `VerificationPolicy` and apply them to the cluster.
1. Trusted resources will look up policies from the resource namespace (usually this is the same as taskrun/pipelinerun namespace).
2. If multiple policies are found. For each policy we will check if the resource url is matching any of the `patterns` in the `resources` list. If matched then this policy will be used for verification.
3. If multiple policies are matched, the resource must pass all the "enforce" mode policies. If the resource only matches policies in "warn" mode and fails to pass the "warn" policy, it will not fail the taskrun or pipelinerun, but log a warning instead.

4. To pass one policy, the resource can pass any public keys in the policy.

Take the following `VerificationPolicies` for example, a resource from "https://github.com/tektoncd/catalog.git", needs to pass both `verification-policy-a` and `verification-policy-b`, to pass `verification-policy-a` the resource needs to pass either `key1` or `key2`.

Example:
```yaml
apiVersion: tekton.dev/v1alpha1
kind: VerificationPolicy
metadata:
  name: verification-policy-a
  namespace: resource-namespace
spec:
  # resources defines a list of patterns
  resources:
    - pattern: "https://github.com/tektoncd/catalog.git"  #git resource pattern
    - pattern: "gcr.io/tekton-releases/catalog/upstream/git-clone"  # bundle resource pattern
    - pattern: " https://artifacthub.io/"  # hub resource pattern
  # authorities defines a list of public keys
  authorities:
    - name: key1
      key:
        # secretRef refers to a secret in the cluster, this secret should contain public keys data
        secretRef:
          name: secret-name-a
          namespace: secret-namespace
        hashAlgorithm: sha256
    - name: key2
      key:
        # data stores the inline public key data
        data: "STRING_ENCODED_PUBLIC_KEY"
  # mode can be set to "enforce" (default) or "warn".
  mode: enforce
```

```yaml
apiVersion: tekton.dev/v1alpha1
kind: VerificationPolicy
metadata:
  name: verification-policy-b
  namespace: resource-namespace
spec:
  resources:
    - pattern: "https://github.com/tektoncd/catalog.git"
  authorities:
    - name: key3
      key:
        # data stores the inline public key data
        data: "STRING_ENCODED_PUBLIC_KEY"
```

`namespace` should be the same of corresponding resources' namespace.

`pattern` is used to filter out remote resources by their sources URL. e.g. git resources pattern can be set to https://github.com/tektoncd/catalog.git. The `pattern` should follow regex schema, we use go regex library's [`Match`](https://pkg.go.dev/regexp#Match) to match the pattern from VerificationPolicy to the `ConfigSource` URL resolved by remote resolution. Note that `.*` will match all resources.
To learn more about regex syntax please refer to [syntax](https://pkg.go.dev/regexp/syntax).
To learn more about `ConfigSource` please refer to resolvers doc for more context. e.g. [gitresolver](./git-resolver.md)

 `key` is used to store the public key, `key` can be configured with `secretRef`, `data`, `kms` note that only 1 of these 3 fields can be configured.

  * `secretRef`: refers to secret in cluster to store the public key.
  * `data`: contains the inline data of the pubic key in "PEM-encoded byte slice" format.
  * `kms`: refers to the uri of the public key, it should follow the format defined in [sigstore](https://docs.sigstore.dev/cosign/kms_support).

`hashAlgorithm` is the algorithm for the public key, by default is `sha256`. It also supports `SHA224`, `SHA384`, `SHA512`.

`mode` controls whether a failing policy will fail the taskrun/pipelinerun, or only log the a warning
 * enforce (default) - fail the taskrun/pipelinerun if verification fails
 * warn - don't fail the taskrun/pipelinerun if verification fails but log a warning

#### Migrate Config key at configmap to VerificationPolicy
**Note:** key configuration in configmap is deprecated,
The following usage of public keys in configmap can be migrated to VerificationPolicy/

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

To migrate to VerificationPolicy: Stores the public key files in a secret, and configure the secret ref in VerificationPolicy

```yaml
apiVersion: tekton.dev/v1alpha1
kind: VerificationPolicy
metadata:
  name: verification-policy-name
  namespace: resource-namespace
spec:
  authorities:
    - name: key1
      key:
        # secretRef refers to a secret in the cluster, this secret should contain public keys data
        secretRef:
          name: secret-name-cosign
          namespace: secret-namespace
        hashAlgorithm: sha256
    - name: key2
      key:
        secretRef:
          name: secret-name-cosign2
          namespace: secret-namespace
        hashAlgorithm: sha256
```
