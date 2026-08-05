---
name: tekton-pipeline-discovery
description: Use this when you need to find your way around tektoncd/pipeline before editing, reviewing, or debugging Tekton Pipelines code.
---

# Tekton Pipeline Discovery

Use this skill for the first pass through the repo. It should help you find the right files quickly without turning into a second set of project rules.

## Start here

1. Read `AGENTS.md`.
2. Use `docs/agents/agent-context.md` as the repo map.
3. Then read the docs for the area you are touching. The common ones are `CONTRIBUTING.md`, `DEVELOPMENT.md`, `api_compatibility_policy.md`, `test/README.md`, and files under `docs/developers/`.

## Working notes

- Keep the change small enough that a maintainer can review it without guessing your intent.
- Test the package you touched before running broad suites.
- Prefer new private helpers under `internal/` unless the code is intentionally public API.
- Do not hand-edit generated code or `vendor/`.
- For CRD/API behavior, check compatibility before changing fields, defaults, status, labels, or generated clients.
- For e2e changes, follow the categories and annotations in `test/README.md`.

## Common commands

```sh
go test ./pkg/...                         # broad package tests
./hack/verify-codegen.sh                  # generated-code consistency
./hack/verify-agent-readiness.sh          # local AGENTS.md and skill discovery check
```

## PR notes

- Follow `.github/pull_request_template.md`.
- Keep the `release-note` block accurate; use `NONE` only when there is no user-facing impact.
- Use `Fixes #...` only when the PR fully resolves the issue.
- For agent-context-only docs changes, `./hack/verify-agent-readiness.sh` is usually the right targeted check.
