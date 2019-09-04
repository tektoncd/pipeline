# Tekton Pipelines cli

[![Go Report Card](https://goreportcard.com/badge/tektoncd/cli)](https://goreportcard.com/report/tektoncd/cli)

The Tekton Pipelines cli project provides a CLI for interacting with
Tekton!

## Getting Started

### Installing `tkn`

Download the latest binary executable for your operating system:

* [Mac OS X](https://github.com/tektoncd/cli/releases/download/v0.3.0/tkn_0.3.0_Darwin_x86_64.tar.gz)

  ```shell
  # Get the tar.xz
  curl -LO https://github.com/tektoncd/cli/releases/download/v0.3.0/tkn_0.3.0_Darwin_x86_64.tar.gz
  # Extract tkn to your PATH (e.g. /usr/local/bin)
  sudo tar xvzf tkn_0.3.0_Darwin_x86_64.tar.gz -C /usr/local/bin tkn
  ```

  You can also use [@chmouel](https://github.com/chmouel)'s unofficial
  brew tap for the time being.

  ```shell
  brew tap chmouel/tektoncd-cli
  brew install tektoncd-cli
  ```

* [Linux AMD 64](https://github.com/tektoncd/cli/releases/download/v0.3.0/tkn_0.3.0_Linux_x86_64.tar.gz)

  ```shell
  # Get the tar.xz
  curl -LO https://github.com/tektoncd/cli/releases/download/v0.3.0/tkn_0.3.0_Linux_x86_64.tar.gz
  # Extract tkn to your PATH (e.g. /usr/local/bin)
  sudo tar xvzf tkn_0.3.0_Linux_x86_64.tar.gz -C /usr/local/bin/ tkn
  ```

* [Linux ARM 64](https://github.com/tektoncd/cli/releases/download/v0.3.0/tkn_0.3.0_Linux_arm64.tar.gz)

  ```shell
  # Get the tar.xz
  curl -LO https://github.com/tektoncd/cli/releases/download/v0.3.0/tkn_0.3.0_Linux_arm64.tar.gz
  # Extract tkn to your PATH (e.g. /usr/local/bin)
  sudo tar xvzf tkn_0.3.0_Linux_arm64.tar.gz -C /usr/local/bin/ tkn
  ```

If you have [go](https://golang.org/) installed, `GO111MODULE="on" go get github.com/tektoncd/cli@v0.3.0` is all you need!

### `tkn` as a `kubectl` plugin

`kubectl` will find any binary named `kubectl-*` on your PATH and consider it as a plugin.
After installing tkn, create a link as kubectl-tkn
  ```shell
$ ln -s /usr/local/bin/tkn /usr/local/bin/kubectl-tkn
  ```

`kubectl` will find any binary named `kubectl-*` on your `PATH` and consider it as a plugin

  ```shell
$ ./kubectl plugin list
/usr/local/bin/kubectl-tkn
  ```

### Useful Commands

The following commands help you understand and effectively use the Tekton CLI:

 * `tkn help:` Displays a list of the commands with helpful information.
 * [`tkn completion:`](docs/cmd/tkn_completion.md) Outputs a BASH or ZSH completion script for `tkn` to allow command completion with Tab.
 * [`tkn version:`](docs/cmd/tkn_version.md) Outputs the cli version.
 * [`tkn pipeline:`](docs/cmd/tkn_pipeline.md) Parent command of the Pipeline command group.
 * [`tkn pipelinerun:`](docs/cmd/tkn_pipelinerun.md) Parent command of the Pipelinerun command group.
 * [`tkn task:`](docs/cmd/tkn_task.md) Parent command of the Task command group.
 * [`tkn taskrun:`](docs/cmd/tkn_taskrun.md) Parent command of the Taskrun command group.
 * [`tkn clustertask:`](docs/cmd/tkn_clustertask.md) Parent command of the ClusterTask command group.
 * [`tkn resource:`](docs/cmd/tkn_resource.md) Parent command of the Resource command group.

For every `tkn` command, you can use `-h` or `--help` flags to display specific help for that command.

## Want to contribute

We are so excited to have you!

- See [CONTRIBUTING.md](CONTRIBUTING.md) for an overview of our processes
- See [DEVELOPMENT.md](DEVELOPMENT.md) for how to get started
- Look at our
  [good first issues](https://github.com/tektoncd/cli/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22)
  and our
  [help wanted issues](https://github.com/tektoncd/cli/issues?q=is%3Aissue+is%3Aopen+label%3A%22help+wanted%22)
