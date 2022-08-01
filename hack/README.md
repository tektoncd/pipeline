# Assorted scripts for development

This directory contains several scripts useful in the development process of
Tekton Pipelines.

- [`update-codegen.sh`](./update-codegen.sh): Updates auto-generated client
  libraries.
- [`update-deps.sh`](./update-deps.sh): Updates Go dependencies.
- [`update-openapigen.sh`](./update-openapigen.sh): Updates OpenAPI specification and Swagger file.
- [`verify-codegen.sh`](./verify-codegen.sh): Verifies that auto-generated
  client libraries are up-to-date.
- [`update-reference-docs.sh`](./update-reference-docs.sh) and related files: Generates [`docs/pipeline-api.md`](../docs/pipeline-api.md).
  - [`reference-docs-gen-config.json`](./reference-docs-gen-config.json) is the configuration file for the [`gen-api-reference-docs`](https://github.com/tektoncd/ahmetb-gen-crd-api-reference-docs) binary.
  - [`reference-docs-template`](./reference-docs-template) contains Go templates for the generated Markdown.
- Release docs have been moved to [the top-level `tekton` dir](../tekton)
