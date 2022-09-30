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
      20th, depending on week-ends and readyness

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

## Releases

### v0.40

- **Latest Release**: [v0.40.1][v0-40-1] (2022-09-27)
- **Initial Release**: [v0.40.0][v0-40-0] (2022-09-16)
- **End of Life**: 2023-01-15
- **Patch Releases**: [v0.40.0][v0-40-0], [v0.40.1][v0-40-1]

### v0.39

- **Latest Release**: [v0.39.0][v0-39-0] (2022-08-18)
- **Initial Release**: [v0.39.0][v0-39-0] (2022-08-18)
- **End of Life**: 2022-12-17
- **Patch Releases**: [v0.39.0][v0-39-0]

### v0.38

- **Latest Release**: [v0.38.4][v0-38-4] (2022-09-14)
- **Initial Release**: [v0.38.0][v0-38-0] (2022-07-25)
- **End of Life**: 2022-11-24
- **Patch Releases**: [v0.38.0][v0-38-0], [v0.38.1][v0-38-1], [v0.38.2][v0-38-2], [v0.38.3][v0-38-3], [v0.38.4][v0-38-4]

### v0.37

- **Latest Release**: [v0.37.5][v0-37-5] (2022-09-25)
- **Initial Release**: [v0.37.0][v0-37-0] (2022-06-21)
- **End of Life**: 2022-10-20
- **Patch Releases**: [v0.37.0][v0-37-0], [v0.37.1][v0-37-1], [v0.37.2][v0-37-2], [v0.37.3][v0-37-3], [v0.37.4][v0-37-4]，[v0.37.5][v0-37-5]

## End of Life Releases

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

[v0-40-1]: https://github.com/tektoncd/pipeline/releases/tag/v0.40.1
[v0-40-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.40.0
[v0-39-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.39.0
[v0-38-4]: https://github.com/tektoncd/pipeline/releases/tag/v0.38.4
[v0-38-3]: https://github.com/tektoncd/pipeline/releases/tag/v0.38.3
[v0-38-2]: https://github.com/tektoncd/pipeline/releases/tag/v0.38.2
[v0-38-1]: https://github.com/tektoncd/pipeline/releases/tag/v0.38.1
[v0-38-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.38.0
[v0-37-5]: https://github.com/tektoncd/pipeline/releases/tag/v0.37.5
[v0-37-4]: https://github.com/tektoncd/pipeline/releases/tag/v0.37.4
[v0-37-3]: https://github.com/tektoncd/pipeline/releases/tag/v0.37.3
[v0-37-2]: https://github.com/tektoncd/pipeline/releases/tag/v0.37.2
[v0-37-1]: https://github.com/tektoncd/pipeline/releases/tag/v0.37.1
[v0-37-0]: https://github.com/tektoncd/pipeline/releases/tag/v0.37.0
