## tkn pipeline delete

Delete a pipeline in a namespace

***Aliases**: rm*

### Usage

```
tkn pipeline delete
```

### Synopsis

Delete a pipeline in a namespace

### Examples


# Delete a Pipeline of name 'foo' in namespace 'bar'
tkn pipeline delete foo -n bar

tkn p rm foo -n bar


### Options

```
  -a, --all                           Whether to delete related resources (pipelineruns) (default: false)
      --allow-missing-template-keys   If true, ignore any errors in templates when a field or map key is missing in the template. Only applies to golang and jsonpath output formats. (default true)
  -f, --force                         Whether to force deletion (default: false)
  -h, --help                          help for delete
  -o, --output string                 Output format. One of: json|yaml|name|go-template|go-template-file|template|templatefile|jsonpath|jsonpath-file.
      --template string               Template string or path to template file to use when -o=go-template, -o=go-template-file. The template format is golang templates [http://golang.org/pkg/text/template/#pkg-overview].
```

### Options inherited from parent commands

```
  -k, --kubeconfig string   kubectl config file (default: $HOME/.kube/config)
  -n, --namespace string    namespace to use (default: from $KUBECONFIG)
  -C, --nocolour            disable colouring (default: false)
```

### SEE ALSO

* [tkn pipeline](tkn_pipeline.md)	 - Manage pipelines

