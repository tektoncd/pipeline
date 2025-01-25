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

### v0.66
- **Latest Release**: [v0.66.0][v0.66-0] (2024-12-04) ([docs][v0.66-0-docs], [examples][v0.66-0-examples])
- **Initial Release**: [v0.66.0][v0.66-0] (2024-12-04)
- **Estimated End of Life**: 2024-12-28
- **Patch Releases**: [v0.66.0][v0.66-0]

### v0.65 (LTS)
- **Latest Release**: [v0.65.0][v0.65-0] (2024-10-28) ([docs][v0.65-0-docs], [examples][v0.65-0-examples])
- **Initial Release**: [v0.65.0][v0.65-0] (2024-10-28)
- **End of Life**: 2025-10-28
- **Patch Releases**: [v0.65.0][v0.65-0]

### v0.62 (LTS)
- **Latest Release**: [v0.62.2][v0.62-2] (2024-08-23) ([docs][v0.62-2-docs], [examples][v0.62-2-examples])
- **Initial Release**: [v0.62.0][v0.62-0] (2024-07-23)
- **End of Life**: 2025-07-23
- **Patch Releases**: [v0.62.0][v0.62-0], [v0.62.1][v0.62-1], [v0.62.2][v0.62-2]

### v0.59 (LTS)

- **Latest Release**: [v0.59.2][v0.59-2] (2024-07-04) ([docs][v0.59-2-docs], [examples][v0.59-2-examples])
- **Initial Release**: [v0.59.0][v0.59-0] (2024-04-25)
- **Estimated End of Life**: 2025-04-24
- **Patch Releases**: [v0.59.0][v0.59-0], [v0.59.1][v0.59-1], [v0.59.2][v0.59-2]

### v0.56 (LTS)

- **Latest Release**: [v0.56.3][v0.56-3] (2024-03-26) ([docs][v0.56-3-docs], [examples][v0.56-3-examples])
- **Initial Release**: [v0.56.0][v0.56-0] (2024-01-19)
- **Estimated End of Life**: 2025-01-19
- **Patch Releases**: [v0.56.0][v0.56-0], [v0.56.1][v0.56-1], [v0.56.2][v0.56-2], [v0.56.3][v0.56-3]

## End of Life Releases

### v0.64
- **Latest Release**: [v0.64.0][v0.64-0] (2024-08-30) ([docs][v0.64-0-docs], [examples][v0.64-0-examples])
- **Initial Release**: [v0.64.0][v0.64-0] (2024-08-30)
- **End of Life**: 2024-09-27
- **Patch Releases**: [v0.64.0][v0.64-0]

### v0.63
- **Latest Release**: [v0.63.0][v0.63-0] (2024-08-30) ([docs][v0.63-0-docs], [examples][v0.63-0-examples])
- **Initial Release**: [v0.63.0][v0.63-0] (2024-08-30)
- **End of Life**: 2024-09-27
- **Patch Releases**: [v0.63.0][v0.63-0]

### v0.61
- **Latest Release**: [v0.61.0][v0.61-0] (2024-06-25) ([docs][v0.61-0-docs], [examples][v0.61-0-examples])
- **Initial Release**: [v0.61.0][v0.61-0] (2024-06-25)
- **End of Life**: 2024-07-25
- **Patch Releases**: [v0.61.0][v0.61-0]

### v0.60
- **Latest Release**: [v0.60.1][v0.60-1] (2024-05-28) ([docs][v0.60-1-docs], [examples][v0.60-1-examples])
- **Initial Release**: [v0.60.0][v0.60-0] (2024-05-22)
- **End of Life**: 2024-06-22
- **Patch Releases**: [v0.60.0][v0.60-0], [v0.60.1][v0.60-1]

### v0.58

- **Latest Release**: [v0.58.0][v0.58-0] (2024-03-20) ([docs][v0.58-0-docs], [examples][v0.58-0-examples])
- **Initial Release**: [v0.58.0][v0.58-0] (2024-03-20)
- **End of Life**: 2024-04-20
- **Patch Releases**: [v0.58.0][v0.58-0]

### v0.57

- **Latest Release**: [v0.57.0][v0.57-0] (2024-02-20) ([docs][v0.57-0-docs], [examples][v0.57-0-examples])
- **Initial Release**: [v0.57.0][v0.57-0] (2024-02-20)
- **Estimated End of Life**: 2024-03-20
- **Patch Releases**: [v0.57.0][v0.57-0]

### v0.54

- **Latest Release**: [v0.54.0][v0.54-0] (2023-11-27) ([docs][v0.54-0-docs], [examples][v0.54-0-examples])
- **Initial Release**: [v0.54.0][v0.54-0] (2023-11-27)
- **Estimated End of Life**: 2023-12-27
- **Patch Releases**: [v0.54.0][v0.54-0]  

### v0.53 (LTS)

- **Latest Release**: [v0.53.5][v0.53-5] (2024-03-26) ([docs][v0.53-5-docs], [examples][v0.53-5-examples])
- **Initial Release**: [v0.53.0][v0.53-0] (2023-10-26)
- **End of Life**: 2024-11-01
- **Patch Releases**: [v0.53.0][v0.53-0], [v0.53.1][v0.53-1], [v0.53.2][v0.53-2], [v0.53.3][v0.53-3], [v0.53.4][v0.53-4], [v0.53.5][v0.53-5] 

### v0.52

- **Latest Release**: [v0.52.0][v0.52-0] (2023-09-20) ([docs][v0.52-0-docs], [examples][v0.52-0-examples])
- **Initial Release**: [v0.52.0][v0.52-0] (2023-09-20)
- **End of Life**: 2023-10-27
- **Patch Releases**: [v0.52.0][v0.52-0]

### v0.51

- **Latest Release**: [v0.51.0][v0.51-0] (2023-08-17) ([docs][v0.51-0-docs], [examples][v0.51-0-examples])
- **Initial Release**: [v0.51.0][v0.51-0] (2023-08-17)
- **End of Life**: 2023-09-17
- **Patch Releases**: [v0.51.0][v0.51-0]

### v0.50 (LTS)

- **Latest Release**: [v0.50.5][v0.50-5] (2023-11-16) ([docs][v0.50-5-docs], [examples][v0.50-5-examples])
- **Initial Release**: [v0.50.0][v0.50-0] (2023-07-25)
- **Estimated End of Life**: 2024-07-25
- **Patch Releases**: [v0.50.0][v0.50-0] [v0.50.1][v0.50-1] [v0.50.2][v0.50-2] [v0.50.3][v0.50-3] [v0.50.4][v0.50-4] [v0.50.5][v0.50-5]

### v0.49

- **Latest Release**: [v0.49.0][v0-49-0] (2023-06-20) ([docs][v0-49-0-docs], [examples][v0-49-0-examples])
- **Initial Release**: [v0.49.0][v0-49-0] (2023-06-20)
- **End of Life**: 2023-07-20
- **Patch Releases**: [v0.49.0][v0-49-0]

### v0.48

- **Latest Release**: [v0.48.0][v0-48-0] (2023-05-25) ([docs][v0-48-0-docs], [examples][v0-48-0-examples])
- **Initial Release**: [v0.48.0][v0-48-0] (2023-05-25)
- **End of Life**: 2023-06-20
- **Patch Releases**: [v0.48.0][v0-48-0]

### v0.47 (LTS)

- **Latest Release**: [v0.47.8][v0-47-8] (2024-04-05) ([docs][v0-47-8-docs], [examples][v0-47-8-examples])
- **Initial Release**: [v0.47.0][v0-47-0] (2023-03-17)
- **Estimated End of Life**: 2024-03-17
- **Patch Releases**: [v0.47.0][v0-47-0], [v0.47.1][v0-47-1], [v0.47.2][v0-47-2], [v0.47.3][v0-47-3], [v0.47.4][v0-47-4], [v0.47.5][v0-47-5], [v0.47.6][v0-47-6], [v0.47.7][v0-47-7], [v0.47.8][v0-47-8]

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

### v0.44 (LTS)

- **Latest Release**: [v0.44.4][v0-44-4] (2023-07-03) ([docs][v0-44-4-docs], [examples][v0-44-4-examples])
- **Initial Release**: [v0.44.0][v0-44-0] (2023-01-24)
- **End of Life**: 2024-01-25
- **Patch Releases**: [v0.44.0][v0-44-0], [v0.44.1][v0-44-1], [v0.44.2][v0-44-2], [v0.44.3][v0-44-3], [v0.44.4][v0-44-4]

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

### v0.41 (LTS)

- **Latest Release**: [0.41.3][v0-41-3] (2023-04-06) ([docs][v0-41-3-docs], [examples][v0-41-3-examples])
- **Initial Release**: [v0.41.0][v0-41-0] (2022-10-31)
- **End of Life**: 2023-10-27
- **Patch Releases**: [v0.41.0][v0-41-0], [v0.41.1][v0-41-1], [v0.41.2][v0-41-2], [v0.41.3][v0-41-3]

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

[v0.66-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.66.0
[v0.65-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.65.0
[v0.64-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.64.0
[v0.63-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.63.0
[v0.62-2]: https://github.com/tektoncd/pipeline/releases/tag/v0.62.2
[v0.62-1]: https://github.com/tektoncd/pipeline/releases/tag/v0.62.1
[v0.62-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.62.0
[v0.61-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.61.0
[v0.60-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.60.0
[v0.60-1]: https://github.com/tektoncd/pipeline/releases/tag/v0.60.1
[v0.59-2]: https://github.com/tektoncd/pipeline/releases/tag/v0.59.2
[v0.59-1]: https://github.com/tektoncd/pipeline/releases/tag/v0.59.1
[v0.59-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.59.0
[v0.58-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.58.0
[v0.57-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.57.0
[v0.56-3]: https://github.com/tektoncd/pipeline/releases/tag/v0.56.3
[v0.56-2]: https://github.com/tektoncd/pipeline/releases/tag/v0.56.2
[v0.56-1]: https://github.com/tektoncd/pipeline/releases/tag/v0.56.1
[v0.56-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.56.0
[v0.54-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.54.0
[v0.53-5]: https://github.com/tektoncd/pipeline/releases/tag/v0.53.5
[v0.53-4]: https://github.com/tektoncd/pipeline/releases/tag/v0.53.4
[v0.53-3]: https://github.com/tektoncd/pipeline/releases/tag/v0.53.3
[v0.53-2]: https://github.com/tektoncd/pipeline/releases/tag/v0.53.2
[v0.53-1]: https://github.com/tektoncd/pipeline/releases/tag/v0.53.1
[v0.53-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.53.0
[v0.52-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.52.0
[v0.51-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.51.0
[v0.50-5]: https://github.com/tektoncd/pipeline/releases/tag/v0.50.5
[v0.50-4]: https://github.com/tektoncd/pipeline/releases/tag/v0.50.4
[v0.50-3]: https://github.com/tektoncd/pipeline/releases/tag/v0.50.3
[v0.50-2]: https://github.com/tektoncd/pipeline/releases/tag/v0.50.2
[v0.50-1]: https://github.com/tektoncd/pipeline/releases/tag/v0.50.1
[v0.50-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.50.0
[v0-49-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.49.0
[v0-48-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.48.0
[v0-47-8]: https://github.com/tektoncd/pipeline/releases/tag/v0.47.8
[v0-47-7]: https://github.com/tektoncd/pipeline/releases/tag/v0.47.7
[v0-47-6]: https://github.com/tektoncd/pipeline/releases/tag/v0.47.6
[v0-47-5]: https://github.com/tektoncd/pipeline/releases/tag/v0.47.5
[v0-47-4]: https://github.com/tektoncd/pipeline/releases/tag/v0.47.4
[v0-47-3]: https://github.com/tektoncd/pipeline/releases/tag/v0.47.3
[v0-47-2]: https://github.com/tektoncd/pipeline/releases/tag/v0.47.2
[v0-47-1]: https://github.com/tektoncd/pipeline/releases/tag/v0.47.1
[v0-47-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.47.0
[v0-46-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.46.0
[v0-45-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.45.0
[v0-44-4]: https://github.com/tektoncd/pipeline/releases/tag/v0.44.4
[v0-44-3]: https://github.com/tektoncd/pipeline/releases/tag/v0.44.3
[v0-44-2]: https://github.com/tektoncd/pipeline/releases/tag/v0.44.2
[v0-44-1]: https://github.com/tektoncd/pipeline/releases/tag/v0.44.1
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

[v0.66-0-docs]: https://github.com/tektoncd/pipeline/tree/v0.66.0/docs#tekton-pipelines
[v0.65-0-docs]: https://github.com/tektoncd/pipeline/tree/v0.65.0/docs#tekton-pipelines
[v0.64-0-docs]: https://github.com/tektoncd/pipeline/tree/v0.64.0/docs#tekton-pipelines
[v0.63-0-docs]: https://github.com/tektoncd/pipeline/tree/v0.63.0/docs#tekton-pipelines
[v0.62-2-docs]: https://github.com/tektoncd/pipeline/tree/v0.62.2/docs#tekton-pipelines
[v0.62-1-docs]: https://github.com/tektoncd/pipeline/tree/v0.62.1/docs#tekton-pipelines
[v0.62-0-docs]: https://github.com/tektoncd/pipeline/tree/v0.62.0/docs#tekton-pipelines
[v0.61-0-docs]: https://github.com/tektoncd/pipeline/tree/v0.61.0/docs#tekton-pipelines
[v0.60-0-docs]: https://github.com/tektoncd/pipeline/tree/v0.60.0/docs#tekton-pipelines
[v0.60-1-docs]: https://github.com/tektoncd/pipeline/tree/v0.60.1/docs#tekton-pipelines
[v0.59-2-docs]: https://github.com/tektoncd/pipeline/tree/v0.59.2/docs#tekton-pipelines
[v0.59-1-docs]: https://github.com/tektoncd/pipeline/tree/v0.59.1/docs#tekton-pipelines
[v0.59-0-docs]: https://github.com/tektoncd/pipeline/tree/v0.59.0/docs#tekton-pipelines
[v0.58-0-docs]: https://github.com/tektoncd/pipeline/tree/v0.58.0/docs#tekton-pipelines
[v0.57-0-docs]: https://github.com/tektoncd/pipeline/tree/v0.57.0/docs#tekton-pipelines
[v0.56-3-docs]: https://github.com/tektoncd/pipeline/tree/v0.56.3/docs#tekton-pipelines
[v0.54-0-docs]: https://github.com/tektoncd/pipeline/tree/v0.54.0/docs#tekton-pipelines
[v0.53-5-docs]: https://github.com/tektoncd/pipeline/tree/v0.53.5/docs#tekton-pipelines
[v0.52-0-docs]: https://github.com/tektoncd/pipeline/tree/v0.52.0/docs#tekton-pipelines
[v0.51-0-docs]: https://github.com/tektoncd/pipeline/tree/v0.51.0/docs#tekton-pipelines
[v0.50-5-docs]: https://github.com/tektoncd/pipeline/tree/v0.50.5/docs#tekton-pipelines
[v0-49-0-docs]: https://github.com/tektoncd/pipeline/tree/v0.49.0/docs#tekton-pipelines
[v0-48-0-docs]: https://github.com/tektoncd/pipeline/tree/v0.48.0/docs#tekton-pipelines
[v0-47-8-docs]: https://github.com/tektoncd/pipeline/tree/v0.47.8/docs#tekton-pipelines
[v0-46-0-docs]: https://github.com/tektoncd/pipeline/tree/v0.46.0/docs#tekton-pipelines
[v0-45-0-docs]: https://github.com/tektoncd/pipeline/tree/v0.45.0/docs#tekton-pipelines
[v0-44-4-docs]: https://github.com/tektoncd/pipeline/tree/v0.44.4/docs#tekton-pipelines
[v0-43-2-docs]: https://github.com/tektoncd/pipeline/tree/v0.43.2/docs#tekton-pipelines
[v0-42-0-docs]: https://github.com/tektoncd/pipeline/tree/v0.42.0/docs#tekton-pipelines
[v0-41-3-docs]: https://github.com/tektoncd/pipeline/tree/v0.41.3/docs#tekton-pipelines
[v0-40-2-docs]: https://github.com/tektoncd/pipeline/tree/v0.40.2/docs#tekton-pipelines
[v0-39-0-docs]: https://github.com/tektoncd/pipeline/tree/v0.39.0/docs#tekton-pipelines
[v0-38-4-docs]: https://github.com/tektoncd/pipeline/tree/v0.38.4/docs#tekton-pipelines
[v0-37-5-docs]: https://github.com/tektoncd/pipeline/tree/v0.37.5/docs#tekton-pipelines

[v0.66-0-examples]: https://github.com/tektoncd/pipeline/tree/v0.66.0/examples#examples
[v0.65-0-examples]: https://github.com/tektoncd/pipeline/tree/v0.65.0/examples#examples
[v0.64-0-examples]: https://github.com/tektoncd/pipeline/tree/v0.64.0/examples#examples
[v0.63-0-examples]: https://github.com/tektoncd/pipeline/tree/v0.63.0/examples#examples
[v0.62-2-examples]: https://github.com/tektoncd/pipeline/tree/v0.62.2/examples#examples
[v0.62-1-examples]: https://github.com/tektoncd/pipeline/tree/v0.62.1/examples#examples
[v0.62-0-examples]: https://github.com/tektoncd/pipeline/tree/v0.62.0/examples#examples
[v0.61-0-examples]: https://github.com/tektoncd/pipeline/tree/v0.61.0/examples#examples
[v0.60-0-examples]: https://github.com/tektoncd/pipeline/tree/v0.60.0/examples#examples
[v0.60-1-examples]: https://github.com/tektoncd/pipeline/tree/v0.60.1/examples#examples
[v0.59-2-examples]: https://github.com/tektoncd/pipeline/tree/v0.59.2/examples#examples
[v0.59-1-examples]: https://github.com/tektoncd/pipeline/tree/v0.59.1/examples#examples
[v0.59-0-examples]: https://github.com/tektoncd/pipeline/tree/v0.59.0/examples#examples
[v0.58-0-examples]: https://github.com/tektoncd/pipeline/tree/v0.58.0/examples#examples
[v0.57-0-examples]: https://github.com/tektoncd/pipeline/tree/v0.57.0/examples#examples
[v0.56-3-examples]: https://github.com/tektoncd/pipeline/tree/v0.56.3/examples#examples
[v0.54-0-examples]: https://github.com/tektoncd/pipeline/tree/v0.54.0/examples#examples
[v0.53-5-examples]: https://github.com/tektoncd/pipeline/tree/v0.53.5/examples#examples
[v0.52-0-examples]: https://github.com/tektoncd/pipeline/tree/v0.52.0/examples#examples
[v0.51-0-examples]: https://github.com/tektoncd/pipeline/tree/v0.51.0/examples#examples
[v0.50-5-examples]: https://github.com/tektoncd/pipeline/tree/v0.50.5/examples#examples
[v0-49-0-examples]: https://github.com/tektoncd/pipeline/tree/v0.49.0/examples#examples
[v0-48-0-examples]: https://github.com/tektoncd/pipeline/tree/v0.48.0/examples#examples
[v0-47-8-examples]: https://github.com/tektoncd/pipeline/tree/v0.47.8/examples#examples
[v0-46-0-examples]: https://github.com/tektoncd/pipeline/tree/v0.46.0/examples#examples
[v0-45-0-examples]: https://github.com/tektoncd/pipeline/tree/v0.45.0/examples#examples
[v0-44-4-examples]: https://github.com/tektoncd/pipeline/tree/v0.44.4/examples#examples
[v0-43-2-examples]: https://github.com/tektoncd/pipeline/tree/v0.43.2/examples#examples
[v0-42-0-examples]: https://github.com/tektoncd/pipeline/tree/v0.42.0/examples#examples
[v0-41-3-examples]: https://github.com/tektoncd/pipeline/tree/v0.41.3/examples#examples
[v0-40-2-examples]: https://github.com/tektoncd/pipeline/tree/v0.40.2/examples#examples
[v0-39-0-examples]: https://github.com/tektoncd/pipeline/tree/v0.39.0/examples#examples
[v0-38-4-examples]: https://github.com/tektoncd/pipeline/tree/v0.38.4/examples#examples
[v0-37-5-examples]: https://github.com/tektoncd/pipeline/tree/v0.37.5/examples#examples
