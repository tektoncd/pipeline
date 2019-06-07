## tkn taskrun logs

Lists the logs for the taskrun in a namespace

### Synopsis

Lists the logs for the taskrun in a namespace

```
tkn taskrun logs NAME [flags]
```

### Options

```
-a, --all                  show all logs including init steps injected by tekton
-h, --help                 help for logs
```

### Options inherited from parent commands

```
Flags:
--allow-missing-template-keys   If true, ignore any errors in templates when a field or map key is missing in the template. Only applies to golang and jsonpath output formats. (default true)
 -h, --help                          help for list
 -o, --output string                 Output format. One of: json|yaml|name|template|go-template|go-template-file|templatefile|jsonpath|jsonpath-file.
      --template string               Template string or path to template file to use when -o=go-template, -o=go-template-file. The template format is golang templates [http://golang.org/pkg/text/template/#pkg-overview].
```

### SEE ALSO

* [tkn taskrun](tkn_taskrun.md)	 - Parent command of the `taskrun` command group