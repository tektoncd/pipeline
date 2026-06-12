![](https://github.com/elastic/crd-ref-docs/workflows/Build/badge.svg)

# CRD Reference Documentation Generator

Generates API reference documentation by scanning a source tree for exported CRD types.

This is a fresh implementation inspired by the https://github.com/ahmetb/gen-crd-api-reference-docs project. While trying to adopt the `gen-crd-api-refernce-docs` to generate documentation for [Elastic Cloud on Kubernetes](https://github.com/elastic/cloud-on-k8s), we encountered a few shortcomings such as the lack of support for Go modules, slow scan times, and rendering logic that was hard to adapt to Asciidoc (our preferred documentation markup language). This project attempts to address those issues by re-implementing the type discovery logic and decoupling the rendering logic so that different markup formats can be supported.

## Usage

Pre-built Linux binaries can be downloaded from the Github Releases tab. Alternatively, you can download and build the source with Go tooling:

```
go get -u github.com/elastic/crd-ref-docs
```

The tool can be invoked as follows to generate documentation:

```
crd-ref-docs \
    --source-path=$GOPATH/src/github.com/elastic/cloud-on-k8s/pkg/apis \
    --config=config.yaml
```

By default, documentation is rendered in Asciidoc format. In order to generate documentation in Markdown format, you will have to specify the `markdown` renderer:

```
crd-ref-docs \
    --source-path=$GOPATH/src/github.com/elastic/cloud-on-k8s/pkg/apis \
    --config=config.yaml \
    --renderer=markdown
```

Default templates are embedded in the binary. You may provide your own templates by specifying the templates directory:

```
crd-ref-docs \
    --source-path=$GOPATH/src/github.com/elastic/cloud-on-k8s/pkg/apis \
    --config=config.yaml \
    --renderer=asciidoctor \
    --templates-dir=templates/asciidoctor
```

Default output mode writes all data to a single output file. 
You can choose between single mode and group mode by specifying the output mode. 
In group mode, separate files are created for each API group, ensuring that the specified output path is an existing directory.
```
crd-ref-docs \
    --source-path=$GOPATH/src/github.com/elastic/cloud-on-k8s/pkg/apis \
    --config=config.yaml \
    --output-path=./docs \
    --output-mode=group
```

### Configuration

Configuration options such as types and fields to exclude from the documentation can be specified using a YAML file.

```yaml
processor:
  # RE2 regular expressions describing types that should be excluded from the generated documentation.
  ignoreTypes:
    - "(Elasticsearch|Kibana|ApmServer)List$"
    - "(Elasticsearch|Kibana|ApmServer)Health$"
    - "(Elasticsearch|Kibana|ApmServer|Reconciler)Status$"
    - "ElasticsearchSettings$"
    - "Associa(ted|tor|tionStatus|tionConf)$"
  # RE2 regular expressions describing type fields that should be excluded from the generated documentation.
  ignoreFields:
    - "status$"
    - "TypeMeta$"

render:
  # Version of Kubernetes to use when generating links to Kubernetes API documentation.
  kubernetesVersion: 1.22
  # Generate better link for known types
  knownTypes:
    - name: SecretObjectReference
      package: sigs.k8s.io/gateway-api/apis/v1beta1
      link: https://gateway-api.sigs.k8s.io/references/spec/#gateway.networking.k8s.io/v1beta1.SecretObjectReference
```

### Advanced Features

#### Custom Markers

You can add custom markers to your CRD types to provide additional information in the generated documentation.
For example, you can add a `hidefromdoc` marker to indicate that a type is hide from the documentation.

```yaml
processor:
 ignoreGroupVersions:
   - "GVK"
 ignoreTypes:
   - "Embedded[2-4]$"
 ignoreFields:
   - "status$"
   - "TypeMeta$"
 customMarkers:
   - name: "hidefromdoc"
     target: field

render:
 kubernetesVersion: 1.25
 knownTypes:
   - name: SecretObjectReference
     package: sigs.k8s.io/gateway-api/apis/v1beta1
     link: https://gateway-api.sigs.k8s.io/references/spec/#gateway.networking.k8s.io/v1beta1.SecretObjectReference
```

You can then add the `hidefromdoc` marker to the field you want to hidden from the documentation. Markers can be added
to fields, types and packages. The `target` field in the configuration specifies the target of the marker (it can be either
`field`, `type` or `package`).

```go
type Embedded1 struct {
	Embedded2 `json:",inline"`
	// +hidefromdoc
	E         string `json:"e,omitempty"`
	EmbeddedX `json:",inline"`
}
```

Then update the templates to render the custom markers. You can find an example [here](./test/templates/markdown/type.tpl).
