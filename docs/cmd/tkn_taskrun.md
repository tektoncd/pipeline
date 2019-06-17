## tkn taskrun

Parent command of the `taskrun` command group

### Synopsis

Parent command of the `taskrun` command group

### Aliases

```
taskrun, tr, taskruns
```

### Available Commands

```
list        Lists taskruns in a namespace
logs        Displays taskrun logs
```

### Options

```
-h, --help                help for taskrun
-k, --kubeconfig string   kubectl config file (default: $HOME/.kube/config)
-n, --namespace string    namespace to use (default: from $KUBECONFIG)
```

### Options inherited from parent commands

```
--azure-container-registry-config string   Path to the file containing Azure container registry configuration information.
```

### SEE ALSO

* [tkn taskrun list](tkn_taskrun_list.md)	 - Lists all `taskruns` in a given namespace.
* [tkn taskrun logs](tkn_taskrun_logs.md)	 - Show logs for the given `taskrun`.