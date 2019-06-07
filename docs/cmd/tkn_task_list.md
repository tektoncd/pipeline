## tkn task list

Lists the `tasks` in a namespace

### Synopsis

Lists the `tasks` in a namespace

```
tkn task list NAME [flags]
```

### Aliases

```
list, ls
```

### Options

```
--allow-missing-template-keys   If true, ignores any errors in a template when a field or map key is missing. Only applies to golang and jsonpath output formats. (default true)
 -h, --help                          help for list
 -o, --output string                 Output format. One of: json|yaml|name|template|go-template|go-template-file|templatefile|jsonpath|jsonpath-file.
      --template string               Template string or path to template file to use when -o=go-template, -o=go-template-file. The template format is golang templates [http://golang.org/pkg/text/template/#pkg-overview].
```

### Options inherited from parent commands

```
--azure-container-registry-config string   Path to the file containing Azure container registry configuration information.
  -k, --kubeconfig string                        kubectl config file (default: $HOME/.kube/config)
  -n, --namespace string                         namespace to use (default: from $KUBECONFIG)
```

### SEE ALSO

* [tkn task](tkn_task.md)	 - Parent command of the `task` command group