# Kubernetes Custom Resource API Reference Docs generator

If you have a project that is Custom Resource Definitions and wanted to generate
API Reference Docs [like this][ar] this tool is for you.

[ar]: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.13/

## Current Users

- [**Knative** API reference docs](https://knative.dev/docs/reference/api/serving-api/)
- [**FluxCD** API reference docs](https://fluxcd.io/docs/components/source/api/)
- [**Argo CD** operator API reference docs](https://argocd-operator.readthedocs.io/en/latest/reference/api.html/)
- [**Contour** ingress controller API reference docs](https://projectcontour.io/docs/v1.19.0/config/api/)
- [**Kubeflow** API reference docs](https://www.kubeflow.org/docs/reference/overview/)
- [**cert-manager** API reference docs](https://cert-manager.io/docs/reference/api-docs/)
- [**Open Service Mesh** API reference docs](https://release-v0-11.docs.openservicemesh.io/docs/api_reference/config/v1alpha1/)
- [**PlanetScale Vitess Operator** API reference docs](https://github.com/planetscale/vitess-operator/blob/main/docs/api.md)
- [**Agones** API reference docs](https://agones.dev/site/docs/reference/agones_crd_api_reference/)
- [**Gardener** API reference docs](https://gardener.cloud/api-reference/)
- [**New Relic Alert Manager** API reference docs](https://github.com/fpetkovski/newrelic-alert-manager/tree/master/docs)
- [**Antrea** API reference docs](https://antrea.io/docs/v1.3.0/docs/api-reference/)
- [**kube-green** API reference docs](https://kube-green.dev/docs/apireference_v1alpha1/)
- _[[ADD YOUR PROJECT HERE]]_

## Why

Normally you would want to use the same [docs generator][dg] as [Kubernetes API
reference][ar], but here's why I wrote a different parser/generator:

1. Today, Kubernetes API [does not][pr] provide OpenAPI specs for CRDs (e.g.
   Knative), therefore the [gen-apidocs][ga]
   generator used by Kubernetes won't work.

2. Even when Kubernetes API starts providing OpenAPI specs for CRDs, your CRD
   must have a validation schema (e.g. Knative API doesn't!)

3. Kubernetes [gen-apidocs][ga] parser relies on running a `kube-apiserver` and
   calling `/apis` endpoint to get OpenAPI specs to generate docs. **This tool
   doesn't need that!**

[dg]: https://github.com/kubernetes-incubator/reference-docs/
[ga]: https://github.com/kubernetes-incubator/reference-docs/tree/master/gen-apidocs/generators
[pr]: https://github.com/kubernetes/kubernetes/pull/71192

## How

This is a custom API reference docs generator that uses the
[k8s.io/gengo](https://godoc.org/k8s.io/gengo) project to parse types and
generate API documentation from it.

Capabilities of this tool include:

- Doesn't depend on OpenAPI specs, or kube-apiserver, or a running cluster.
- Relies only on the Go source code (pkg/apis/**/*.go) to parse API types.
- Can link to other sites for external APIs. For example, if your types have a
  reference to Kubernetes core/v1.PodSpec, you can link to it.
- [Configurable](./example-config.json) settings to hide certain fields or types
  entirely from the generated output.
- Either output to a file or start a live http-server (for rapid iteration).
- Supports markdown rendering from godoc type, package and field comments.

## Try it out

1. Clone this repository.

2. Make sure you have go1.11+ installed. Then run `go build`, you should get a
   `gen-crd-api-reference-docs` binary executable in the current directory.

3. Clone a Knative repository, set GOPATH correctly,
   and call the compiled binary within that directory.

    ```sh
    # go into a repository root with GOPATH set. (I use my own script
    # goclone(1) to have a separate GOPATH for each repo I clone.)
    $ goclone knative/build

    $ /path/to/gen-crd-api-reference-docs \
        -config "/path/to/example-config.json" \
        -api-dir "github.com/knative/build/pkg/apis/build/v1alpha1" \
        -out-file docs.html
    ```

4. Visit `docs.html` to view the results.

-----

This is not an official Google project. See [LICENSE](./LICENSE).
