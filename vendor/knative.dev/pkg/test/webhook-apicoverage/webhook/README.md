# Webhook

Webhook based API-Coverage tool uses
[ValidatingAdmissionWebhook](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#validatingadmissionwebhook)
which is a web-server that the K8 API-Server calls into for every API-Object
update to verify if the object is valid before storing it into its datastore.
Each validation request has the json representation of the object being
created/modified, that the tool uses to capture coverage data. `webhook` package
inside this folder provides a mechanism for individual repos to setup
ValidatingAdmissionWebhook.

[APICoverageWebhook](webhook.go) type inside the package encapsulates necessary
configuration details and helper methods required to setup the webhook. Each
repo is expected to call into `SetupWebhook()` providing following three
parameters:

1. `http.Handler`: This is the http handler (that implements
   `ServeHTTP( w http.ResponseWriter, r *http.Request)`) that the web server
   created by APICoverageWebhook uses.
1. `rules`: This is an array of `RuleWithOperations` objects from the
   `k8s.io/api/admissionregistration/v1beta1` package that the webhook uses for
   validation on each API Object update. e.g: knative-serving while calling this
   method would provide rules that will handle API Objects like `Service`,
   `Configuration`, `Route` and `Revision`.
1. `namespace`: Namespace name where the webhook would be installed.
1. `stop` channel: Channel to terminate webhook's web server.

`SetupWebhook()` method in its implementation creates a TLS based web server and
registers the webhook by creating a ValidatingWebhookConfiguration object inside
the K8 cluster.

[APICoverageRecorder](apicoverage_recorder.go) type inside the package
encapsulates the apicoverage recording capabilities. Repo using this type is
expected to set:

1. `ResourceForest`: Specifying the version and initializing the
   [ResourceTrees](../resourcetree/resourcetree.go)
1. `ResourceMap`: Identifying the resources whose APICoverage needs to be
   calculated.
1. `NodeRules`: [NodeRules](../resourcetree/rule.go) that are applicable for the
   repo.
1. `FieldRules`: [FieldRules](../resourcetree/rule.go) that are applicable for
   the repo.
1. `DisplayRules`: [DisplayRules](../view/rule.go) to be used by
   `GetResourceCoverage` method.
