## tkn pipeline logs

Show pipeline logs

### Usage

```
tkn pipeline logs
```

### Synopsis

Show pipeline logs

### Examples


  # show logs interactively for no inputs
    tkn pipeline logs -n namespace

  # show logs interactively for given pipeline
    tkn pipeline logs pipeline_name -n namespace

  # show logs of given pipeline for last run
    tkn pipeline logs pipeline_name -n namespace --last

  # show logs for given pipeline and pipelinerun
    tkn pipeline logs pipeline_name pipelinerun_name -n namespace
  
   

### Options

```
  -a, --all      show all logs including init steps injected by tekton
  -f, --follow   stream live logs
  -h, --help     help for logs
  -l, --last     show logs for last run
```

### Options inherited from parent commands

```
  -k, --kubeconfig string   kubectl config file (default: $HOME/.kube/config)
  -n, --namespace string    namespace to use (default: from $KUBECONFIG)
```

### SEE ALSO

* [tkn pipeline](tkn_pipeline.md)	 - Manage pipelines

