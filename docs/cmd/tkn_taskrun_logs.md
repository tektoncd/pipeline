## tkn taskrun logs

Show taskruns logs

### Synopsis

Show taskruns logs

```
tkn taskrun logs
```

### Examples

```

# show the logs of TaskRun named "foo" from the namespace "bar"
tkn taskrun logs foo -n bar

# show the live logs of TaskRun named "foo" from the namespace "bar" 
tkn taskrun logs -f foo -n bar

```

### Options

```
  -a, --all      show all logs including init steps injected by tekton
  -f, --follow   stream live logs
  -h, --help     help for logs
```

### Options inherited from parent commands

```
  -k, --kubeconfig string   kubectl config file (default: $HOME/.kube/config)
  -n, --namespace string    namespace to use (default: from $KUBECONFIG)
```

### SEE ALSO

* [tkn taskrun](tkn_taskrun.md)	 - Manage taskruns

