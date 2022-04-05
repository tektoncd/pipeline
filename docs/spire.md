<!--
---
linkTitle: "SPIRE"
weight: 1660
---
-->
# SPIRE TaskRun result attestations

The SPIRE TaskRun result attestations feature provides the non-falsifiable provenance to the build processes that run in the pipeline. They ensure that the results of the tekton pipeline executions originate from the build workloads themselves and that they have not been tampered. 

When SPIRE TaskRun result attestations is enabled, all TaskRuns will produce a signature alongside its results, which can then be used to validate its provenance. For example, a TaskRun result would be:
```
$ tkn tr describe cache-image-pipelinerun-8dq9c-fetch-from-git
...
<truncated>
...
📝 Results

 NAME                    VALUE
 ∙ RESULT_MANIFEST       commit,url,SVID,commit.sig,url.sig
 ∙ RESULT_MANIFEST.sig   MEUCIQD55MMII9SEk/esQvwNLGC43y7efNGZ+7fsTdq+9vXYFAIgNoRW7cV9WKriZkcHETIaAKqfcZVJfsKbEmaDyohDSm4=
 ∙ SVID                  -----BEGIN CERTIFICATE-----
MIICGzCCAcGgAwIBAgIQH9VkLxKkYMidPIsofckRQTAKBggqhkjOPQQDAjAeMQsw
CQYDVQQGEwJVUzEPMA0GA1UEChMGU1BJRkZFMB4XDTIyMDIxMTE2MzM1MFoXDTIy
MDIxMTE3MzQwMFowHTELMAkGA1UEBhMCVVMxDjAMBgNVBAoTBVNQSVJFMFkwEwYH
KoZIzj0CAQYIKoZIzj0DAQcDQgAEBRdg3LdxVAELeH+lq8wzdEJd4Gnt+m9G0Qhy
NyWoPmFUaj9vPpvOyRgzxChYnW0xpcDWihJBkq/EbusPvQB8CKOB4TCB3jAOBgNV
HQ8BAf8EBAMCA6gwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMAwGA1Ud
EwEB/wQCMAAwHQYDVR0OBBYEFID7ARM5+vwzvnLPMO7Icfnj7l7hMB8GA1UdIwQY
MBaAFES3IzpGDqgV3QcQNgX8b/MBwyAtMF8GA1UdEQRYMFaGVHNwaWZmZTovL2V4
YW1wbGUub3JnL25zL2RlZmF1bHQvdGFza3J1bi9jYWNoZS1pbWFnZS1waXBlbGlu
ZXJ1bi04ZHE5Yy1mZXRjaC1mcm9tLWdpdDAKBggqhkjOPQQDAgNIADBFAiEAi+LR
JkrZn93PZPslaFmcrQw3rVcEa4xKmPleSvQaBoACIF1QB+q1uwH6cNvWdbLK9g+W
T9Np18bK0xc6p5SuTM2C
-----END CERTIFICATE-----
 ∙ commit       aa79de59c4bae24e32f15fda467d02ae9cd94b01
 ∙ commit.sig   MEQCIEJHk+8B+mCFozp0F52TQ1AadlhEo1lZNOiOnb/ht71aAiBCE0otKB1R0BktlPvweFPldfZfjG0F+NUSc2gPzhErzg==
 ∙ url          https://github.com/buildpacks/samples
 ∙ url.sig      MEUCIF0Fuxr6lv1MmkreqDKcPH3m+eXp+gY++VcxWgGCx7T1AiEA9U/tROrKuCGfKApLq2A9EModbdoGXyQXFOpAa0aMpOg=
```

SPIRE TaskRun result attestations is currently an alpha experimental feature. 

## Architecture Overview

Since this feature relies on a SPIRE installation, we will show how it integrates into the architecture of Tekton.

```
┌─────────────┐  Register TaskRun Workload Identity           ┌──────────┐
│             ├──────────────────────────────────────────────►│          │
│  Tekton     │                                               │  SPIRE   │
│  Controller │◄───────────┐                                  │  Server  │
│             │            │ Listen on TaskRun                │          │
└────────────┬┘            │                                  └──────────┘
 ▲           │     ┌───────┴───────────────────────────────┐     ▲
 │           │     │           Tekton TaskRun              │     │
 │           │     └───────────────────────────────────────┘     │
 │  Configure│                                          ▲        │ Attest
 │  Pod &    │                                          │        │   +
 │  check    │                                          │        │ Request
 │  ready    │     ┌───────────┐                        │        │ SVIDs
 │           └────►│  TaskRun  ├────────────────────────┘        │
 │                 │  Pod      │                                 │
 │                 └───────────┘     TaskRun Entrypointer        │
 │                   ▲               Sign Result and update      │
 │ Get               │ Get SVID      TaskRun status with         │
 │ SPIRE             │               signature + cert            │
 │ server            │                                           │
 │ Credentials       │                                           ▼
┌┴───────────────────┴─────────────────────────────────────────────────────┐
│                                                                          │
│   SPIRE Agent    ( Runs as   )                                           │
│   + CSI Driver   ( Daemonset )                                           │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘
```

Initial Setup:
1. As part of the SPIRE deployment, the SPIRE server attests the agents running on each node in the cluster.
1. The Tekton Controller is configured to have workload identity entry creation permissions to the SPIRE server.
1. As part of the Tekton Controller operations, the Tekton Controller will retrieve an identity that it can use to talk to the SPIRE server to register TaskRun workloads.

When a TaskRun is created:
1. The Tekton Controller creates a TaskRun pod and its associated resources
1. When the TaskRun pod is ready, the Tekton Controller registers an identity with the information of the pod to the SPIRE server. This will tell the SPIRE server the identity of the TaskRun to use as well as how to attest the workload/pod.
1. After the TaskRun steps complete, as part of the entrypointer code, it requests an SVID from SPIFFE workload API (via the SPIRE agent socket)
1. The SPIRE agent will attest the workload and request an SVID.
1. The entrypointer receives an x509 SVID, containing the x509 certificate and associated private key. 
1. The entrypointer signs the results of the TaskRun and emits the signatures and x509 certificate to the TaskRun results for later verification.

## Enabling SPIRE TaskRun result attestations
To enable SPIRE TaskRun attestations:
1. Make sure `enable-spire` is set to `"true"` in the `feature-flags` configmap, see [`install.md`](./install.md#customizing-the-pipelines-controller-behavior) for details
1. Create a SPIRE deployment containing a SPIRE server, SPIRE agents and the SPIRE CSI driver, for convenience, [this sample single cluster deployment](https://github.com/spiffe/spiffe-csi/tree/main/example/config) can be used.
1. Register the SPIRE workload entry for Tekton with the "Admin" flag, which will allow the Tekton controller to communicate with the SPIRE server to manage the TaskRun identities dynamically.
    ```
    # This example is assuming use of the above SPIRE deployment
    # Example where trust domain is "example.org" and cluster name is "example-cluster"
    
    # Register a node alias for all nodes of which the Tekton Controller may reside
    kubectl -n spire exec -it \
        deployment/spire-server -- \
        /opt/spire/bin/spire-server entry create \
            -node \
            -spiffeID spiffe://example.org/allnodes \
            -selector k8s_psat:cluster:example-cluster
    
    # Register the tekton controller workload to have access to creating entries in the SPIRE server
    kubectl -n spire exec -it \
        deployment/spire-server -- \
        /opt/spire/bin/spire-server entry create \
            -admin \
            -spiffeID spiffe://example.org/tekton/controller \
            -parentID spiffe://example.org/allnode \
            -selector k8s:ns:tekton-pipelines \
            -selector k8s:pod-label:app:tekton-pipelines-controller \
            -selector k8s:sa:tekton-pipelines-controller
    
    ```
1. Modify the controller (`config/controller.yaml`) to provide access to the SPIRE agent socket.
    ```yaml
    # Add the following the volumeMounts of the "tekton-pipelines-controller" container
    - name: spiffe-workload-api
      mountPath: /spiffe-workload-api
      readOnly: true
    
    # Add the following to the volumes of the controller pod
    - name: spiffe-workload-api
      csi:
        driver: "csi.spiffe.io"
    ```
1. (Optional) Modify the controller (`config/controller.yaml`) to configure non-default SPIRE options by adding arguments to the CLI.
    ```yaml
          containers:
          - name: tekton-pipelines-controller
            image: ko://github.com/tektoncd/pipeline/cmd/controller
            args: [
              # These images are built on-demand by `ko resolve` and are replaced
              # by image references by digest.
              "-kubeconfig-writer-image", "ko://github.com/tektoncd/pipeline/cmd/kubeconfigwriter",
              "-git-image", "ko://github.com/tektoncd/pipeline/cmd/git-init",
              "-entrypoint-image", "ko://github.com/tektoncd/pipeline/cmd/entrypoint",
              "-nop-image", "ko://github.com/tektoncd/pipeline/cmd/nop",
              "-imagedigest-exporter-image", "ko://github.com/tektoncd/pipeline/cmd/imagedigestexporter",
              "-pr-image", "ko://github.com/tektoncd/pipeline/cmd/pullrequest-init",
              "-workingdirinit-image", "ko://github.com/tektoncd/pipeline/cmd/workingdirinit",
    
              # Configure optional SPIRE arguments
    +         "-spire-trust-domain", "example.org",
    +         "-spire-socket-path", "/spiffe-workload-api/spire-agent.sock",
    +         "spire-server-addr", "spire-server.spire.svc.cluster.local:8081"
    +         "spire-node-alias-prefix", "/tekton-node/",
    
              # This is gcr.io/google.com/cloudsdktool/cloud-sdk:302.0.0-slim
              "-gsutil-image", "gcr.io/google.com/cloudsdktool/cloud-sdk@sha256:27b2c22bf259d9bc1a291e99c63791ba0c27a04d2db0a43241ba0f1f20f4067f",
              # The shell image must be root in order to create directories and copy files to PVCs.
              # gcr.io/distroless/base:debug as of October 21, 2021
              # image shall not contains tag, so it will be supported on a runtime like cri-o
              "-shell-image", "gcr.io/distroless/base@sha256:cfdc553400d41b47fd231b028403469811fcdbc0e69d66ea8030c5a0b5fbac2b",
              # for script mode to work with windows we need a powershell image
              # pinning to nanoserver tag as of July 15 2021
              "-shell-image-win", "mcr.microsoft.com/powershell:nanoserver@sha256:b6d5ff841b78bdf2dfed7550000fd4f3437385b8fa686ec0f010be24777654d6",
            ]
    ```

## Sample TaskRun attestation

To demonstrate, we will use a simple task run that writes some results:

```yaml
kind: TaskRun
apiVersion: tekton.dev/v1beta1
metadata:
  name: non-falsifiable-provenance
spec:
  timeout: 60s
  taskSpec:
    steps:
    - name: non-falsifiable
      image: ubuntu
      script: |
        #!/usr/bin/env bash
        printf "%s" "hello" > "$(results.foo.path)"
        printf "%s" "world" > "$(results.bar.path)"
    results:
    - name: foo
    - name: bar
```


An example termination message would be:
```
message: '[{"key":"RESULT_MANIFEST","value":"foo,bar","type":1},{"key":"RESULT_MANIFEST.sig","value":"MEQCIB4grfqBkcsGuVyoQd9KUVzNZaFGN6jQOKK90p5HWHqeAiB7yZerDA+YE3Af/ALG43DQzygiBpKhTt8gzWGmpvXJFw==","type":1},{"key":"SVID","value":"-----BEGIN
        CERTIFICATE-----\nMIICCjCCAbCgAwIBAgIRALH94zAZZXdtPg97O5vG5M0wCgYIKoZIzj0EAwIwHjEL\nMAkGA1UEBhMCVVMxDzANBgNVBAoTBlNQSUZGRTAeFw0yMjAzMTQxNTUzNTlaFw0y\nMjAzMTQxNjU0MDlaMB0xCzAJBgNVBAYTAlVTMQ4wDAYDVQQKEwVTUElSRTBZMBMG\nByqGSM49AgEGCCqGSM49AwEHA0IABPLzFTDY0RDpjKb+eZCIWgUw9DViu8/pM8q7\nHMTKCzlyGqhaU80sASZfpkZvmi72w+gLszzwVI1ZNU5e7aCzbtSjgc8wgcwwDgYD\nVR0PAQH/BAQDAgOoMB0GA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAMBgNV\nHRMBAf8EAjAAMB0GA1UdDgQWBBSsUvspy+/Dl24pA1f+JuNVJrjgmTAfBgNVHSME\nGDAWgBSOMyOHnyLLGxPSD9RRFL+Yhm/6qzBNBgNVHREERjBEhkJzcGlmZmU6Ly9l\neGFtcGxlLm9yZy9ucy9kZWZhdWx0L3Rhc2tydW4vbm9uLWZhbHNpZmlhYmxlLXBy\nb3ZlbmFuY2UwCgYIKoZIzj0EAwIDSAAwRQIhAM4/bPAH9dyhBEj3DbwtJKMyEI56\n4DVrP97ps9QYQb23AiBiXWrQkvRYl0h4CX0lveND2yfqLrGdVL405O5NzCcUrA==\n-----END
        CERTIFICATE-----\n","type":1},{"key":"bar","value":"world","type":1},{"key":"bar.sig","value":"MEUCIQDOtg+aEP1FCr6/FsHX+bY1d5abSQn2kTiUMg4Uic2lVQIgTVF5bbT/O77VxESSMtQlpBreMyw2GmKX2hYJlaOEH1M=","type":1},{"key":"foo","value":"hello","type":1},{"key":"foo.sig","value":"MEQCIBr+k0i7SRSyb4h96vQE9hhxBZiZb/2PXQqReOKJDl/rAiBrjgSsalwOvN0zgQay0xQ7PRbm5YSmI8tvKseLR8Ryww==","type":1}]'
```

Parsed, the fields woud be:
- `RESULT_MANIFEST`: List of results that should be present, to prevent pick and choose attacks
- `RESULT_MANIFEST.sig`: The signature of the result manifest
- `SVID`: The x509 certificate that will be used to verify the signature trust chain to the authority
- `*.sig`: The signature of each individual result output
```
 ∙ RESULT_MANIFEST       foo,bar
 ∙ RESULT_MANIFEST.sig   MEQCIB4grfqBkcsGuVyoQd9KUVzNZaFGN6jQOKK90p5HWHqeAiB7yZerDA+YE3Af/ALG43DQzygiBpKhTt8gzWGmpvXJFw==
 ∙ SVID                  -----BEGIN CERTIFICATE-----
MIICCjCCAbCgAwIBAgIRALH94zAZZXdtPg97O5vG5M0wCgYIKoZIzj0EAwIwHjEL
MAkGA1UEBhMCVVMxDzANBgNVBAoTBlNQSUZGRTAeFw0yMjAzMTQxNTUzNTlaFw0y
MjAzMTQxNjU0MDlaMB0xCzAJBgNVBAYTAlVTMQ4wDAYDVQQKEwVTUElSRTBZMBMG
ByqGSM49AgEGCCqGSM49AwEHA0IABPLzFTDY0RDpjKb+eZCIWgUw9DViu8/pM8q7
HMTKCzlyGqhaU80sASZfpkZvmi72w+gLszzwVI1ZNU5e7aCzbtSjgc8wgcwwDgYD
VR0PAQH/BAQDAgOoMB0GA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAMBgNV
HRMBAf8EAjAAMB0GA1UdDgQWBBSsUvspy+/Dl24pA1f+JuNVJrjgmTAfBgNVHSME
GDAWgBSOMyOHnyLLGxPSD9RRFL+Yhm/6qzBNBgNVHREERjBEhkJzcGlmZmU6Ly9l
eGFtcGxlLm9yZy9ucy9kZWZhdWx0L3Rhc2tydW4vbm9uLWZhbHNpZmlhYmxlLXBy
b3ZlbmFuY2UwCgYIKoZIzj0EAwIDSAAwRQIhAM4/bPAH9dyhBEj3DbwtJKMyEI56
4DVrP97ps9QYQb23AiBiXWrQkvRYl0h4CX0lveND2yfqLrGdVL405O5NzCcUrA==
-----END CERTIFICATE-----
 ∙ bar       world
 ∙ bar.sig   MEUCIQDOtg+aEP1FCr6/FsHX+bY1d5abSQn2kTiUMg4Uic2lVQIgTVF5bbT/O77VxESSMtQlpBreMyw2GmKX2hYJlaOEH1M=
 ∙ foo       hello
 ∙ foo.sig   MEQCIBr+k0i7SRSyb4h96vQE9hhxBZiZb/2PXQqReOKJDl/rAiBrjgSsalwOvN0zgQay0xQ7PRbm5YSmI8tvKseLR8Ryww==
```


However, the verification material be removed from the results as part of the TaskRun status:
```console
$ tkn tr describe non-falsifiable-provenance
Name:              non-falsifiable-provenance
Namespace:         default
Service Account:   default
Timeout:           1m0s
Labels:
 app.kubernetes.io/managed-by=tekton-pipelines

🌡️  Status

STARTED          DURATION     STATUS
38 seconds ago   36 seconds   Succeeded

📝 Results

 NAME        VALUE
 ∙ bar       world
 ∙ foo       hello

🦶 Steps

 NAME                STATUS
 ∙ non-falsifiable   Completed
```

## How is the result being verified

The signatures are being verified by the Tekton controller, the process of verification is as follows:

- Verifying the SVID
  - Obtain the trust bundle from the SPIRE server
  - Verify the SVID with the trust bundle
  - Verify that the SVID spiffe ID is for the correct TaskRun
- Verifying the result manifest
  - Verify the content of `RESULT_MANIFEST` with the field `RESULT_MANIFEST.sig` with the SVID public key
  - Verify that there is a corresponding field for all items listed in `RESULT_MANIFEST` (besides SVID and `*.sig` fields)
- Verify individual result fields
  - For each of the items in the results, verify its content against its associated `.sig` field


## Further Details

To learn more about SPIRE TaskRun attestations, check out the [TEP](https://github.com/tektoncd/community/blob/main/teps/0089-nonfalsifiable-provenance-support.md).
