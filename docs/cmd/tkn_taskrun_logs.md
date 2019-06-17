## tkn taskrun logs

Lists the logs for the taskrun in a namespace

### Synopsis

Lists the logs for the taskrun in a namespace

```
tkn taskrun logs NAME [flags]
```

### Options

```
  -a, --all      show all logs including init steps injected by tekton
  -f, --follow   stream the live logs
  -h, --help     help for logs

```

### Options inherited from parent commands

```
Flags:
  -k, --kubeconfig string                        kubectl config file (default: $HOME/.kube/config)
  -n, --namespace string                         namespace to use (default: from $KUBECONFIG)
```

### SEE ALSO

* [tkn taskrun](tkn_taskrun.md)	 - Parent command of the `taskrun` command group