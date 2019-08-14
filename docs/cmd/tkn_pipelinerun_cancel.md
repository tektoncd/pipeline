## tkn pipelinerun cancel

Cancel the PipelineRun

### Usage

```
tkn pipelinerun cancel pipelinerunName
```

### Synopsis

Cancel the PipelineRun

### Examples


  # cancel the PipelineRun named "foo" from the namespace "bar"
    tkn pipelinerun cancel foo -n bar
   

### Options

```
  -h, --help   help for cancel
```

### Options inherited from parent commands

```
  -k, --kubeconfig string   kubectl config file (default: $HOME/.kube/config)
  -n, --namespace string    namespace to use (default: from $KUBECONFIG)
```

### SEE ALSO

* [tkn pipelinerun](tkn_pipelinerun.md)	 - Manage pipelineruns

