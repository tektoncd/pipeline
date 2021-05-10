# Tekton Plumbing 2020 Roadmap

This is an incomplete list of work we hope to accomplish in 2020.

Highlights:

- [Release Automation](#release-automation)
- [Tekton Based CD](#tekton-base-cd)
- [Tekton Based CI](#tekton-base-ci)

## Release Automation

We do nightly releases for each Pipeline, Triggers and Dashboard. We deploy them
nightly to the Robocat cluster. We would like to verify those release by running nightly
tests in the Robocat cluster. Related: [#288](https://github.com/tektoncd/plumbing/issues/288).

Full releases automation:
- Triggering a release does not require access to the infra clusters
- Automated release note generation
- Post release automated integration testing


## Tekton Based CD

- Build and release bots and services.
- CD of Tekton resources.
- Fully automated setup of an infra-like cluster
- Remove boiler plate code in scheduled tasks [#262](https://github.com/tektoncd/plumbing/issues/262)
- CD `prow` and `dogfooding` clusters


## Tekton Based CI

Dogfooding:
- Simplify existing Tekton based CI with features from Pipeline and Triggers
  - Cloud events notifications
  - Task from OCI registries
  - Triggerable bundles?
- Make it easy to define new CI jobs
- Implement CI jobs using Tekton. Examples:
  - [#291](https://github.com/tektoncd/plumbing/issues/291)
  - [#288](https://github.com/tektoncd/plumbing/issues/288)
  - [#286](https://github.com/tektoncd/plumbing/issues/286)
  - [#283](https://github.com/tektoncd/plumbing/issues/283)
- Update the logs application to support CI requirements
- Build a way to track job runs:
  - GitHub App, own solution, testgrid?

Testing:
- Test infra changes in CI
- Portable tests
- Test against different cloud infrastructure

Infra:
- Make Tekton infra portable (e.g. GitLab or others)
  - Reduce GitHub specific solution