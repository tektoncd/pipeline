# Contributing to Tekton

Thank you for contributing your time and expertise to Tekton. This
document describes the contribution guidelines for the project.

**Note:** Before you start contributing, you must read and abide by our **[Code of Conduct](./code-of-conduct.md)**.


## Contributing to Tekton code

To set up your environment and begin working on our code, see [Developing for Tekton](./DEVELOPMENT.md).

[The `community` repo](https://github.com/tektoncd/community) contains information on the following:

- [Development standards](https://github.com/tektoncd/community/blob/main/standards.md), including:
  - [Writing high quality code](https://github.com/tektoncd/community/blob/main/standards.md#coding-standards)
  - [Adopting good development principles](https://github.com/tektoncd/community/blob/main/standards.md#principles)
  - [Writing useful commit messages](https://github.com/tektoncd/community/blob/main/standards.md#commit-messages)
- [Contacting other contributors](https://github.com/tektoncd/community/blob/main/contact.md)
- [Tekton development processes](https://github.com/tektoncd/community/tree/main/process#readme), including:
  - [Finding things to work on](https://github.com/tektoncd/community/tree/main/process#finding-something-to-work-on)
  - [Proposing new features](https://github.com/tektoncd/community/tree/main/process#proposing-features)
  - [Performing code reviews](https://github.com/tektoncd/community/tree/main/process#reviews)
  - [Becoming a community member and maintainer](https://github.com/tektoncd/community/blob/main/process/contributor-ladder.md)
- [Making changes to the Tekton API](api_compatibility_policy.md#approving-api-changes)
- [Understanding the Tekton automation infrastructure](https://github.com/tektoncd/plumbing)

Additionally, please read the following resources specific to Tekton Pipelines:

- [Tekton Pipelines GitHub project](https://github.com/orgs/tektoncd/projects/3)
- [Tekton Pipelines roadmap](roadmap.md)
- [Tekton Pipelines API compatibility policy](api_compatibility_policy.md)

For support in contributing to specific areas, contact the relevant [Tekton Pipelines Topical Owner(s)](topical-ownership.md).

## Slash Commands

The project includes GitHub slash commands to automate common workflows:

### `/cherry-pick`

Automatically cherry-picks a merged PR to one or more target branches.

**Usage**: `/cherry-pick <target-branch> [<target-branch2> ...]`

**Examples**:
- `/cherry-pick release-v0.47.x`
- `/cherry-pick release-v0.47.x release-v1.3.x`

**Requirements**:
- PR must be merged
- User must have write permissions
- Target branch(es) must exist

The command creates a new PR with the cherry-picked changes for each target branch.

## Contributing to Tekton documentation

If you want to contribute to Tekton documentation, see the
[Tekton Documentation Contributor's Guide](https://github.com/tektoncd/website/blob/main/content/en/docs/Contribute/_index.md).

This guide describes:
- The contribution process for documentation
- Our standards for writing high quality content
- Our formatting conventions

It also includes a primer for getting started with writing documentation and improving your writing skills.
