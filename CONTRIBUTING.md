# Contributing to Tekton Pipelines

Welcome to the Tekton Pipelines project! Thanks for considering contributing to
our project and we hope you'll enjoy it :D

**All contributors must comply with
[the code of conduct](./code-of-conduct.md).**

To get started developing, see our [DEVELOPMENT.md](./DEVELOPMENT.md).

In this file you'll find info on:

- [Contacting other contributors](#contact)
- [Principles](#principles)
- [Proposing features](#proposing-features)
- [Development process](#development-process)
- [The pull request process](#pull-request-process) and
  [Prow commands](#prow-commands)
- [Standards](#standards) around [commit messages](#commit-messages) and
  [code](#coding-standards)
- [Finding something to work on](#finding-something-to-work-on)
- [The roadmap](#roadmap)
- [API compatibility policy](#api-compatibility-policy)

_See also
[the contribution guidelines for Knative](https://github.com/knative/docs/blob/master/community/CONTRIBUTING.md)._

## Contact

This work is being done by the Tekton Pipeline working group. If you are
interested please join our meetings
[at 9am PST on Tuesdays](https://calendar.google.com/event?action=TEMPLATE&tmeid=amZyNTljOWpkZWdibmpsY3JlazNodDU5NWdfMjAxOTAzMjZUMTYwMDAwWiBnb29nbGUuY29tX2Qzb3Zjdm8xcDMyMTloOTg5NTczdjk4Zm5zQGc&tmsrc=google.com_d3ovcvo1p3219h989573v98fns%40group.calendar.google.com&scp=ALL)
and or in slack at
[`#build-pipeline`](https://knative.slack.com/messages/build-pipeline)!

All docs shared with this group are made visible to members of
[tekton-dev@](https://groups.google.com/forum/#!forum/tekton-dev), please join
if you are interested!

## Principles

When possbile, try to practice:

- **Documentation driven development** - Before implementing anything, write
  docs to explain how it will work
- **Test driven development** - Before implementing anything, write tests to
  cover it

Minimize the number of integration tests written and maximize the unit tests!
Unit test coverage should increase or stay the same with every PR.

This means that most PRs should include both:

- [Tests](https://github.com/tektoncd/pipeline/tree/master/test#tests)
- [Documentation](https://github.com/tektoncd/pipeline/tree/master/docs)
  explaining features being added, including updates to
  [DEVELOPMENT.md](./DEVELOPMENT.md) if required

## Proposing features

If you have an idea for a feature, or if you have a solution for an existing
issue that involves an API change (i.e. changes
[the structure of a CRD](api_compatibility_policy.md#what-does-compatibility-mean-here)),
we highly suggest that you propose the changes before implementing them.

This is for two main reasons:

1. [Yes is forever](https://twitter.com/solomonstre/status/715277134978113536)
2. It's easier/cheaper to make changes before implementation (and you'll feel
   less emotionally invested!)

Some suggestions for how to do this:

1. Write up a design doc and share it with
   [tekton-dev@](https://groups.google.com/forum/#!forum/tekton-dev)
2. Bring your design/ideas to [our working group meetings](#contact) for
   discussion

A great proposal will include:

- **The use case(s) it solves** Who needs this and why?
- **Requirements** What needs to be true about the solution?
- **2+ alternative proposals** Even if alternatives aren't obvious, forcing
  yourself to brainstorm a couple more approaches may give you new ideas or make
  clear that your initial proposal is the best one

Also feel free to reach out to us on [slack](#contact) if you want any
help/guidance.

Thanks so much!!

## Development Process

Our contributors are made up of:

- A core group of OWNERS (defined in [OWNERS](OWNERS)) who can
  [approve PRs](#getting-sign-off)
- Any and all other contributors!

If you are interested in becoming an OWNER, take a look at the
[approver requirements](https://github.com/knative/docs/blob/master/contributing/ROLES.md#approver)
and follow up with an existing OWNER [on slack](#contact).

### OWNER review process

Reviewers will be auto-assigned by [Prow](#pull-request-process) from the
[OWNERS file](OWNERS), which acts as suggestions for which `OWNERS` should
review the PR. Your review requests can be viewed at
[https://github.com/pulls/review-requested](https://github.com/pulls/review-requested).

`OWNERS` who prepared to give the final `/approve` and `/lgtm` for a PR should
use the `assign` button to indicate they are willing to own the review of that
PR.

### Project stuff

As the project progresses we define
[milestones](https://help.github.com/articles/about-milestones/) to indicate
what functionality the OWNERS are focusing on.

If you are interested in contributing but not an OWNER, feel free to take on
something from the milestone but
[be aware of the contributor SLO](#contributor-slo).

You can see more details (including a burndown, issues in epics, etc.) on our
[zenhub board](https://app.zenhub.com/workspaces/pipelines-5bc61a054b5806bc2bed4fb2/boards?repos=146641150).
To see this board, you must:

- Ask [an OWNER](OWNERS) via [slack](https://knative.slack.com) for an
  invitation
- Add [the zenhub browser extension](https://www.zenhub.com/extension) to see
  new info via GitHub (or just use zenhub.com directly)

## Pull Request Process

This repo uses [Prow](https://github.com/kubernetes/test-infra/tree/master/prow)
and related tools like
[Tide](https://github.com/kubernetes/test-infra/tree/master/prow/tide) and
[Gubernator](https://github.com/kubernetes/test-infra/tree/master/gubernator)
(see
[Knative Prow](https://github.com/knative/test-infra/blob/master/ci/prow/prow_setup.md)).
This means that automation will be applied to your pull requests.

_See also
[Knative docs on reviewing](https://github.com/knative/docs/blob/master/community/REVIEWING.md)._

### Prow configuration

See [infra/README.md](./infra/README.md).

### Prow commands

Prow has a [number of commands](https://prow.knative.dev/command-help) you can
use to interact with it. More on the Prow proces in general
[is available in the k8s docs](https://github.com/kubernetes/community/blob/master/contributors/guide/owners.md#the-code-review-process).

#### Getting sign off

Before a PR can be merged, it must have both `/lgtm` AND `/approve`:

- `/lgtm` can be added by anyone in
  [the knative org](https://github.com/orgs/knative/people)
- `/approve` can be added only by
  [OWNERS](https://github.com/tektoncd/pipeline/blob/master/OWNERS)

[OWNERS](https://github.com/tektoncd/pipeline/blob/master/OWNERS) automatically
get `/approve` but still will need an `/lgtm` to merge.

The merge will happen automatically once the PR has both `/lgtm` and `/approve`,
and all tests pass. If you don't want this to happen you should
[`/hold`](#preventing-the-merge) the PR.

Any changes will cause the `/lgtm` label to be removed and it will need to be
re-applied.

#### Preventing the merge

If you don't a PR to be merged, e.g. so that the author can make changes, you
can put it on hold with `/hold`. Remove the hold with `/hold cancel`.

#### Tests

If you are a member of
[the knative org](https://github.com/orgs/knative/people), tests (required to
merge) will be automatically kicked off for your PR. If not,
[someone with `/lgtm` or `/approve` permission](#getting-sign-off) will need to
add `/ok-to-test` to your PR to allow the tests to run.

When tests (run by Prow) fail, it will add a comment to the PR with the commands
to re-run any failing tests

#### Cats and dogs

You can add dog and cat pictures with `/woof` and `/meow`.

## Standards

This section describes the standards we will try to maintain in this repo.

### Commit Messages

All commit messages should follow
[these best practices](https://chris.beams.io/posts/git-commit/), specifically:

- Start with a subject line
- Contain a body that explains _why_ you're making the change you're making
- Reference an issue number one exists, closing it if applicable (with text such
  as
  ["Fixes #245" or "Closes #111"](https://help.github.com/articles/closing-issues-using-keywords/))

Aim for [2 paragraphs in the body](https://www.youtube.com/watch?v=PJjmw9TRB7s).
Not sure what to put? Include:

- What is the problem being solved?
- Why is this the best approach?
- What other approaches did you consider?
- What side effects will this approach have?
- What future work remains to be done?

### Coding standards

The code in this repo should follow best practices, specifically:

- [Go code review comments](https://github.com/golang/go/wiki/CodeReviewComments)

## Finding something to work on

Thanks so much for considering contributing to our project!! We hope very much
you can find something interesting to work on:

- To find issues that we particularly would like contributors to tackle, look
  for
  [issues with the "help wanted" label](https://github.com/tektoncd/pipeline/issues?q=is%3Aissue+is%3Aopen+label%3A%22help+wanted%22).
- Issues that are good for new folks will additionally be marked with
  ["good first issue"](https://github.com/tektoncd/pipeline/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22).

### Assigning yourself an issue

To assign an issue to a user (or yourself), use GitHub or the Prow command by
writing a comment in the issue such as:

```none
/assign @your_github_username
```

Unfortunately, GitHub will only allow issues to be assigned to users who are
part of [the knative org](https://github.com/orgs/knative/people).

But don't let that stop you! **Leave a comment in the issue indicating you would
like to work on it** and we will consider it assigned to you.

### Contributor SLO

If you declare your intention to work on an issue:

- If it becomes urgent that the issue be resolved (e.g. critical bug or nearing
  the end of [a milestone](#project-stuff)), someone else may take over
  (apologies if this happens!!)
- If you do not respond to queries on an issue within about 3 days and someone
  else wants to work on your issue, we will assume you are no longer interested
  in working on it and it is fair game to assign to someone else (no worries at
  all if this happens, we don't mind!)

## Roadmap

The project's roadmap for 2019 is published [here](./roadmap-2019.md).

## API compatibility policy

The API compatibility policy (i.e. the policy for making backwards incompatible
API changes) can be found [here](api_compatibility_policy.md).
