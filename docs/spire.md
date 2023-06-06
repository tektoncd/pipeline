<!--
---
linkTitle: "TaskRun Result Attestation"
weight: 1660
---
-->
⚠️ This is a work in progress: SPIRE support is not yet functional

TaskRun result attestations is currently an alpha experimental feature. Currently all that is implemented is support for configuring Tekton to connect to SPIRE and enabling TaskRun to sign and verify the TaskRunStatus. See [TEP-0089](https://github.com/tektoncd/community/blob/main/teps/0089-nonfalsifiable-provenance-support.md) for details on the overall design and feature set.

This being a large feature, this will be implemented in the following phases. This document will be updated as we implement new phases.
1.  Add a client for SPIRE (done).
2.  Add a configMap which initializes SPIRE (done).
3.  Modify TaskRun to sign and verify TaskRunStatus using SPIRE (done). 
4.  Enabling Chains to verify the TaskRun Results.

When the TaskRun result attestations feature is [enabled](./spire.md#enabling-taskrun-result-attestations) all TaskRuns will produce a signature alongside its results, which can then be used to validate its provenance. For example, a TaskRun result that creates user-specified results `commit` and `url` would look like the following. `SVID`, `RESULT_MANIFEST`, `RESULT_MANIFEST.sig`, `commit.sig` and `url.sig` are generated attestations by the integration of SPIRE and Tekton Controller.

Parsed, the fields would be:
```
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

However, the verification materials are removed from the final results as part of the TaskRun status. It is stored in the termination messages (more details below):

```
$ tkn tr describe cache-image-pipelinerun-8dq9c-fetch-from-git
...
<truncated>
...
📝 Results
 NAME                    VALUE
 ∙ commit       aa79de59c4bae24e32f15fda467d02ae9cd94b01
 ∙ url          https://github.com/buildpacks/samples
```

## Architecture Overview

This feature relies on a SPIRE installation. This is how it integrates into the architecture of Tekton:

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

## Enabling TaskRun result attestations

To enable TaskRun attestations:
1. Make sure `enforce-nonfalsifiability` is set to `"spire"` and `enable-api-fields` is set to `"alpha"` in the `feature-flags` configmap, see [`install.md`](./install.md#customizing-the-pipelines-controller-behavior) for details
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
1. (Optional) Modify the configmap (`config/config-spire.yaml`) to configure non-default SPIRE options.
    ```yaml
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: config-spire
      namespace: tekton-pipelines
    labels:
      app.kubernetes.io/instance: default
      app.kubernetes.io/part-of: tekton-pipelines
    data:
      # More explanation about the fields is at the SPIRE Server Configuration file
      # https://spiffe.io/docs/latest/deploying/spire_server/#server-configuration-file
      # spire-trust-domain specifies the SPIRE trust domain to use.
      spire-trust-domain: "example.org"
      # spire-socket-path specifies the SPIRE agent socket for SPIFFE workload API.
      spire-socket-path: "unix:///spiffe-workload-api/spire-agent.sock"
      # spire-server-addr specifies the SPIRE server address for workload/node registration.
      spire-server-addr: "spire-server.spire.svc.cluster.local:8081"
      # spire-node-alias-prefix specifies the SPIRE node alias prefix to use.
      spire-node-alias-prefix: "/tekton-node/"
    ```

## Sample TaskRun attestation

The following example shows how this feature works:

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


The termination message is:
```
message: '[{"key":"RESULT_MANIFEST","value":"foo,bar","type":1},{"key":"RESULT_MANIFEST.sig","value":"MEQCIB4grfqBkcsGuVyoQd9KUVzNZaFGN6jQOKK90p5HWHqeAiB7yZerDA+YE3Af/ALG43DQzygiBpKhTt8gzWGmpvXJFw==","type":1},{"key":"SVID","value":"-----BEGIN
        CERTIFICATE-----\nMIICCjCCAbCgAwIBAgIRALH94zAZZXdtPg97O5vG5M0wCgYIKoZIzj0EAwIwHjEL\nMAkGA1UEBhMCVVMxDzANBgNVBAoTBlNQSUZGRTAeFw0yMjAzMTQxNTUzNTlaFw0y\nMjAzMTQxNjU0MDlaMB0xCzAJBgNVBAYTAlVTMQ4wDAYDVQQKEwVTUElSRTBZMBMG\nByqGSM49AgEGCCqGSM49AwEHA0IABPLzFTDY0RDpjKb+eZCIWgUw9DViu8/pM8q7\nHMTKCzlyGqhaU80sASZfpkZvmi72w+gLszzwVI1ZNU5e7aCzbtSjgc8wgcwwDgYD\nVR0PAQH/BAQDAgOoMB0GA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAMBgNV\nHRMBAf8EAjAAMB0GA1UdDgQWBBSsUvspy+/Dl24pA1f+JuNVJrjgmTAfBgNVHSME\nGDAWgBSOMyOHnyLLGxPSD9RRFL+Yhm/6qzBNBgNVHREERjBEhkJzcGlmZmU6Ly9l\neGFtcGxlLm9yZy9ucy9kZWZhdWx0L3Rhc2tydW4vbm9uLWZhbHNpZmlhYmxlLXBy\nb3ZlbmFuY2UwCgYIKoZIzj0EAwIDSAAwRQIhAM4/bPAH9dyhBEj3DbwtJKMyEI56\n4DVrP97ps9QYQb23AiBiXWrQkvRYl0h4CX0lveND2yfqLrGdVL405O5NzCcUrA==\n-----END
        CERTIFICATE-----\n","type":1},{"key":"bar","value":"world","type":1},{"key":"bar.sig","value":"MEUCIQDOtg+aEP1FCr6/FsHX+bY1d5abSQn2kTiUMg4Uic2lVQIgTVF5bbT/O77VxESSMtQlpBreMyw2GmKX2hYJlaOEH1M=","type":1},{"key":"foo","value":"hello","type":1},{"key":"foo.sig","value":"MEQCIBr+k0i7SRSyb4h96vQE9hhxBZiZb/2PXQqReOKJDl/rAiBrjgSsalwOvN0zgQay0xQ7PRbm5YSmI8tvKseLR8Ryww==","type":1}]'
```

Parsed, the fields are:
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


However, the verification materials are removed from the results as part of the TaskRun status:
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


# TaskRun Status attestations

Each TaskRun status that is written by the tekton-pipelines-controller will be signed to ensure that there is no external
tampering of the TaskRun status. Upon each retrieval of the TaskRun, the tekton-pipelines-controller checks if the status is initialized,
and that the signature validates the current status.
The signature and SVID will be stored as annotations on the TaskRun Status field, and can be verified by a client.

The verification is done on every consumption of the TaskRun except when the TaskRun is uninitialized. When uninitialized, the 
tekton-pipelines-controller is not influenced by fields in the status and thus will not sign incorrect reflections of the TaskRun.

The spec and TaskRun annotations/labels are not signed when there are valid interactions from other controllers or users (i.e. cancelling taskrun).
Editing the object annotations/labels or spec will not result in any unverifiable outcome of the status field.

As the TaskRun progresses, the Pipeline Controller will reconcile the TaskRun object and continually verify the current hash against the `tekton.dev/status-hash-sig` before updating the hash to match the new status and creating a new signature.

An example TaskRun annotations would be:

```console
$ tkn tr describe non-falsifiable-provenance -oyaml
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  annotations:
    pipeline.tekton.dev/release: 3ee99ec
  creationTimestamp: "2022-03-04T19:10:46Z"
  generation: 1
  labels:
    app.kubernetes.io/managed-by: tekton-pipelines
  name: non-falsifiable-provenance
  namespace: default
  resourceVersion: "23088242"
  uid: 548ebe99-d40b-4580-a9bc-afe80915e22e
spec:
  serviceAccountName: default
  taskSpec:
    results:
    - description: ""
      name: foo
    - description: ""
      name: bar
    steps:
    - image: ubuntu
      name: non-falsifiable
      resources: {}
      script: |
        #!/usr/bin/env bash
        sleep 30
        printf "%s" "hello" > "$(results.foo.path)"
        printf "%s" "world" > "$(results.bar.path)"
  timeout: 1m0s
status:
  annotations:
    tekton.dev/controller-svid: |
      -----BEGIN CERTIFICATE-----
      MIIB7jCCAZSgAwIBAgIRAI8/08uXSn9tyv7cRN87uvgwCgYIKoZIzj0EAwIwHjEL
      MAkGA1UEBhMCVVMxDzANBgNVBAoTBlNQSUZGRTAeFw0yMjAzMDQxODU0NTlaFw0y
      MjAzMDQxOTU1MDlaMB0xCzAJBgNVBAYTAlVTMQ4wDAYDVQQKEwVTUElSRTBZMBMG
      ByqGSM49AgEGCCqGSM49AwEHA0IABL+e9OjkMv+7XgMWYtrzq0ESzJi+znA/Pm8D
      nvApAHg3/rEcNS8c5LgFFRzDfcs9fxGSSkL1JrELzoYul1Q13XejgbMwgbAwDgYD
      VR0PAQH/BAQDAgOoMB0GA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAMBgNV
      HRMBAf8EAjAAMB0GA1UdDgQWBBR+ma+yZfo092FKIM4F3yhEY8jgDDAfBgNVHSME
      GDAWgBRKiCg5+YdTaQ+5gJmvt2QcDkQ6KjAxBgNVHREEKjAohiZzcGlmZmU6Ly9l
      eGFtcGxlLm9yZy90ZWt0b24vY29udHJvbGxlcjAKBggqhkjOPQQDAgNIADBFAiEA
      8xVWrQr8+i6yMLDm9IUjtvTbz9ofjSsWL6c/+rxmmRYCIBTiJ/HW7di3inSfxwqK
      5DKyPrKoR8sq8Ne7flkhgbkg
      -----END CERTIFICATE-----
    tekton.dev/status-hash: 76692c9dcd362f8a6e4bda8ccb4c0937ad16b0d23149ae256049433192892511
    tekton.dev/status-hash-sig: MEQCIFv2bW0k4g0Azx+qaeZjUulPD8Ma3uCUn0tXQuuR1FaEAiBHQwN4XobOXmC2nddYm04AZ74YubUyNl49/vnbnR/HcQ==
  completionTime: "2022-03-04T19:11:22Z"
  conditions:
  - lastTransitionTime: "2022-03-04T19:11:22Z"
    message: All Steps have completed executing
    reason: Succeeded
    status: "True"
    type: Succeeded
  - lastTransitionTime: "2022-03-04T19:11:22Z"
    message: Spire verified
    reason: TaskRunResultsVerified
    status: "True"
    type: SignedResultsVerified
  podName: non-falsifiable-provenance-pod
  startTime: "2022-03-04T19:10:46Z"
  steps:
  ...
  <TRUNCATED>
```

## How is the status being verified

The signature are being verified by the Tekton controller, the process of verification is as follows:

- Verify status-hash fields
  - verify `tekton.dev/status-hash` content against its associated `tekton.dev/status-hash-sig` field. If status hash does 
    not match invalidate the `tekton.dev/verified = no` annotation will be added

## Further Details

To learn more about SPIRE attestations, check out the [TEP](https://github.com/tektoncd/community/blob/main/teps/0089-nonfalsifiable-provenance-support.md).
