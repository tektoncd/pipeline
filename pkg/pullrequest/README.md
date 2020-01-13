# PullRequest Resource E2E Tests

This directory contains E2E tests for the Pull Request resource. The E2E test
can run in a variety of modes, talking to both fake and real SCM providers.

## Fake SCM

In the Fake SCM mode, all requests are redirected to a local HTTP server that
intercepts the request and feeds back fake data. `testdata/scm` contains this
data, which was derived from making actual requests to the GitHub API. This data
should be kept as close as possible to the upstream API response.

The fake server can be configured to look for authentication tokens in the
request. By default if a token is not provided via an environment variable, one
will be set arbitrarily. Providing a valid token is not required for the Fake
SCM mode.

In order for the fake SCM proxy to successfully intercept requests, URLs must
use `http`.

## Real SCM

```sh
$ go test . -proxy false
```

In the Real SCM mode, the E2E test will communicate with real SCM providers. In
order to support uploads, a valid authentication token must be provided.

The test will first look for an SCM-specific auth code, or default back to the
standard `AUTH_TOKEN` environment variable if one is not present. Currently the
valid SCM-specific auth codes are:

-   `GITHUB_TOKEN`
