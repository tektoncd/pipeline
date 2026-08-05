# Tekton Pipelines repo map for agents

Use this as a map after you read `AGENTS.md`. It points you toward the right code and docs; it is not another rule file.

The source-of-truth docs are still `README.md`, `CONTRIBUTING.md`, `DEVELOPMENT.md`, `api_compatibility_policy.md`, and `test/README.md`.

## What this repo does

Tekton Pipelines defines Kubernetes-native CI/CD APIs and the controllers that run them. The main resources are `Task`, `TaskRun`, `Pipeline`, `PipelineRun`, `CustomRun`, and resolver objects. The main binaries are the controller, webhook, entrypoint, events controller, remote resolvers, and small helper images.

## Runtime path

1. A user creates Tekton resources through the Kubernetes API.
2. Webhooks default, validate, and convert those resources.
3. Reconcilers watch resources through generated informers.
4. TaskRun reconciliation builds Pods and entrypoint config.
5. PipelineRun reconciliation resolves the graph and creates child runs.
6. Resolver controllers fetch remote task and pipeline definitions.
7. Event and notification controllers publish run state changes.

## Where to look

| Path | Use it for |
| --- | --- |
| `cmd/controller` | Controller binary setup and reconciler wiring. |
| `cmd/webhook` | Defaulting, validation, and conversion webhooks. |
| `cmd/entrypoint` | The process wrapper injected into step containers. |
| `pkg/apis/pipeline` | API types, defaults, validation, and conversions. |
| `pkg/client` | Generated clients, listers, informers, and reconcilers. |
| `pkg/reconciler/taskrun` | TaskRun reconciliation, Pod creation, and TaskRun status. |
| `pkg/reconciler/pipelinerun` | PipelineRun graph resolution and child run orchestration. |
| `pkg/reconciler/resolution` | Remote resolution request handling. |
| `pkg/reconciler/events`, `pkg/reconciler/notifications` | CloudEvents and notification controllers. |
| `pkg/pod` | Pod and entrypoint construction helpers. |
| `internal` | Private helpers. Prefer this for new non-API packages. |
| `config` | Install manifests and CRDs. |
| `test` | E2E, conformance, and test utilities. |
| `hack` | Codegen, verification, release, and local-cluster scripts. |
| `skills/tekton-pipeline-discovery` | Repo-specific discovery skill for agents. |

## Common change paths

- API or CRD changes: read `api_compatibility_policy.md`; run the generated-code and OpenAPI updates.
- Webhook changes: add or update validation/defaulting/conversion tests under `pkg/apis/...`.
- Reconciler changes: start with fake-client unit tests; add e2e coverage when behavior changes across controllers or Pods.
- Generated files: run the matching `hack/update-*` script instead of editing generated output by hand.
- E2E tests: keep `@test:execution=parallel` or `serial` annotations correct.

## Useful targeted checks

```sh
./hack/verify-agent-readiness.sh
go test ./pkg/apis/pipeline/v1 ./pkg/apis/pipeline/v1beta1
go test ./pkg/reconciler/taskrun ./pkg/reconciler/pipelinerun
./hack/verify-codegen.sh
```

Start narrow. Broaden the test run only when the touched area needs it.
