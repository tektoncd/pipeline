# Tools

Tools package is intended to contain types and public helper methods that
provide utilities to solves common requirements across repos. It currently
contains following helper methods:

1. `GetDefaultKubePath`: Helper method to retrieve the path for Kubeconfig
   inside users home directory.
1. `GetWebhookServiceIP`: Helper method to retrieve the public IP address for
   the webhook service. The service is setup as part of the apicoverage-webhook
   setup.
1. `GetResourceCoverage`: Helper method to retrieve Coverage data for a resource
   passed as parameter. The coverage data is retrieved from the API that is
   exposed by the HTTP server in [Webhook Setup](../webhook/webhook.go)
1. `GetAndWriteResourceCoverage`: Helper method that uses `GetResourceCoverage`
   to retrieve resource coverage and writes output to a file.
1. `GetTotalCoverage`: Helper method to retrieve total coverage data for a repo.
   The coverage data is retrieved from the API that is exposed by the HTTP
   server in [Webhook Setup](../webhook/webhook.go)
1. `GetAndWriteTotalCoverage`: Helper method that uses `GetTotalCoverage` to
   retrieve total coverage and writes output to a file.
