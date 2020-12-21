# How to use logstream

This is a guide to start using `logstream` in your e2e testing.

## Requirements

1. The `SYSTEM_NAMESPACE` environment variable must be configured. Many of the
   knative test scripts already define this, and in some places (e.g. serving)
   randomize it. However, to facilitate usage outside of CI, you should consider
   including a package like
   [this](https://github.com/knative/serving/blob/master/test/defaultsystem/system.go)
   and linking it like
   [this](https://github.com/knative/serving/blob/e797247322b5aa35001152d2a2715dbc20a86cc4/test/conformance.go#L20-L23)

2) Test resources must be named with
   [`test.ObjectNameForTest(t)`](https://github.com/knative/networking/blob/40ef99aa5db0d38730a89a1de7e5b28b8ef6eed5/vendor/knative.dev/pkg/test/helpers/name.go#L50)

3. At the start of your test add: `t.Cleanup(logstream.Start(t))`

With that, you will start getting logs from the processes in the system
namespace interleaved into your test output via `t.Log`.

## How it works

In Knative we use `zap.Logger` for all of our logging, and most of those loggers
(e.g. in the context of a reconcile) have been decorated with the "key" of the
resource being processed. `logstream` simply decodes these structured log
messages and when the `key` matches the naming prefix that `ObjectNameForTest`
uses, it includes it into the test's output.

## Integrating in Libraries.

When a shared component is set up and called from reconciliation, it may have
it's own logger. If that component is dealing with individual resources, it can
scope individual log statements to that resource by decorating the logger with
its key like so:

```
logger := logger.With(zap.String(logkey.Key, resourceKey))
```

Now, any log statements that the library prints through this logger will appear
in the logstream!

For an example of this pattern, see
[the knative/networking prober library](https://github.com/knative/networking/blob/master/pkg/status/status.go).
