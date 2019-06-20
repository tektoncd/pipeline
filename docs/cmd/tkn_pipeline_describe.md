## tkn pipeline describe

Describes the `pipeline`

### Synopsis

Describes the `pipeline`

```
tkn pipeline describe NAME [flags]
```

### Aliases

```
describe, desc
```

### Options

```
--allow-missing-template-keys   If true, ignores any errors in template when a field or map key is missing. Only applies to golang and jsonpath output formats. (default true)
 -h, --help                          help for describe
 -o, --output string                 Output format. One of: json|yaml|name|go-template|go-template-file|templatefile|template|jsonpath|jsonpath-file.
    --template string               Template string or path to template file to use when -o=go-template, -o=go-template-file. The template format is golang templates [http://golang.org/pkg/text/template/#pkg-overview].
```

### Options inherited from parent commands

```
--azure-container-registry-config string        Path to the file containing Azure container registry configuration information.
 -k, --kubeconfig string                        kubectl config file (default: $HOME/.kube/config)
 -n, --namespace string                         namespace to use (default: from $KUBECONFIG)
```

### SEE ALSO

* [tkn pipeline](tkn_pipeline.md)	 - Parent command of the `pipeline` command group
