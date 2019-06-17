## tkn pipelinerun logs

Lists the logs for the `pipelinerun` in a namespace

### Synopsis

Lists the logs for the `pipelinerun` in a namespace

```
tkn pipelinerun logs NAME [flags]
```

### Options

```
-a, --all                  show all logs including init steps injected by tekton
-h, --help                 help for logs
-f  --follow               stream the live logs 
-t, --only-tasks strings   show the logs for mentioned task only
```

### Options inherited from parent commands

```
--azure-container-registry-config string   Path to the file containing Azure container registry configuration information.
 -k, --kubeconfig string                        kubectl config file (default: $HOME/.kube/config)
 -n, --namespace string                         namespace to use (default: from $KUBECONFIG)
```

### SEE ALSO

* [tkn pipelinerun](tkn_pipelinerun.md)	 - Parent command of the `pipelinerun` command group