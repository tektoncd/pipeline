<!--
---
linkTitle: "PSP migration"
weight: 1400
---
-->
# PodSecurityPolicy Migration

> :warning: **`PodSecurityPolicy(PSP)` is [deprecated](https://kubernetes.io/blog/2021/04/08/kubernetes-1-21-release-announcement/#podsecuritypolicy-deprecation).**
> 
> It was deprecated in Kubernetes v1.21 and removed from Kubernetes in v1.25 and has been tracked down in the [issue](https://github.com/tektoncd/pipeline/issues/4112).
<br/><br/>*This guide aims to point out the ways of the 3 alternatives given the context of the deprecation and to weigh the pros and cons among them.*

- [Context of PodSecurityPolicy Deprecation](#context-of-podsecuritypolicy-deprecation)
- [Weighing among the 3 Alternatives](#weighing-among-the-3-alternatives)
- [Policies not covered by PSA in PSP](#policies-not-covered-by-psa-in-psp)
<br/><br/>


## Context of PSP Deprecation
- ### PodSecurityPolicy(PSP)

PSP is a built-in admission controller that allows a cluster administrator to control security-sensitive aspects of the Pod specification. It has been provided as reference in the Tekton pipeline for a cluster-level resource that controls the actions that a pod can perform and what it can access. The objects in PSP as in [101-podsecuritypolicy.yaml](./../config/101-podsecuritypolicy.yaml) define a set of conditions that a pod must run with in order to be accepted into the system. 
 
- ### [Pod Security Admission](https://kubernetes.io/docs/concepts/security/pod-security-admission/) (PSA) 

PSA places requirements on a Pod's Security Context and other related fields according to the three levels defined by the Pod Security Standards: privileged, baseline, and restricted.

- ### [Open Policy Agent](https://www.openpolicyagent.org/) (OPA):

The Open Policy Agent is an open source, general-purpose policy engine that unifies policy enforcement across the stack. OPA provides a high-level declarative language that lets you specify policy as code and simple APIs to offload policy decision-making from your software.
<br/><br/>

## Weighing among the 3 Alternatives

| Alternatives | Pros | Cons |
| ------------ | ---- | ---- |
| Migrating to PSA | As suggested in kubernetes doc, can be used with OPA as a complement. | PSA  is less  granular than PSP. There [policies that are not matched up with PSP](#policies-not-covered-by-psa-in-psp) |
| Using OPA        | Could be more granularly controlled by users | It introduces additional dependencies |
| Removing PSP     |  | Users have to enforce securities policies in their own ways |

- ### Migrating to PSA
    The Kubernetes official guide suggests going the way of migrating to PSA with OPA as compelment according to https://kubernetes.io/docs/tasks/configure-pod-container/migrate-from-psp/.

    TODO: add a specific case according to our 101-psp.yaml

- ### Using OPA
    With OPA, the built-in checks and customized specifications can be enforced on resources configurations that are even more granular and specific than PSP.
    https://www.openpolicyagent.org/ could be used as a guide for the migration.

- ### Removing PSP
    To remove PSP entirely, the user would need to enforce all security policies on their own approach.


<br/><br/>

## Policies not covered by PSA in PSP
- ### seLinux ###
    For PSP, the 'seLinux' rule is set to 'RunAsAny', with no default provided. Allows any `seLinuxOptions` to be specified. However for PSA, this would require the `Privileged` level as at the `Baseline` level, setting the SELinux type is restricted, and setting a custom SELinux user or role option is forbidden. This shall require the 'privileged' level

- ### supplementalGroups ###
    Set as a range with `mustRunAs` rule in PSP but not specified in PSA

- ### fsGroup ###
    Set as a range with `mustRunAs` rule in PSP but not specified in PSA
