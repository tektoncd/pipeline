# Agent notes for Tekton Pipelines

If you're an agent working in this repo, start here. This file is meant to orient you quickly; it does not replace the project docs or maintainer judgment.

## Ground rules

- These notes apply to the whole repo.
- Do not add more `AGENTS.md` files unless a maintainer asks.
- Keep changes narrow and easy to review.
- Prefer the smallest test that proves the change.
- Do not hand-edit generated files or `vendor/`; use the matching update script or dependency workflow.
- When in doubt, follow nearby code and ask before inventing a new pattern.

## Repo shape

- `cmd/`: binaries such as controller, webhook, entrypoint, resolver, and events.
- `pkg/apis/`: API types, defaults, validation, conversion, and generated clients.
- `pkg/reconciler/`: TaskRun, PipelineRun, CustomRun, resolver, event, and notification reconcilers.
- `pkg/pod`, `pkg/workspace`, `pkg/substitution`: helpers used by the reconcilers.
- `internal/`: private implementation packages; prefer this for new non-API helpers.
- `config/`: install manifests and CRDs. Many files here are generated.
- `test/`: e2e tests, conformance tests, helpers, and presubmit scripts.
- `docs/`: user-facing and developer documentation.

## Before changing code

- Read `CONTRIBUTING.md`, `DEVELOPMENT.md`, `test/README.md`, and the docs for the area you are touching.
- For API changes, read `api_compatibility_policy.md` first and expect codegen/OpenAPI updates.
- For reconciler changes, add or update package tests with fake clients before reaching for e2e tests.
- For e2e changes, follow the annotations and categories in `test/README.md`.

## Useful checks

- Format Go: `gofmt -w <files>`.
- Test one package: `go test ./path/to/package`.
- Test one case: `go test ./path/to/package -run TestName`.
- Run common packages: `go test ./pkg/...`.
- Run generated-code check: `./hack/verify-codegen.sh`.
- Update generated code when needed: `./hack/update-codegen.sh`.
- Run the local agent-context check: `./hack/verify-agent-readiness.sh`.

## More context

- Repo map for agents: `docs/agents/agent-context.md`.
- Discovery skill: `skills/tekton-pipeline-discovery/SKILL.md`.
- Skill discovery symlinks live under `.agents/`, `.claude/`, `.codex/`, and `.cursor/`.
