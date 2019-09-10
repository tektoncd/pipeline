## tkn pipelinerun list

Lists pipelineruns in a namespace

***Aliases**: ls*

### Usage

```
tkn pipelinerun list
```

### Synopsis

Lists pipelineruns in a namespace

### Examples


# List all PipelineRuns of Pipeline name 'foo'
tkn pipelinerun list foo -n bar

# List all pipelineruns in a namespaces 'foo'
tkn pr list -n foo


### Options

```
      --allow-missing-template-keys   If true, ignore any errors in templates when a field or map key is missing in the template. Only applies to golang and jsonpath output formats. (default true)
  -h, --help                          help for list
  -l, --limit int                     limit pipelineruns listed (default: return all pipelineruns)
  -o, --output string                 Output format. One of: json|yaml|name|go-template|go-template-file|template|templatefile|jsonpath|jsonpath-file.
      --template string               Template string or path to template file to use when -o=go-template, -o=go-template-file. The template format is golang templates [http://golang.org/pkg/text/template/#pkg-overview].
```

### Options inherited from parent commands

```
  -k, --kubeconfig string   kubectl config file (default: $HOME/.kube/config)
  -n, --namespace string    namespace to use (default: from $KUBECONFIG)
```

### SEE ALSO

* [tkn pipelinerun](tkn_pipelinerun.md)	 - Manage pipelineruns

