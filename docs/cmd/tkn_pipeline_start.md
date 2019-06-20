## tkn pipeline start

Start the `pipeline`

### Synopsis

Start the `pipeline` by creating a pipelinerun

```
tkn pipeline start NAME [flags]
```

### Aliases

```
start, trigger
```

### Options

```
 -h, --help                          help for describe
 -r, --resource                      resource name and resource reference
 -p, --param                         param key and value
 -s --serviceaccount                 serviceaccount to run pipeline
```

### Options inherited from parent commands

```
--azure-container-registry-config string        Path to the file containing Azure container registry configuration information.
 -k, --kubeconfig string                        kubectl config file (default: $HOME/.kube/config)
 -n, --namespace string                         namespace to use (default: from $KUBECONFIG)
```

### SEE ALSO

* [tkn pipeline](tkn_pipeline.md)	 - Parent command of the `pipeline` command group
