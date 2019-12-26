# pullrequest-init

pullrequest-init fetches pull request data from the given URL and places it in
the provided path.

This binary outputs a generic pull request object into a set of generic files, as well as
provider specific payloads.

Currently supported providers:

*   GitHub

## Generic pull request payload

For information about the payloads written to disk, see the [resource documentation](../../docs/resources.md#pull-request-resource).
