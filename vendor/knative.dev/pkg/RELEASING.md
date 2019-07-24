# Releasing knative/pkg

We release the components of Knative every 6 weeks. All of these components must
be moved to the latest "release" of the knative/pkg shared library prior to each
release, but likely this should happen incrementally over each milestone.

## Release Process.

### Step #1: Monday the week prior to each release.

On Monday of the week prior to the Knative release, each of the downstream
repositories should stage a `[WIP]` Pull Request that advances the knative/pkg
dependency to the latest commit.

At present, these downstream repositories include:

1. knative/serving
1. knative/eventing
1. knative/eventing-contrib
1. knative/build
1. knative/sample-controller
1. GoogleCloudPlatform/cloud-run-events

> The automation that auto-bumps these lives
> [here](https://github.com/mattmoor/knobots/tree/knative/cmd/periodic/kodata).

`Gopkg.toml` should look like:

```toml
[[override]]
  name = "knative.dev/pkg"
  branch = "master"
```

Then the following is run:

```shell
dep ensure -update knative.dev/pkg
./hack/update-codegen.sh
```

If problems are found, they are addressed and the update is merged, and this
process repeats until knative/pkg can be cleanly updated without any changes.

> If by mid-week, we do not have a clean PR in each repository, the person
> driving the update should escalate to avoid delaying the release.

### Step #2: Friday the week prior to each release.

A release branch is snapped on knative/pkg with the form `release-0.X` where `X`
reflects the forthcoming Knative release. The commit at which this branch is
snapped will be the version that has been staged into `[WIP]` PRs in every
repository.

These staging PRs are then updated to:

```toml
[[override]]
  name = "knative.dev/pkg"
  # The 0.X release branch.
  branch = "release-0.X"
```

The `[WIP]` is removed, and the PRs are reviewed and merged.

## Backporting Fixes

If a problem is found in knative/pkg in an older release and a fix must be
backported then it should first be fixed at HEAD (if still relevant). It should
be called out and likely discussed (with a lead) in the original PR whether the
fix is desired and eligible for back-porting to a release branch. This may raise
the review bar, or lead to trade-offs in design; it is better to front-load this
consideration to avoid delaying a cherry-pick.

Once that PR has merged (if still relevant), the same commit should be
cherry-picked onto `release-0.Y` and any merge problems fixed up.

Once the change is ready, a PR should be sent against `release-0.Y` with the
prefix `[RELEASE-0.Y] Your PR Title` to clearly designate this PR as targeting
the release branch. A lead will review and make a ruling on the PR, but if this
consideration was front-loaded, it should be a short review.

### Picking up fixes

Downstream repositories should reference `knative/pkg` release branches from
their own release branches, so to update the `knative/pkg` dependency we run:

```shell
dep ensure -update knative.dev/pkg
./hack/update-deps.sh
```
