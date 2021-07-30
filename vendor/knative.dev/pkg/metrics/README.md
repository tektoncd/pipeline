# Common metrics export interfaces for Knative

See [the Plan](#the-plan) for details on where this is heading.

## Current status

The code currently uses OpenCensus to support exporting metrics to multiple
backends. Currently, two backends are supported: Prometheus and OpenCensus/OTel.

Metrics export is controlled by a ConfigMap called `config-observability` which
is a key-value map with specific values supported for each of the OpenCensus
and Prometheus backends. Hot-reload of the ConfigMap on a running process is
supported by directly watching (via the Kubernetes API) the
`config-observability` object. Configuration via environment is also supported
for use by the `queue-proxy`, which runs with user permissions in the user's
namespace.

## Problems

There are currently
[6 supported Golang exporters for OpenCensus](https://opencensus.io/exporters/supported-exporters/go/).
We do not want to build all of those backends into the core of `knative.dev/pkg` and all
downstream dependents, and we'd like all the code shipped in `knative.dev/pkg`
to be able to be tested without needing any special environment setup.

With the current direct-integration setup, there needs to be initial and ongoing
work in `pkg` (which should be high-value, low-churn code) to maintain and
update stats exporters which need to be statically linked into ~all Knative
binaries. This setup also causes problems for vendors who may want or need to
perform an out-of-tree integration (e.g. proprietary or partially-proprietary
monitoring stacks).

Another problem is that each vendor's exporter requires different parameters,
supplied as Golang `Options` methods which may require complex connections with
the Knative ConfigMap. Two examples of this are secrets like API keys and the
Prometheus monitoring port (which requires additional service/etc wiring).

See also
[this doc](https://docs.google.com/document/d/1t-aov3XrhobjCKW4kwScY44QAoahiwxoyXXFtZyL8jw/edit),
where the plan was worked out.

## The plan

OpenCensus (and eventually OpenTelemetry) offers an sidecar or host-level agent
with speaks the OpenCensus protocol and can proxy from this protocol to multiple
backends.

![OpenCensus Agent configuration](https://github.com/census-instrumentation/opencensus-service/raw/master/images/opencensus-service-deployment-models.png)
(From OpenCensus Documentation)

**We will standardize on export to the OpenCensus export protocol, and encourage
vendors to implement their own OpenCensus Agent or Collector DaemonSet, Sidecar,
or other
[OpenCensus Protocol](https://github.com/census-instrumentation/opencensus-proto/tree/master/src/opencensus/proto/agent)
service which connects to their desired monitoring environment.** For now, we
will use the `config-observability` ConfigMap to provide the OpenCensus
endpoint, but we will work with the OpenTelemetry group to define a
kubernetes-friendly standard export path.

**Additionally, once OpenTelemetry agent is stable, we will propose adding the
OpenTelemetry agent running on a localhost port as part of the runtime
contract.**

We need to make sure that the OpenCensus library does not block, fail, or queue
metrics in-process excessively in the case where the OpenCensus Agent is not
present on the cluster. This will allow us to ship Knative components which
attempt to reach out the Agent if present, and which simply retain local
statistics for a short period of time if not.

### Concerns

- Unsure about the stability of the OpenCensus Agent (or successor). We're
  currently investigating this, but the OpenCensus agent seems to have been
  recommended by several others.
- Running `fluentd` as a sidecar was very big (400MB) and had a large impact on
  cold start times.
  - Mitigation: run the OpenCensus agent as a DaemonSet (like we do with
    `fluentd` now).
- Running as a DaemonSet may make it more difficult to ensure that metrics for
  each namespace end up in the right place.
  - We have this problem with the built-in configurations today, so this doesn't
    make the problem substantially worse.
  - May want/need some connection between the Agent and the Kubelet to verify
    sender identities eventually.
  - Only expose OpenCensus Agent on localhost, not outside the node.

### Steps to reach the goal

- [x] [Add OpenCensus Agent as one of the export options](https://github.com/knative/pkg/issues/955).
- [x] Ensure that all tests pass in a non-Google-Cloud connected environment.
      **This is true today.**
      [Ensure this on an ongoing basis.](https://github.com/knative/pkg/issues/957)
- [x] Google to implement OpenCensus Agent configuration to match what they are
      doing for Stackdriver now. (No public issue link because this should be in
      Google's vendor-specific configuration.)
- [x] Document how to configure OpenCensus/OpenTelemetry Agent + Prometheus to
      achieve the current level of application visibility, and determine a
      long-term course for how to maintain this as a "bare minimum" supported
      configuration. https://github.com/knative/docs/pull/3005
- [x] Stop adding exporter features outside of the OpenCensus / OpenTelemetry
      export as of 0.13 release (03 March 2020). Between now and 0.13, small
      amounts of additional features can be built in to assist with the bridging
      process or to support existing products. New products should build on the
      OpenCensus Agent approach.
- [x] Removal of the Stackdriver OpenCensus Exporter
      https://github.com/knative/pkg/issues/2173
- [ ] Revisit Adopting OpenTelemetry SDKs instead of OpenCensus
