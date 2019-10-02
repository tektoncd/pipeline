## tkn pipeline start

Start pipelines

***Aliases**: trigger*

### Usage

```
tkn pipeline start pipeline [RESOURCES...] [PARAMS...] [SERVICEACCOUNT]
```

### Synopsis

To start a pipeline, you will need to pass the following:

- Resources
- Parameters, at least those that have no default value

### Examples

To run a Pipeline that has one git resource and no parameter.

	$ tkn pipeline start --resource source=samples-git


To run a Pipeline that has one git resource, one image resource and
two parameters (foo and bar)


	$ tkn pipeline start --resource source=samples-git \
		--resource image=my-image \
		--param foo=yay \
		--param bar=10

### Options

```
  -h, --help                          help for start
  -l, --labels strings                pass labels as label=value.
  -L, --last                          re-run the pipeline using last pipelinerun values
  -p, --param strings                 pass the param as key=value
  -r, --resource strings              pass the resource name and ref as name=ref
  -s, --serviceaccount string         pass the serviceaccount name
      --task-serviceaccount strings   pass the service account corresponding to the task
```

### Options inherited from parent commands

```
  -k, --kubeconfig string   kubectl config file (default: $HOME/.kube/config)
  -n, --namespace string    namespace to use (default: from $KUBECONFIG)
  -C, --nocolour            disable colouring (default: false)
```

### SEE ALSO

* [tkn pipeline](tkn_pipeline.md)	 - Manage pipelines

