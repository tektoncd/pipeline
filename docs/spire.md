<!--
---
linkTitle: "TaskRun Result Attestation"
weight: 1660
---
-->
⚠️ This is a work in progress: SPIRE support is not yet functional

TaskRun result attestations is currently an alpha experimental feature. Currently all that is implemented is support for configuring Tekton to connect to SPIRE. See TEP-0089 for details on the overall design and feature set.

This being a large feature, this will be implemented in the following phases. This document will be updated as we implement new phases.
1.  Add a client for SPIRE (done).
2.  Add a configMap which initializes SPIRE (in progress).
3.  Modify TaskRun to sign and verify TaskRun Results using SPIRE.
4.  Modify Tekton Chains to verify the TaskRun Results.

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
