<!--
---
linkTitle: "Step Composition Contract"
weight: 8
---
-->

# Tekton Step Composition v0.1

This document outlines the step composition model.

## Aims

* Allow Tasks and Steps to be used from versioned files in git (or other sources like OCI).
* Add local steps before/after/in between the steps from a shared Task
* Override properties on an inherited step such as changing the image, command, args, environment variables, volumes, script.
* Try be fairly DRY yet simple so that its immediately obvious looking at a Tekton YAML what it means. e.g. default to inheriting tasks from github so that the [tekton catalog](https://github.com/tektoncd/catalog) tasks can be used in a nice concise way

