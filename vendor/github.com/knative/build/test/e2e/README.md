# End-to-End testing

## Running the tests

To run all e2e tests:
```
go test -tags e2e ./test/e2e/... -count=1
```

`-count=1` is the idiomatic way to bypass test caching, so that tests will
always run.

To run a single e2e test:

```
go test -tags e2e ./test/e2e/... -count=1 -test.run=<regex>
```

## What the tests do

By default, tests use your current Kubernetes config to talk to your currently
configured cluster.

When they run tests ensure that a namespace named `build-tests` exists, then
starts running builds in it.
