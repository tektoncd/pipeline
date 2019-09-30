# Release creation

## Branching

As far as branching goes, we have two use-cases:

1. Creating a branch based off an upstream release tag.
2. Having a branch that follow upstream's HEAD and serves as a vehicle for continuous integration.

A prerequisite for both scripts is that your local clone of the repository has a remote "upstream"
that points to the upstream repository and a remote "openshift" that points to the openshift fork.

Run the scripts from the root of the repository.

### Creating a branch based off an upstream release tag

To create a clean branch from an upstream release tag, use the `create-release-branch.sh` script:

```bash
$ ./openshift/release/create-release-branch.sh v0.4.1 release-0.4
```

This will create a new branch "release-0.4" based off the tag "v0.4.1" and add all OpenShift specific
files that we need to run CI on top of it.

### Updating a branch that follow upstream's HEAD

To update a branch to the latest HEAD of upstream use the `update-to-head.sh` script:

```bash
$ ./openshift/release/update-to-head.sh release-0.5
```

That will pull the latest master from upstream, rebase the current fixes on the release-0.5 branch
on top of it and update the Openshift specific files if necessary.