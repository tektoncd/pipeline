## tkn pipelinerun

Parent command of the `pipelinerun` command group

### Synopsis

Parent command of the `pipelinerun` command group

### Aliases

```
pipelinerun, pr, pipelineruns
```

### Available Commands

```
describe    Describe a pipelinerun in a namespace
list        Lists pipelineruns in a namespace
logs        Display pipelinerun logs

```

### Options

```
-h, --help                help for pipelinerun
-k, --kubeconfig string   kubectl config file (default: $HOME/.kube/config)
-n, --namespace string    namespace to use (default: from $KUBECONFIG)
```

### SEE ALSO

* [tkn pipelinerun list](tkn_pipelinerun_list.md)	 - Lists all the pipelineruns in a given namespace.
* [tkn pipelinerun describe](tkn_pipelinerun_describe.md)	 - Describe given `pipelinerun`.
* [tkn pipelinerun logs](tkn_pipelinerun_logs.md)	 - Show logs for the  given `pipelinerun`.