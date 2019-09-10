## tkn taskrun cancel

Cancel a taskrun in a namespace

### Usage

```
tkn taskrun cancel
```

### Synopsis

Cancel a taskrun in a namespace

### Examples


# Cancel the TaskRun named "foo" from the namespace "bar"
tkn taskrun cancel foo -n bar


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

* [tkn taskrun](tkn_taskrun.md)	 - Manage taskruns

