# Tekton cli Repo CI/CD

We dogfood our project by using Tekton Pipelines to build, test and
release the Tekton Pipelines cli!

This directory contains the
[`Tasks`](https://github.com/tektoncd/pipeline/blob/master/docs/tasks.md) and
[`Pipelines`](https://github.com/tektoncd/pipeline/blob/master/docs/pipelines.md)
that we (will) use for the release and the pull-requests.

TODO(tektoncd/pipeline#538): In tektoncd/pipeline#538 or tektoncd/pipeline#537 we will update
[Prow](https://github.com/tektoncd/pipeline/blob/master/CONTRIBUTING.md#pull-request-process)
to invoke these `Pipelines` automatically, but for now we will have to invoke
them manually.

## Release Pipeline

You can use the script [`./release.sh`](release.sh) that would do everything
that needs to be done for the release.

The first argument if not provided is the release version you want to do, it
need to respect a format like `v1.2.3` which is basically SEMVER.

If it detects that you are doing a minor release it would ask you for some
commits to be cherry-picked in this release. Alternatively you can provide the
commits separed by a space to the second argument of the script. You do need to
make sure to provide them in order from the oldest to the newest (as listed).

If you give the `*` argument for the commits to pick up it would apply all the new
commits.

It will them use your TektonCD cluster and apply what needs to be done for
running the release.

And finally it will launch the `tkn`  cli to show the logs.

Make sure we have a nice ChangeLog before doign the release, listing `features`
and `bugs` and be thankful to the contributors by listing them.

## Debugging

You can define the env variable `PUSH_REMOTE` to push to your own remote (i.e:
which pont to your username on github),

You need to be **careful** if you have write access to the `homebrew` repository, it
would do a release there. Until we can find a proper fix I would generate a
commit which remove the brews data and cherry-pick it in the script.
