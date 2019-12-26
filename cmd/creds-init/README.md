# creds-init

`creds-init` initializes credentials from the provided flags and the
mounted secrets. This currently supports:

- git credentials
- docker config credentials

## git credentials

The binary will either create an ssh configuration file (with
`-ssh-git` flag) or a git configuration [`.gitconfig`]() file and a
git credential [`.git-credentials`]() file (with `-basic-git` flag).

### `-ssh-git`

This uses the `ssh-privatekey` and `known_hosts` keys of the secret to generate:
- a `~/.ssh/id_{secret}` private key
- a `~/.ssh/config` file
- a `~/.ssh/known_hosts`

With a `Secret` that looks like:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ssh-key
  annotations:
    tekton.dev/git-0: github.com # Described below
type: kubernetes.io/ssh-auth
data:
  ssh-privatekey: <base64 encoded>
  # This is non-standard, but its use is encouraged to make this more secure.
  known_hosts: <base64 encoded>
```

The flag `-ssh-git=ssh-key=github.com` (with the environment variable
`HOME=/tekton/home`) would result with the following files:

- `~/.ssh/config`

	```
	HostName github.com
	IdentityFile /tekton/home/.ssh/id_foo
	Port 22
	```
- `~/.ssh/id_rsa` with the content of `ssh-privatekey` decoded
- `~/.ssh/known_hosts` with the content of `known_hosts` decoded


### `-basic-git`

This uses `username` and `password` credentials from a
`kubernetes.io/basic-auth` secret and add it in the generated docker's
`.gitconfig` file.

With a `Secret` that looks like:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: foo
  annotations:
    tekton.dev/git-0: https://github.com # Described below
type: kubernetes.io/basic-auth
stringData:
  username: <username>
  password: <password>
```

The flag `-basic-git=foo=github.com` (with the environment variable
`HOME=/tekton/home`) would result of the following files:

- `/tekton/home/.gitconfig`

  ```
  [credential]
	  helper = store
  [credential "https://github.com"]
	  username = <username>
  ```

- `/tekton/home/.git-credentials`

  ```
  https://<username>:<password>@github.com
  ```

## docker credentials

The binary will create a Docker [`config.json`
file](https://docs.docker.com/engine/reference/commandline/cli/#configuration-files)
with the provided flags (either `-basic-docker`, `-docker-config` or
`-docker-cfg`). This is documented
[here](https://github.com/tektoncd/pipeline/blob/master/docs/auth.md#basic-authentication-docker).

If all the following flag are provided (`-basic-docker`,
`-docker-config` and `-docker-cfg`), `creds-init` will merge the
credentials from those ; `-basic-auth` taking precedence over
`-docker-config` taking precedence over `-docker-cfg`.

### `-basic-docker`

This uses `username` and `password` credentials from a
`kubernetes.io/basic-auth` secret and add it in the generated docker's
`config.json` file.

With a `Secret` that looks like:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: foo
type: kubernetes.io/basic-auth
stringData:
  username: admin
  password: foobar
```

The flag `-basic-docker=foo=https://us.gcr.io` would result of a
docker's `config.json` file looking like:

```json
{
	"auths": {
		"https://us.gcr.io" : {
			"username": "admin",
			"password": "foobar",
			"auth": "YWRtaW46Zm9vYmFy"
		}
	}
}
```

Note that `auth` field is `base64(username+":"+password)`.

### `-docker-config`

This uses the `config.json` key from a secret of type
`kubernetes.io/dockerconfigjson` to populate the generated docker's
`config.json` file.

### `-docker-cfg`

This uses the `.dockercfg` key from a secret of type
`kubernetes.io/dockercfg` to populate the generated docker's
`config.json` file. The `.dockercfg` file is the old, deprecated
docker's client configuration format.
