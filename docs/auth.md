<!--
---
linkTitle: "Authentication"
weight: 301
---
-->

# Authentication at Run Time

This document describes how Tekton handles authentication when executing
`TaskRuns` and `PipelineRuns`. Since authentication concepts and processes
apply to both of those entities in the same manner, this document collectively
refers to `TaskRuns` and `PipelineRuns` as `Runs` for the sake of brevity.

- [Overview](#overview)
- [Understanding credential selection](#understanding-credential-selection)
- [Using `Secrets` as a non-root user](#using-secrets-as-a-non-root-user)
- [Limiting `Secret` access to specific `Steps`](#limiting-secret-access-to-specific-steps)
- [Configuring authentication for Git](#configuring-authentication-for-git)
  - [Configuring `basic-auth` authentication for Git](#configuring-basic-auth-authentication-for-git)
  - [Configuring `ssh-auth` authentication for Git](#configuring-ssh-auth-authentication-for-git)
  - [Using a custom port for SSH authentication](#using-a-custom-port-for-ssh-authentication)
  - [Using SSH authentication in `git` type `Tasks`](#using-ssh-authentication-in-git-type-tasks)
- [Configuring authentication for Docker](#configuring-authentication-for-docker)
  - [Configuring `basic-auth` authentication for Docker](#configuring-basic-auth-authentication-for-docker)
  - [Configuring `docker*` authentication for Docker](#configuring-docker-authentication-for-docker)
- [Technical reference](#technical-reference)
  - [`basic-auth` for Git](#basic-auth-for-git)
  - [`ssh-auth` for Git](#ssh-auth-for-git)
  - [`basic-auth` for Docker](#basic-auth-for-docker)
  - [Errors and their meaning](#errors-and-their-meaning)
    - ["unsuccessful cred copy" Warning](#unsuccessful-cred-copy-warning)
      - [Multiple Steps with varying UIDs](#multiple-steps-with-varying-uids)
      - [A Workspace or Volume is also Mounted for the same credentials](#a-workspace-or-volume-is-also-mounted-for-the-same-credentials)
      - [A Task employes a read-only-Workspace or Volume for `$HOME`](#a-task-employs-a-read-only-workspace-or-volume-for-home)
      - [The Step is named `image-digest-exporter`](#the-step-is-named-image-digest-exporter)
- [Disabling Tekton's Built-In Auth](#disabling-tektons-built-in-auth)
  - [Why would an organization want to do this?](#why-would-an-organization-want-to-do-this)
  - [What are the effects of making this change?](#what-are-the-effects-of-making-this-change)
  - [How to disable the built-in auth](#how-to-disable-the-built-in-auth)

## Overview

Tekton supports authentication for Git and Docker registries using Kubernetes Secrets. The supported Secret types are:

<table>
    <thead>
        <th>Git</th>
        <th>Docker</th>
    </thead>
    <tbody>
        <tr>
            <td><code>kubernetes.io/basic-auth</code><br>
                <code>kubernetes.io/ssh-auth</code>
            </td>
            <td><code>kubernetes.io/basic-auth</code><br>
                <code>kubernetes.io/dockercfg</code><br>
                <code>kubernetes.io/dockerconfigjson</code>
            </td>
    </tbody>
</table>

A TaskRun or PipelineRun (collectively referred to as "Run") gains access to these Secrets through its associated ServiceAccount. Each supported Secret must include a Tekton-specific annotation for Tekton to recognize and process it (see [Understanding Credential Selection](#understanding-credential-selection)).

Tekton handles authentication in two stages:

1. **Pod Scheduling and Image Pulling**: When a Run is executed, Tekton creates a Pod to run the defined Steps. During this Pod creation phase, Tekton uses the provided Docker credentials (if any) to authenticate with the respective registries and pull the required container images.

2. **Step Execution**: After the Pod is created and the images are pulled, Tekton executes the Steps within the Pod. At this stage, Tekton sets up the appropriate authentication credentials for Git and Docker within the container's environment, allowing the Steps to access and interact with the respective services.

To provide the appropriate credentials within the container's environment, Tekton performs the following credential initialization process for every Pod it creates:

1. Tekton accesses all Secrets associated with the Run and aggregates their contents into a temporary directory (`/tekton/creds`).

2. Tekton then copies or symlinks the relevant credential files from the temporary directory into the user's home directory (`$HOME`) within the container.

This process ensures that the container has access to the necessary Git and Docker credentials during Step execution, allowing the Steps to authenticate with the respective services.

For Git authentication, Tekton generates a `~/.gitconfig` file or a `~/.ssh` directory within the container.
For Docker authentication, Tekton generates a `~/.docker/config.json` file within the container.

Each Secret type has specific rules governing credential formatting and merging. Tekton follows these rules when aggregating and merging credentials of the supported types.

TODO(#5357): Update docs to explain recommended methods of passing secrets in via workspaces

## Understanding credential selection

A `Run` might require multiple types of authentication. For example, a `Run` might require access to
multiple private Git and Docker repositories. You must properly annotate each `Secret` to specify the
domains for which Tekton can use the credentials that the `Secret` contains. Tekton **ignores** all
`Secrets` that are not properly annotated.

A credential annotation key must begin with `tekton.dev/git-` or `tekton.dev/docker-` and its value is the
URL of the host for which you want Tekton to use that credential. In the following example, Tekton uses a
`basic-auth` (username/password pair) `Secret` to access Git repositories at `github.com` and `gitlab.com`
as well as Docker repositories at `gcr.io`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  annotations:
    tekton.dev/git-0: https://github.com
    tekton.dev/git-1: https://gitlab.com
    tekton.dev/docker-0: https://gcr.io
type: kubernetes.io/basic-auth
stringData:
  username: <cleartext username>
  password: <cleartext password>
```

And in this example, Tekton uses an `ssh-auth` `Secret` to access Git repositories
at `github.com` only:

```yaml
apiVersion: v1
kind: Secret
metadata:
  annotations:
    tekton.dev/git-0: github.com
type: kubernetes.io/ssh-auth
stringData:
  ssh-privatekey: <private-key>
  # This is non-standard, but its use is encouraged to make this more secure.
  # Omitting this results in the server's public key being blindly accepted.
```

## Using `Secrets` as a non-root user

In certain scenarios you might need to use `Secrets` as a non-root user. For example:

- Your platform randomizes the user and/or groups that your containers use to execute.
- The `Steps` in your `Task` define a non-root `securityContext`.
- Your `Task` specifies a global non-root `securityContext` that applies to all `Steps` in the `Task`.

The following are considerations for executing `Runs` as a non-root user:

- `ssh-auth` for Git requires the user to have a valid home directory configured in `/etc/passwd`.
  Specifying a UID that has no valid home directory results in authentication failure.
- Since SSH authentication ignores the `$HOME` environment variable, you must either move or symlink
  the appropriate `Secret` files from the `$HOME` directory defined by Tekton (`/tekton/home`) to
  the non-root user's valid home directory to use SSH authentication for either Git or Docker.

For an example of configuring SSH authentication in a non-root `securityContext`,
see [`authenticating-git-commands`](../examples/v1/taskruns/authenticating-git-commands.yaml).

## Limiting `Secret` access to specific `Steps`

As described earlier in this document, Tekton stores supported `Secrets` in
`$HOME/tekton/home` and makes them available to all `Steps` within a `Task`. 

If you want to limit a `Secret` to only be accessible to specific `Steps` but not
others, you must explicitly specify a `Volume` using the `Secret` definition and
manually `VolumeMount` it into the desired `Steps` instead of using the procedures
described later in this document.

## Configuring authentication for Git

This section describes how to configure the following authentication schemes for use with Git:

- [Configuring `basic-auth` authentication for Git](#configuring-basic-auth-authentication-for-git)
- [Configuring `ssh-auth` authentication for Git](#configuring-ssh-auth-authentication-for-git)
- [Using a custom port for SSH authentication](#using-a-custom-port-for-ssh-authentication)
- [Using SSH authentication in `git` type `Tasks`](#using-ssh-authentication-in-git-type-tasks)

### Configuring `basic-auth` authentication for Git

This section describes how to configure a `basic-auth` type `Secret` for use with Git. In the example below,
before executing any `Steps` in the `Run`, Tekton creates a `~/.gitconfig` file containing the credentials
specified in the `Secret`. 

Note: Github deprecated basic authentication with username and password. You can still use basic authentication, but you wil need to use a personal access token instead of the cleartext password in the following example. You can find out how to create such a token on the [Github documentation site](https://docs.github.com/en/github/authenticating-to-github/creating-a-personal-access-token).

1. In `secret.yaml`, define a `Secret` that specifies the username and password that you want Tekton
   to use to access the target Git repository:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: basic-user-pass
     annotations:
       tekton.dev/git-0: https://github.com # Described below
   type: kubernetes.io/basic-auth
   stringData:
     username: <cleartext username>
     password: <cleartext password>
   ```

   In the above example, the value for `tekton.dev/git-0` specifies the URL for which Tekton will use this `Secret`,
   as described in [Understanding credential selection](#understanding-credential-selection).

2. In `serviceaccount.yaml`, associate the `Secret` with the desired `ServiceAccount`:

   ```yaml
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: build-bot
   secrets:
     - name: basic-user-pass
   ```

3. In `run.yaml`, associate the `ServiceAccount` with your `Run` by doing one of the following:

   - Associate the `ServiceAccount` with your `TaskRun`:

     ```yaml
     apiVersion: tekton.dev/v1beta1
     kind: TaskRun
     metadata:
       name: build-push-task-run-2
     spec:
       serviceAccountName: build-bot
       taskRef:
         name: build-push
     ```
   - Associate the `ServiceAccount` with your `PipelineRun`:

     ```yaml
     apiVersion: tekton.dev/v1beta1
     kind: PipelineRun
     metadata:
       name: demo-pipeline
       namespace: default
     spec:
       serviceAccountName: build-bot
       pipelineRef:
         name: demo-pipeline
     ```

4. Execute the `Run`:

   ```shell
   kubectl apply --filename secret.yaml serviceaccount.yaml run.yaml
   ```

### Configuring `ssh-auth` authentication for Git

This section describes how to configure an `ssh-auth` type `Secret` for use with Git. In the example below,
before executing any `Steps` in the `Run`, Tekton creates a `~/.ssh/config` file containing the SSH key
specified in the `Secret`.

1. In `secret.yaml`, define a `Secret` that specifies your SSH private key:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: ssh-key
     annotations:
       tekton.dev/git-0: github.com # Described below
   type: kubernetes.io/ssh-auth
   stringData:
     ssh-privatekey: <private-key>
     # This is non-standard, but its use is encouraged to make this more secure.
     # If it is not provided then the git server's public key will be requested
     # when the repo is first fetched.
     known_hosts: <known-hosts>
   ```

   In the above example, the value for `tekton.dev/git-0` specifies the URL for which Tekton will use this `Secret`,
   as described in [Understanding credential selection](#understanding-credential-selection).

2. Generate the `ssh-privatekey` value. For example:

   `cat ~/.ssh/id_rsa`

3. Set the value of the `known_hosts` field to the generated `ssh-privatekey` value from the previous step.

4. In `serviceaccount.yaml`, associate the `Secret` with the desired `ServiceAccount`:

   ```yaml
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: build-bot
   secrets:
     - name: ssh-key
   ```

5. In `run.yaml`, associate the `ServiceAccount` with your `Run` by doing one of the following:

   - Associate the `ServiceAccount` with your `TaskRun`:

     ```yaml
     apiVersion: tekton.dev/v1beta1
     kind: TaskRun
     metadata:
       name: build-push-task-run-2
     spec:
       serviceAccountName: build-bot
       taskRef:
         name: build-push
     ```

   - Associate the `ServiceAccount` with your `PipelineRun`:

   ```yaml
   apiVersion: tekton.dev/v1beta1
   kind: PipelineRun
   metadata:
     name: demo-pipeline
     namespace: default
   spec:
     serviceAccountName: build-bot
     pipelineRef:
       name: demo-pipeline
   ```

6. Execute the `Run`:

   ```shell
   kubectl apply --filename secret.yaml,serviceaccount.yaml,run.yaml
   ```

### Using a custom port for SSH authentication

You can specify a custom SSH port in your `Secret`. 

```
apiVersion: v1
kind: Secret
metadata:
  name: ssh-key-custom-port
  annotations:
    tekton.dev/git-0: example.com:2222
type: kubernetes.io/ssh-auth
stringData:
  ssh-privatekey: <private-key>
  known_hosts: <known-hosts>
```

### Using SSH authentication in `git` type `Tasks`

You can use SSH authentication as described earlier in this document when invoking `git` commands
directly in the `Steps` of a `Task`. Since `ssh` ignores the `$HOME` variable and only uses the
user's home directory specified in `/etc/passwd`, each `Step` must symlink `/tekton/home/.ssh`
to the home directory of its associated user.

**Note:** This explicit symlinking is not necessary when using the
[`git-clone` `Task`](https://github.com/tektoncd/catalog/tree/v1beta1/git) from Tekton Catalog.

For example usage, see [`authenticating-git-commands`](../examples/v1/taskruns/authenticating-git-commands.yaml).

## Configuring authentication for Docker

This section describes how to configure the following authentication schemes for use with Docker:

- [Configuring `basic-auth` authentication for Docker](#configuring-basic-auth-authentication-for-docker)
- [Configuring `docker*` authentication for Docker](#configuring-docker-authentication-for-docker)

### Configuring `basic-auth` authentication for Docker

This section describes how to configure the `basic-auth` (username/password pair) type `Secret` for use with Docker.

In the example below, before executing any `Steps` in the `Run`, Tekton creates a `~/.docker/config.json` file containing
the credentials specified in the `Secret`.

1. In `secret.yaml`, define a `Secret` that specifies the username and password that you want Tekton
   to use to access the target Docker registry:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: basic-user-pass
     annotations:
       tekton.dev/docker-0: https://gcr.io # Described below
   type: kubernetes.io/basic-auth
   stringData:
     username: <cleartext username>
     password: <cleartext password>
   ```

   In the above example, the value for `tekton.dev/docker-0` specifies the URL for which Tekton will use this `Secret`,
   as described in [Understanding credential selection](#understanding-credential-selection).

2. In `serviceaccount.yaml`, associate the `Secret` with the desired `ServiceAccount`:

   ```yaml
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: build-bot
   secrets:
     - name: basic-user-pass
   ```

3. In `run.yaml`, associate the `ServiceAccount` with your `Run` by doing one of the following:

   - Associate the `ServiceAccount` with your `TaskRun`:

     ```yaml
     apiVersion: tekton.dev/v1beta1
     kind: TaskRun
     metadata:
       name: build-push-task-run-2
     spec:
       serviceAccountName: build-bot
       taskRef:
         name: build-push
     ```

   - Associate the `ServiceAccount` with your `PipelineRun`:

     ```yaml
     apiVersion: tekton.dev/v1beta1
     kind: PipelineRun
     metadata:
       name: demo-pipeline
       namespace: default
     spec:
       serviceAccountName: build-bot
       pipelineRef:
         name: demo-pipeline
     ```

4. Execute the `Run`:

   ```shell
   kubectl apply --filename secret.yaml serviceaccount.yaml run.yaml
   ```

## Configuring `docker*` authentication for Docker

This section describes how to configure Docker authentication using the `dockercfg`
and `dockerconfigjson` Secret types.

### Image Pulling Authentication
The `dockercfg` and `dockerconfigjson` Secrets are used as `imagePullSecrets` to
provide the necessary credentials for Tekton to pull container images from
private Docker registries during the Pod creation phase.

When a `Run` is executed, Tekton creates a `Pod` to run the defined Steps. During 
this `Pod` creation phase, Tekton uses the provided `imagePullSecrets` to 
authenticate with the respective registries and pull the required container 
images. This image pulling process is a crucial step that happens before the 
Steps within the `Pod` can start executing.

Note: If you specify both the Tekton `basic-auth` and the Kubernetes `dockercfg`
or `dockerconfigjson` Secrets, Tekton merges all credentials from all specified 
`Secrets`, but Tekton's basic-auth Secret overrides the Kubernetes Secrets.

1. Define a `Secret` based on your Docker client configuration file.
   
   ```bash
   kubectl create secret generic regcred \
    --from-file=.dockerconfigjson=<path/to/.docker/config.json> \
    --type=kubernetes.io/dockerconfigjson
   ```
   For more information, see [Pull an Image from a Private Registry](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/)
   in the Kubernetes documentation.

2. In `serviceaccount.yaml`, associate the `Secret` with the desired `ServiceAccount`:

   ```yaml
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: build-bot
   secrets:
     - name: regcred
   ```

3. In `run.yaml`, associate the `ServiceAccount` with your `Run` by doing one of the following:

   - Associate the `ServiceAccount` with your `TaskRun`:

     ```yaml
     apiVersion: tekton.dev/v1beta1
     kind: TaskRun
     metadata:
       name: build-with-basic-auth
     spec:
       serviceAccountName: build-bot
       steps:
       # ...
     ```

   - Associate the `ServiceAccount` with your `PipelineRun`:

     ```yaml
     apiVersion: tekton.dev/v1beta1
     kind: PipelineRun
     metadata:
       name: demo-pipeline
       namespace: default
     spec:
       serviceAccountName: build-bot
       pipelineRef:
         name: demo-pipeline
     ```

4. Execute the build:

   ```shell
   kubectl apply --filename secret.yaml --filename serviceaccount.yaml --filename taskrun.yaml
   ```

### In-Pod Docker Authentication
In addition to image pulling authentication, Tekton also sets up Docker 
authentication within the Pod's container environment. This authentication setup 
allows the Steps within the `Pod` to interact with Docker registries during 
execution, enabling operations like pushing or pulling images, or performing 
other OCI image manipulations.

After the Pod is created and the container images are pulled, Tekton generates 
a `~/.docker/config.json` file within the container. This file contains the 
necessary Docker authentication credentials, allowing the Steps to authenticate 
with Docker registries during execution.

The Docker authentication credentials used for in-Pod authentication are derived
from the same Secrets specified as `imagePullSecrets`. Tekton follows the credential 
formatting and merging rules defined by the `dockercfg` and `dockerconfigjson` Secret 
types when generating the `~/.docker/config.json` file.


## Technical reference

This section provides a technical reference for the implementation of the authentication mechanisms
described earlier in this document.

### `basic-auth` for Git

Given URLs, usernames, and passwords of the form: `https://url{n}.com`,
`user{n}`, and `pass{n}`, Tekton generates the following:

```
=== ~/.gitconfig ===
[credential]
    helper = store
[credential "https://url1.com"]
    username = "user1"
[credential "https://url2.com"]
    username = "user2"
...
=== ~/.git-credentials ===
https://user1:pass1@url1.com
https://user2:pass2@url2.com
...
```

### `ssh-auth` for Git

Given hostnames, private keys, and `known_hosts` of the form: `url{n}.com`,
`key{n}`, and `known_hosts{n}`, Tekton generates the following. 

By default, if no value is specified for `known_hosts`, Tekton configures SSH to accept
**any public key** returned by the server on first query. Tekton does this
by setting Git's `core.sshCommand` variable to `ssh -o StrictHostKeyChecking=accept-new`.
This behaviour can be prevented
[using a feature-flag: `require-git-ssh-secret-known-hosts`](./install.md#customizing-the-pipelines-controller-behavior).
Set this flag to `true` and all Git SSH Secrets _must_ include a `known_hosts`.

```
=== ~/.ssh/id_key1 ===
{contents of key1}
=== ~/.ssh/id_key2 ===
{contents of key2}
...
=== ~/.ssh/config ===
Host url1.com
    HostName url1.com
    IdentityFile ~/.ssh/id_key1
Host url2.com
    HostName url2.com
    IdentityFile ~/.ssh/id_key2
...
=== ~/.ssh/known_hosts ===
{contents of known_hosts1}
{contents of known_hosts2}
...
```

### `basic-auth` for Docker

Given URLs, usernames, and passwords of the form: `https://url{n}.com`,
`user{n}`, and `pass{n}`, Tekton generates the following. Since Docker doesn't
support the `kubernetes.io/ssh-auth` type `Secret`, Tekton ignores annotations
on `Secrets` of that type.

```
=== ~/.docker/config.json ===
{
  "auths": {
    "https://url1.com": {
      "auth": "$(echo -n user1:pass1 | base64)",
      "email": "not@val.id",
    },
    "https://url2.com": {
      "auth": "$(echo -n user2:pass2 | base64)",
      "email": "not@val.id",
    },
    ...
  }
}
```

## Errors and their meaning

### "unsuccessful cred copy" Warning

This message has the following format:

> `warning: unsuccessful cred copy: ".docker" from "/tekton/creds" to
> "/tekton/home": unable to open destination: open
> /tekton/home/.docker/config.json: permission denied`

The precise credential and paths mentioned can vary. This message is only a
warning but can be indicative of the following problems:

#### Multiple Steps with varying UIDs

Multiple Steps with different users / UIDs are trying to initialize docker
or git credentials in the same Task. If those Steps need access to the
credentials then they may fail as they might not have permission to access them.

This happens because, by default, `/tekton/home` is set to be a Step user's home
directory and Tekton makes this directory a shared volume that all Steps in a
Task have access to. Any credentials initialized by one Step are overwritten
by subsequent Steps also initializing credentials.

If the Steps reporting this warning do not use the credentials mentioned
in the message then you can safely ignore it.

This can most easily be resolved by ensuring that each Step executing in your
Task and TaskRun runs with the same UID. A blanket UID can be set with [a
TaskRun's `Pod template` field](./taskruns.md#specifying-a-pod-template).

If you require Steps to run with different UIDs then you should disable
Tekton's built-in credential initialization and use Workspaces to mount
credentials from Secrets instead. See [the section on disabling Tekton's
credential initialization](#disabling-tektons-built-in-auth).

#### A Workspace or Volume is also Mounted for the same credentials

A Task has mounted both a Workspace (or Volume) for credentials and the TaskRun
has attached a service account with git or docker credentials that Tekton will
try to initialize.

The simplest solution to this problem is to not mix credentials mounted via
Workspace with those initialized using the process described in this document.
See [the section on disabling Tekton's credential initialization](#disabling-tektons-built-in-auth).

#### A Task employs a read-only Workspace or Volume for `$HOME`

A Task has mounted a read-only Workspace (or Volume) for the user's `HOME`
directory and the TaskRun attaches a service account with git or docker
credentials that Tekton will try to initialize.

The simplest solution to this problem is to not mix credentials mounted via
Workspace with those initialized using the process described in this document.
See [the section on disabling Tekton's credential initialization](#disabling-tektons-built-in-auth).

#### The contents of `$HOME` are `chown`ed to a different user

A Task Step that modifies the ownership of files in the user home directory
may prevent subsequent Steps from initializing credentials in that same home
directory. The simplest solution to this problem is to avoid running chown
on files and directories under `/tekton`. Another option is to run all Steps
with the same UID.

#### The Step is named `image-digest-exporter`

If you see this warning reported specifically by an `image-digest-exporter` Step
you can safely ignore this message. The reason it appears is that this Step is
injected by Tekton and it runs with a non-root UID
that can differ from those of the Steps in the Task. The Step does not use
these credentials.

---

## Disabling Tekton's Built-In Auth

### Why would an organization want to do this?

There are a number of reasons that an organization may want to disable
Tekton's built-in credential handling:

1. The mechanism can be quite difficult to debug.
2. There are an extremely limited set of supported credential types.
3. Tasks with Steps that have different UIDs can break if multiple Steps
are trying to share access to the same credentials.
4. Tasks with Steps that have different UIDs can log more warning messages,
creating more noise in TaskRun logs. Again this is because multiple Steps
with differing UIDs cannot share access to the same credential files.


### What are the effects of making this change?

1. Credentials must now be passed explicitly to Tasks either with [Workspaces](./workspaces.md#using-workspaces-in-tasks),
environment variables (using [`envFrom`](https://kubernetes.io/docs/concepts/configuration/secret/#use-case-as-container-environment-variables) in your Steps and a Task param to
specify a Secret), or a custom volume and volumeMount definition.

### How to disable the built-in auth

To disable Tekton's built-in auth, edit the `feature-flag` `ConfigMap` in the
`tekton-pipelines` namespace and update the value of `disable-creds-init`
from `"false"` to `"true"`.

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
