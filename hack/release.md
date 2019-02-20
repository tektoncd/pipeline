# Creating a new Tekton Pipeline release

The `release.sh` script automates the creation of Tekton Pipeline releases,
either nightly or versioned ones.

By default, the script creates a nightly release but does not publish it
anywhere.

## Common flags for cutting releases

The following flags affect the behavior of the script, no matter the type of the
release.

- `--skip-tests` Do not run tests before building the release. Otherwise, build,
  unit and end-to-end tests are run and they all must pass for the release to be
  built.
- `--tag-release`, `--notag-release` Tag (or not) the generated images with
  either `vYYYYMMDD-<commit_short_hash>` (for nightly releases) or `vX.Y.Z` for
  versioned releases. _For versioned releases, a tag is always added._
- `--release-gcs` Defines the GCS bucket where the manifests will be stored. By
  default, this is `knative-nightly/build-pipeline`. This flag is ignored if the
  release is not being published.
- `--release-gcr` Defines the GCR where the images will be stored. By default,
  this is `gcr.io/knative-nightly`. This flag is ignored if the release is not
  being published.
- `--publish`, `--nopublish` Whether the generated images should be published to
  a GCR, and the generated manifests written to a GCS bucket or not. If yes, the
  `--release-gcs` and `--release-gcr` flags can be used to change the default
  values of the GCR/GCS used. If no, the images will be pushed to the `ko.local`
  registry, and the manifests written to the local disk only (in the repository
  root directory).

## Creating nightly releases

Nightly releases are built against the current git tree. The behavior of the
script is defined by the common flags. You must have write access to the GCR and
GCS bucket the release will be pushed to, unless `--nopublish` is used.

Examples:

```bash
# Create and publish a nightly, tagged release.
./hack/release.sh --publish --tag-release

# Create release, but don't test, publish or tag it.
./hack/release.sh --skip-tests --nopublish --notag-release
```

## Creating versioned releases

_Note: only Knative admins can create versioned releases._

To specify a versioned release to be cut, you must use the `--version` flag.
Versioned releases are usually built against a branch in the Tekton Pipeline
repository, specified by the `--branch` flag.

- `--version` Defines the version of the release, and must be in the form
  `X.Y.Z`, where X, Y and Z are numbers.
- `--branch` Defines the branch in Tekton Pipeline repository from which the
  release will be built. If not passed, the `master` branch at HEAD will be
  used. This branch must be created before the script is executed, and must be
  in the form `release-X.Y`, where X and Y must match the numbers used in the
  version passed in the `--version` flag. This flag has no effect unless
  `--version` is also passed.
- `--release-notes` Points to a markdown file containing a description of the
  release. This is optional but highly recommended. It has no effect unless
  `--version` is also passed.

If this is the first time you're cutting a versioned release, you'll be prompted
for your GitHub username, password, and possibly 2-factor authentication
challenge before the release is published.

Since we are currently using
[the knative release scripts](vendor/github.com/knative/test-infra/scripts/release.sh#L404)
the title of the release will be _Knative Build Pipeline release vX.Y.Z_ and we
will manually need to change this to _Tekton Pipeline release vX.Y.Z_. It will
also be tagged _vX.Y.Z_ (both on GitHub and as a git annotated tag).

#### Release notes

Release notes will need to be manually collected for the release by looking at
the `Release Notes` section of every PR which has been merged between the last
release and the current one.

Visiting
[github.com/knative/build-pipeline/compare](https://github.com/knative/build-pipeline/compare)
will allow you to compare changes between git tags.
