# Tekton Security Threat Model

This document defines Tekton's security boundaries, clarifies what the project is and is not responsible for enforcing, and guides security researchers on what to report.

If you have found something you believe is a vulnerability, please read this document before filing a report. Many reports we receive describe expected behavior that is intentional by design or is the responsibility of the Kubernetes cluster operator, not Tekton.

---

## How Tekton Fits Into a Kubernetes Cluster

Tekton is a CI/CD execution framework built on top of Kubernetes primitives. It does not run as a hypervisor, sandbox, or security boundary. It translates higher-level CI/CD abstractions (`TaskRun`, `PipelineRun`) into native Kubernetes `Pods` and relies entirely on the host cluster's control plane for enforcement.

This is not an implementation shortcut. It is the design. Tekton intentionally delegates enforcement to the Kubernetes layer so that operators can apply their own admission policies, RBAC configurations, and network controls without Tekton getting in the way.

**Tekton explicitly trusts:**

- The Kubernetes API server
- Configured admission controllers (Pod Security Admission, OPA/Gatekeeper, Kyverno, or any other admission webhook)
- The cluster's RBAC model for access decisions

**Tekton does not re-implement:**

- Privileged container restrictions
- `hostPath` or `hostNetwork` access controls
- Namespace isolation or multi-tenancy boundaries
- Network egress filtering

If your cluster's admission configuration permits a given Pod spec, Tekton will submit that spec. Tekton is the messenger; the admission controller is the gatekeeper.

---

## Trust Boundaries

### TaskRun/PipelineRun Creation is Equivalent to Pod Creation

A user with RBAC permission to create `TaskRuns` or `PipelineRuns` has, by design, equivalent capability to a user who can create native Kubernetes `Pods`. Tekton translates these resources into Pods. This equivalence is intentional and documented.

Cluster administrators who want to restrict what kinds of Pods can be created must use the appropriate Kubernetes mechanisms: Pod Security Admission policies, `LimitRange`, resource quotas, and custom admission webhooks. Tekton does not impose its own layer on top of these controls, and it would be incorrect for it to do so.

### The `aggregate-to-edit` ClusterRole

Tekton ships ClusterRole definitions that aggregate into the Kubernetes `edit` role. This means users with `edit` permissions on a namespace gain the ability to create and manage Tekton resources.

This is consistent with standard Kubernetes design. Users with `edit` access can already create `Pods`, `Deployments`, `Jobs`, and other workload resources. Aggregating Tekton's custom resources into the `edit` role does not expand a user's effective privileges beyond what they already have in the cluster. If your security model requires that `edit` users cannot create workloads, you should restrict workload creation using admission controllers (such as Pod Security Admission, OPA Gatekeeper, or Kyverno), not by filing a Tekton vulnerability report.

---

## Out of Scope: What We Will Not Fix as Security Vulnerabilities

### Pod-Level Privilege Escalation via `podTemplate`

Tekton's `TaskRun` and `PipelineRun` resources expose a `podTemplate` field that allows callers to configure Pod-level settings, including security contexts, volume mounts, and node selectors.

A user with permission to create a `TaskRun` can use `podTemplate` to request a privileged container, a `hostPath` mount, or other elevated configurations. This is not a Tekton vulnerability. By design, granting a user permission to create a `TaskRun` intentionally delegates the ability to configure Pod-level settings to that user through the Tekton controller. This is an expected trust delegation, not a privilege escalation vulnerability.

The correct enforcement layer is the cluster's admission control policy. If your organization wants to prevent privileged containers, deploy and enforce Pod Security Admission at the `restricted` or `baseline` profile. If a report claims that `podTemplate` "bypasses" Tekton security controls, that claim assumes Tekton is responsible for controls it explicitly delegates to the cluster operator. We will decline such reports and explain why.

### Resolver SSRF to User-Controlled URLs

Tekton Resolvers (Git Resolver, HTTP Resolver, Bundle Resolver, etc.) are designed to fetch Pipeline and Task definitions from remote locations specified by the user. Fetching a URL that a user provides is the entire purpose of these components.

Reports claiming that a resolver can be made to fetch a user-controlled URL, and framing this as a Server-Side Request Forgery (SSRF) vulnerability, describe the resolver doing exactly what it was built to do. We will not accept these as security issues.

If your threat model requires restricting which external endpoints Tekton controllers can reach, the correct solution is to apply Kubernetes `NetworkPolicy` rules or enforce egress filtering at the cluster or cloud-provider level. Tekton does not maintain its own egress allowlist, and adding one would conflict with the open-ended flexibility the resolver architecture is built around.

### Documentation Discrepancies

Documentation errors, inaccurate examples, or content that does not match implementation behavior are bugs. They should be filed as regular GitHub issues. They are not security vulnerabilities unless a specific piece of documentation made an explicit, unambiguous security guarantee that the implementation demonstrably fails to uphold. If you believe that bar is met, explain exactly which documentation made the guarantee and exactly how the implementation violates it.

---

## In Scope: What We Want Reported

The following categories represent genuine Tekton security concerns. If you have found something in one of these areas, please report it through the process described in [SECURITY.md](https://github.com/tektoncd/.github/blob/main/SECURITY.md).

### Bypass of Tekton's Internal Validation

Tekton enforces certain constraints at the controller level: path validation for workspace mounts, checks against the `/tekton/` directory prefix, and resource name validation. A bypass that allows an attacker to violate these constraints through crafted input is a valid vulnerability.

### Controller Denial of Service via Malformed Resources

If a crafted `TaskRun`, `PipelineRun`, or other Tekton custom resource causes a controller to panic, enter an unrecoverable state, consume unbounded memory, or cease processing legitimate work, that is a DoS vulnerability. This includes unsafe streaming patterns such as unbounded reads from remote endpoints, where a malicious remote endpoint or user-controlled payload could cause the controller to exhaust memory.

### Secret or Credential Leakage from Controllers

If Tekton's controllers or resolvers expose secrets, tokens, or credentials in ways beyond what the task author explicitly configured (for example, leaking environment variable values into controller logs, or including credentials in error messages returned to unprivileged users), that is a vulnerability.

### Trust Verification Bypass in TrustedResources / VerificationPolicy

Tekton's `TrustedResources` feature and `VerificationPolicy` resources implement supply chain integrity checks via image and bundle signature verification. A vulnerability that allows an attacker to substitute an unverified or maliciously modified resource while convincing Tekton that verification passed is a serious supply chain security issue and should be reported immediately. (Note: These are currently considered alpha features).

### Unbounded Resource Consumption Without Operator Remediation

If a Tekton control plane component consumes unbounded CPU, memory, or network resources in response to user-provided input, and there is no operator-accessible configuration knob to cap or throttle that consumption, this represents a denial-of-service risk that belongs in Tekton's scope. Contrast this with standard workload resource consumption, which operators control via `LimitRange` and resource quotas.

---

## Vulnerability Disclosure Process and Credit

### Reporting

Submit vulnerability reports through GitHub's private security advisory feature on the [Tekton Pipelines repository](https://github.com/tektoncd/pipeline/security/advisories/new), or by emailing the maintainer list listed in [SECURITY.md](https://github.com/tektoncd/.github/blob/main/SECURITY.md). Do not open a public GitHub issue for potential vulnerabilities.

Include as much of the following as you have available: a description of the finding, the affected component and version, a reproduction case or proof-of-concept, and your assessment of impact and exploitability.

### Triage Timeline

You will receive an initial response acknowledging your report within 7 to 14 business days. That response will include a preliminary assessment of whether the finding falls within Tekton's threat model boundaries.

If additional information is needed from you, we will ask for it during this window. We aim to keep reporters informed throughout the process rather than leaving reports in silence.

### If Your Report is Declined

If we determine that a report falls outside Tekton's scope, we will tell you why. The response will reference the specific threat model boundary that applies, explain the reasoning, and identify the correct remediation layer (if one exists). We will not respond with a generic "this is by design" and nothing more.

If you believe our decline is incorrect, you are welcome to reply and make the case for why the specific threat model boundary does not apply to your finding. We will re-evaluate.

### Credit for Valid Vulnerabilities

Confirmed vulnerabilities will be credited to the reporter in the corresponding GitHub security advisory and in the release notes for the patch that addresses them. If you prefer not to be credited, say so in your report and we will respect that.

We do not operate a paid bug bounty program. Credit is the form of recognition we can offer.