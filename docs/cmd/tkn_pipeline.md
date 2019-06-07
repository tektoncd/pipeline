## tkn pipeline

Parent command of the `pipeline` command group

### Synopsis

Parent command of the `pipeline` command group

### Aliases

```
pipeline, p, pipelines
```

### Available Commands

```
describe    Describes a pipeline in a namespace
list        Lists pipelines in a namespace
```

### Options

```
-h, --help                help for pipeline
-k, --kubeconfig string   kubectl config file (default: $HOME/.kube/config)
-n, --namespace string    namespace to use (default: from $KUBECONFIG)
```

### Options inherited from parent commands

```
--azure-container-registry-config string   Path to the file containing Azure container registry configuration information.
```

### SEE ALSO

* [tkn pipeline list](tkn_pipeline_list.md)	 - Lists all `pipelines` in a given namespace.
* [tkn pipeline describe](tkn_pipeline_describe.md)	 - Describe given `pipeline`.