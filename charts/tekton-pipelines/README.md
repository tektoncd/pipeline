# Tekton-Pipelines Helm Chart

# Deployment with Helm

This chart can be deployed directly from github like so:

- $ helm repo add tekton-pipelines https://raw.githubusercontent.com/tektoncd/pipeline/master/repo/charts/
- $ helm install tekton-pipelines/tekton-pipelines --name <my-release> [--tls]

# Override / Add values

Look at values.yaml to see what options are currently configurable in the config templates.

Currently supported:

- namespace
    - annotations

## Development

Setup your helm cli, cozy up to gotpl and iterate.  Keep your config objects in order by prefixing with numbers.

- `brew install kubernetes-helm`
- `cd charts/tekton-pipelines`
- `helm lint`
- `helm template .`
- `helm install --dry-run --debug .`

Add values and template helpers as needed.  But you must ensure sensible zero-conf defaults.

## Release

When you're ready to release the configs, simply write them out to release.yaml.

- `helm lint`
- `helm template . > release.yaml`
