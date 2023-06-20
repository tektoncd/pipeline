# Tekton Pipeline Releases

## Release Frequency

Tekton Pipelines follows the Tekton community [release policy][release-policy]
as follows:

- Versions are numbered according to semantic versioning: `vX.Y.Z`
- A new release is produced on a monthly basis
- Four releases a year are chosen for [long term support (LTS)](https://github.com/tektoncd/community/blob/main/releases.md#support-policy).
  All remaining releases are supported for approximately 1 month (until the next
  release is produced)
    - LTS releases take place in January, April, July and October every year
    - The first Tekton Pipelines LTS release will be **v0.41.0** in October 2022
    - Releases happen towards the middle of the month, between the 13th and the
      20th, depending on week-ends and readiness

Tekton Pipelines produces nightly builds, publicly available on
`gcr.io/tekton-nightly`. 

### Transition Process

Before release v0.41 Tekton Pipelines has worked on the basis of an undocumented
support period of four months, which will be maintained for the releases between
v0.37 and v0.40.

## Release Process

Tekton Pipeline releases are made of YAML manifests and container images.
Manifests are published to cloud object-storage as well as
[GitHub][tekton-pipeline-releases]. Container images are signed by
[Sigstore][sigstore] via [Tekton Chains][tekton-chains]; signatures can be
verified through the [public key][chains-public-key] hosted by the Tekton Chains
project.

Further documentation available:

- The Tekton Pipeline [release process][tekton-releases-docs]
- [Installing Tekton][tekton-installation]
- Standard for [release notes][release-notes-standards]

## Release

### v0.49

- **Latest Release**: [v0.49.0][v0-49-0] (2023-06-20) ([docs][v0-49-0-docs], [examples][v0-49-0-examples])
- **Initial Release**: [v0.49.0][v0-49-0] (2023-06-20)
- **Estimated End of Life**: 2023-07-20
- **Patch Releases**: [v0.49.0][v0-49-0]

### v0.47 (LTS)

- **Latest Release**: [v0.47.2][v0-47-2] (2023-06-13) ([docs][v0-47-2-docs], [examples][v0-47-2-examples])
- **Initial Release**: [v0.47.0][v0-47-0] (2023-03-17)
- **Estimated End of Life**: 2023-04-17
- **Patch Releases**: [v0.47.0][v0-47-0], [v0.47.1][v0-47-1], [v0.47.2][v0-47-2] 

### v0.44 (LTS)

- **Latest Release**: [v0.44.3][v0-44-3] (2023-01-24) ([docs][v0-44-3-docs], [examples][v0-44-3-examples])
- **Initial Release**: [v0.44.0][v0-44-0] (2023-01-24)
- **Estimated End of Life**: 2024-01-24
- **Patch Releases**: [v0.44.0][v0-44-0], [v0.44.2][v0-44-2], [v0.44.3][v0-44-3]
 
### v0.41 (LTS)

- **Latest Release**: [0.41.3][v0-41-3] (2023-04-06) ([docs][v0-41-3-docs], [examples][v0-41-3-examples])
- **Initial Release**: [v0.41.0][v0-41-0] (2022-10-31)
- **Estimated End of Life**: 2023-10-30
- **Patch Releases**: [v0.41.0][v0-41-0], [v0.41.1][v0-41-1], [v0.41.2][v0-41-2], [v0.41.3][v0-41-3]

## End of Life Releases

### v0.48

- **Latest Release**: [v0.48.0][v0-48-0] (2023-05-25) ([docs][v0-48-0-docs], [examples][v0-48-0-examples])
- **Initial Release**: [v0.48.0][v0-48-0] (2023-05-25)
- **End of Life**: 2023-06-20
- **Patch Releases**: [v0.48.0][v0-48-0]

### v0.46

- **Latest Release**: [v0.46.0][v0-46-0] (2023-03-17) ([docs][v0-46-0-docs], [examples][v0-46-0-examples])
- **Initial Release**: [v0.46.0][v0-46-0] (2023-03-17)
- **End of Life**: 2023-04-17
- **Patch Releases**: [v0.46.0][v0-46-0]

### v0.45

- **Latest Release**: [v0.45.0][v0-45-0] (2023-02-17) ([docs][v0-45-0-docs], [examples][v0-45-0-examples])
- **Initial Release**: [v0.45.0][v0-45-0] (2023-02-17)
- **End of Life**: 2023-03-17
- **Patch Releases**: [v0.45.0][v0-45-0]

### v0.43

- **Latest Release**: [v0.43.2][v0-43-2] (2023-01-10) ([docs][v0-43-2-docs], [examples][v0-43-2-examples])
- **Initial Release**: [v0.43.0][v0-43-0] (2022-12-22)
- **End of Life**: 2023-04-22
- **Patch Releases**: [v0.43.0][v0-43-0], [v0.43.1][v0-43-1], [v0.43.2][v0-43-2]

### v0.42

- **Latest Release**: [v0.42.0][v0-42-0] (2022-11-22) ([docs][v0-42-0-docs], [examples][v0-42-0-examples])
- **Initial Release**: [v0.42.0][v0-42-0] (2022-11-22)
- **End of Life**: 2023-03-22
- **Patch Releases**: [v0.42.0][v0-42-0]

### v0.40

- **Latest Release**: [v0.40.2][v0-40-2] (2022-10-03) ([docs][v0-40-2-docs], [examples][v0-40-2-examples])
- **Initial Release**: [v0.40.0][v0-40-0] (2022-09-16)
- **End of Life**: 2023-01-15
- **Patch Releases**: [v0.40.0][v0-40-0], [v0.40.1][v0-40-1], [v0.40.2][v0-40-2]

### v0.39

- **Latest Release**: [v0.39.0][v0-39-0] (2022-08-18) ([docs][v0-39-0-docs], [examples][v0-39-0-examples])
- **Initial Release**: [v0.39.0][v0-39-0] (2022-08-18)
- **End of Life**: 2022-12-17
- **Patch Releases**: [v0.39.0][v0-39-0]

### v0.38

- **Latest Release**: [v0.38.4][v0-38-4] (2022-09-14) ([docs][v0-38-4-docs], [examples][v0-38-4-examples])
- **Initial Release**: [v0.38.0][v0-38-0] (2022-07-25)
- **End of Life**: 2022-11-24
- **Patch Releases**: [v0.38.0][v0-38-0], [v0.38.1][v0-38-1], [v0.38.2][v0-38-2], [v0.38.3][v0-38-3], [v0.38.4][v0-38-4]

### v0.37

- **Latest Release**: [v0.37.5][v0-37-5] (2022-09-25) ([docs][v0-37-5-docs], [examples][v0-37-5-examples])
- **Initial Release**: [v0.37.0][v0-37-0] (2022-06-21)
- **End of Life**: 2022-10-20

Older releases are EOL and available on [GitHub][tekton-pipeline-releases].

[release-policy]: https://github.com/tektoncd/community/blob/main/releases.md
[sigstore]: https://sigstore.dev
[tekton-chains]: https://github.com/tektoncd/chains
[tekton-pipeline-releases]: https://github.com/tektoncd/pipeline/releases
[chains-public-key]: https://github.com/tektoncd/chains/blob/main/tekton.pub
[tekton-releases-docs]: tekton
[tekton-installation]: docs/install.md
[release-notes-standards]:
    https://github.com/tektoncd/community/blob/main/standards.md#release-notes

[v0-49-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.49.0
[v0-48-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.48.0
[v0-47-2]: https://github.com/tektoncd/pipeline/releases/tag/v0.47.2
[v0-47-1]: https://github.com/tektoncd/pipeline/releases/tag/v0.47.1
[v0-47-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.47.0
[v0-46-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.46.0
[v0-45-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.45.0
[v0-44-3]: https://github.com/tektoncd/pipeline/releases/tag/v0.44.3
[v0-44-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.44.0
[v0-43-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.43.0
[v0-43-1]: https://github.com/tektoncd/pipeline/releases/tag/v0.43.1
[v0-43-2]: https://github.com/tektoncd/pipeline/releases/tag/v0.43.2
[v0-42-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.42.0
[v0-41-3]: https://github.com/tektoncd/pipeline/releases/tag/v0.41.3
[v0-41-2]: https://github.com/tektoncd/pipeline/releases/tag/v0.41.2
[v0-41-1]: https://github.com/tektoncd/pipeline/releases/tag/v0.41.1
[v0-41-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.41.0
[v0-40-2]: https://github.com/tektoncd/pipeline/releases/tag/v0.40.2
[v0-40-1]: https://github.com/tektoncd/pipeline/releases/tag/v0.40.1
[v0-40-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.40.0
[v0-39-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.39.0
[v0-38-4]: https://github.com/tektoncd/pipeline/releases/tag/v0.38.4
[v0-38-3]: https://github.com/tektoncd/pipeline/releases/tag/v0.38.3
[v0-38-2]: https://github.com/tektoncd/pipeline/releases/tag/v0.38.2
[v0-38-1]: https://github.com/tektoncd/pipeline/releases/tag/v0.38.1
[v0-38-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.38.0
[v0-37-5]: https://github.com/tektoncd/pipeline/releases/tag/v0.37.5
[v0-37-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.37.0

[v0-49-0-docs]: https://github.com/tektoncd/pipeline/tree/v0.49.0/docs#tekton-pipelines
[v0-48-0-docs]: https://github.com/tektoncd/pipeline/tree/v0.48.0/docs#tekton-pipelines
[v0-47-2-docs]: https://github.com/tektoncd/pipeline/tree/v0.47.2/docs#tekton-pipelines
[v0-46-0-docs]: https://github.com/tektoncd/pipeline/tree/v0.46.0/docs#tekton-pipelines
[v0-45-0-docs]: https://github.com/tektoncd/pipeline/tree/v0.45.0/docs#tekton-pipelines
[v0-44-3-docs]: https://github.com/tektoncd/pipeline/tree/v0.44.3/docs#tekton-pipelines
[v0-43-0-docs]: https://github.com/tektoncd/pipeline/tree/v0.43.0/docs#tekton-pipelines
[v0-43-1-docs]: https://github.com/tektoncd/pipeline/tree/v0.43.1/docs#tekton-pipelines
[v0-43-2-docs]: https://github.com/tektoncd/pipeline/tree/v0.43.2/docs#tekton-pipelines
[v0-42-0-docs]: https://github.com/tektoncd/pipeline/tree/v0.42.0/docs#tekton-pipelines
[v0-41-3-docs]: https://github.com/tektoncd/pipeline/tree/v0.41.3/docs#tekton-pipelines
[v0-40-2-docs]: https://github.com/tektoncd/pipeline/tree/v0.40.2/docs#tekton-pipelines
[v0-39-0-docs]: https://github.com/tektoncd/pipeline/tree/v0.39.0/docs#tekton-pipelines
[v0-38-4-docs]: https://github.com/tektoncd/pipeline/tree/v0.38.4/docs#tekton-pipelines
[v0-37-5-docs]: https://github.com/tektoncd/pipeline/tree/v0.37.5/docs#tekton-pipelines

[v0-49-0-examples]: https://github.com/tektoncd/pipeline/tree/v0.49.0/examples#examples
[v0-48-0-examples]: https://github.com/tektoncd/pipeline/tree/v0.48.0/examples#examples
[v0-47-2-examples]: https://github.com/tektoncd/pipeline/tree/v0.47.2/examples#examples
[v0-46-0-examples]: https://github.com/tektoncd/pipeline/tree/v0.46.0/examples#examples
[v0-45-0-examples]: https://github.com/tektoncd/pipeline/tree/v0.45.0/examples#examples
[v0-44-3-examples]: https://github.com/tektoncd/pipeline/tree/v0.44.3/examples#examples
[v0-43-0-examples]: https://github.com/tektoncd/pipeline/tree/v0.43.0/examples#examples
[v0-43-1-examples]: https://github.com/tektoncd/pipeline/tree/v0.43.1/examples#examples
[v0-43-2-examples]: https://github.com/tektoncd/pipeline/tree/v0.43.2/examples#examples
[v0-42-0-examples]: https://github.com/tektoncd/pipeline/tree/v0.42.0/examples#examples
[v0-41-3-examples]: https://github.com/tektoncd/pipeline/tree/v0.41.3/examples#examples
[v0-40-2-examples]: https://github.com/tektoncd/pipeline/tree/v0.40.2/examples#examples
[v0-39-0-examples]: https://github.com/tektoncd/pipeline/tree/v0.39.0/examples#examples
[v0-38-4-examples]: https://github.com/tektoncd/pipeline/tree/v0.38.4/examples#examples
[v0-37-5-examples]: https://github.com/tektoncd/pipeline/tree/v0.37.5/examples#examples
