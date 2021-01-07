# :zap: Zapdriver

Blazing fast, [Zap][zap]-based [Stackdriver][stackdriver] logging.

[zap]: https://github.com/uber-go/zap
[stackdriver]: https://cloud.google.com/stackdriver/

## Usage

This package provides three building blocks to support the full array of
structured logging capabilities of Stackdriver:

* [Special purpose logging fields](#special-purpose-logging-fields)
* [Pre-configured Stackdriver-optimized encoder](#pre-configured-stackdriver-optimized-encoder)
* [Custom Stackdriver Zap core](#custom-stackdriver-zap-core)
* [Using Error Reporting](#using-error-reporting)

The above components can be used separately, but to start, you can create a new
Zap logger with all of the above included:

```golang
logger, err := zapdriver.NewProduction() // with sampling
logger, err := zapdriver.NewDevelopment() // with `development` set to `true`
```

The above functions give back a pointer to a `zap.Logger` object, so you can use
[Zap][zap] like you've always done, except that it now logs in the proper
[Stackdriver][stackdriver] format.

You can also create a configuration struct, and build your logger from there:

```golang
config := zapdriver.NewProductionConfig()
config := zapdriver.NewDevelopmentConfig()
```

Or, get the Zapdriver encoder, and build your own configuration struct from
that:

```golang
encoder := zapdriver.NewProductionEncoderConfig()
encoder := zapdriver.NewDevelopmentEncoderConfig()
```

Read on to learn more about the available Stackdriver-specific log fields, and
how to use the above-mentioned components.

### Special purpose logging fields

You can use the following fields to add extra information to your log entries.
These fields are parsed by Stackdriver to make it easier to query your logs or
to use the log details in the Stackdriver monitoring interface.

* [`HTTP`](#http)
* [`Label`](#label)
* [`SourceLocation`](#sourcelocation)
* [`Operation`](#operation)
* [`TraceContext`](#tracecontext)

#### HTTP

You can log HTTP request/response cycles using the following field:

```golang
HTTP(req *HTTPPayload) zap.Field
```

You can either manually build the request payload:

```golang
req := &HTTPPayload{
  RequestMethod: "GET",
  RequestURL: "/",
  Status: 200,
}
```

Or, you can auto generate the struct, based on the available request and
response objects:

```golang
NewHTTP(req *http.Request, res *http.Response) *HTTPPayload
```

You are free to pass in `nil` for either the request or response object, if one
of them is unavailable to you at the point of logging. Any field depending on
one or the other will be omitted if `nil` is passed in.

Note that there are some fields that are not populated by either the request or
response object, and need to be set manually:

* `ServerIP string`
* `Latency string`
* `CacheLookup bool`
* `CacheHit bool`
* `CacheValidatedWithOriginServer bool`
* `CacheFillBytes string`

If you have no need for those fields, the quickest way to get started is like
so:

```golang
logger.Info("Request Received.", zapdriver.HTTP(zapdriver.NewHTTP(req, res)))
```

#### Label

You can add a "label" to your payload as follows:

```golang
Label(key, value string) zap.Field
```

Note that underwater, this sets the key to `labels.<key>`. You need to be using
the `zapdriver.Core` core for this to be converted to the proper format for
Stackdriver to recognize the labels.

See "Custom Stackdriver Zap core" for more details.

If you have a reason not to use the provided Core, you can still wrap labels in
the right `labels` namespace by using the available function:

```golang
Labels(fields ...zap.Field) zap.Field
```

Like so:

```golang
logger.Info(
  "Did something.",
  zapdriver.Labels(
    zapdriver.Label("hello", "world"),
    zapdriver.Label("hi", "universe"),
  ),
)
```

Again, wrapping the `Label` calls in `Labels` is not required if you use the
supplied Zap Core.

#### SourceLocation

You can add a source code location to your log lines to be picked up by
Stackdriver.

Note that you can set this manually, or use `zapdriver.Core` to automatically
add this. If you set it manually, _and_ use `zapdriver.Core`, the manual call
stack will be preserved over the automated one.

```golang
SourceLocation(pc uintptr, file string, line int, ok bool) zap.Field
```

Note that the function signature equals that of the return values of
`runtime.Caller()`. This allows you to catch the stack frame at one location,
while logging it at a different location, like so:

```golang
pc, file, line, ok := runtime.Caller(0)

// do other stuff...

logger.Error("Something happened!", zapdriver.SourceLocation(pc, file, line, ok))
```

If you use `zapdriver.Core`, the above use-case is the only use-case where you
would want to manually set the source location. In all other situations, you can
simply omit this field, and it will be added automatically, using the stack
frame at the location where the log line is triggered.

If you don't use `zapdriver.Core`, and still want to add the source location at
the frame of the triggered log line, you'd do it like this:

```golang
logger.Error("Something happened!", zapdriver.SourceLocation(runtime.Caller(0)))
```

#### Operation

The `Operation` log field allows you to group log lines into a single
"operation" performed by the application:

```golang
Operation(id, producer string, first, last bool) zap.Field
```

For a pair of logs that belong to the same operation, you should use the same
`id` between them. The `producer` is an arbitrary identifier that should be
globally unique amongst all the logs of all your applications (meaning it should
probably be the unique name of the current application). You should set `first`
to true for the first log in the operation, and `last` to true for the final log
of the operation.

```golang
logger.Info("Started.", zapdriver.Operation("3g4d3g", "my-app", true, false))
logger.Debug("Progressing.", zapdriver.Operation("3g4d3g", "my-app", false, false))
logger.Info("Done.", zapdriver.Operation("3g4d3g", "my-app", false, true))
```

Instead of defining the "start" and "end" booleans, you can also use these three
convenience functions:

```golang
OperationStart(id, producer string) zap.Field
OperationCont(id, producer string) zap.Field
OperationEnd(id, producer string) zap.Field
```

#### TraceContext

You can add trace context information to your log lines to be picked up by
Stackdriver.

```golang
TraceContext(trace string, spanId string, sampled bool, projectName string) []zap.Field
```

Like so:

```golang
logger.Error("Something happened!", zapdriver.TraceContext("105445aa7843bc8bf206b120001000", "0", true, "my-project-name")...)
```

### Pre-configured Stackdriver-optimized encoder

The Stackdriver encoder maps all Zap log levels to the appropriate
[Stackdriver-supported levels][levels]:

> DEBUG     (100) Debug or trace information.
>
> INFO      (200) Routine information, such as ongoing status or performance.
>
> WARNING   (400) Warning events might cause problems.
>
> ERROR     (500) Error events are likely to cause problems.
>
> CRITICAL  (600) Critical events cause more severe problems or outages.
>
> ALERT     (700) A person must take an action immediately.
>
> EMERGENCY (800) One or more systems are unusable.

[levels]: https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry#LogSeverity

It also sets some of the default keys to use [the right names][names], such as
`timestamp`, `severity`, and `message`.

[names]: https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry

You can use this encoder if you want to build your Zap logger configuration
manually:

```golang
zapdriver.NewProductionEncoderConfig()
```

For parity-sake, there's also `zapdriver.NewDevelopmentEncoderConfig()`, but it
returns the exact same encoder right now.

### Custom Stackdriver Zap core

A custom Zap core is included in this package to support some special use-cases.

First of all, if you use `zapdriver.NewProduction()` (or `NewDevelopment`) , you
already have this core enabled, so everything _just works_ ™.

There are two use-cases which require this core:

1. If you use `zapdriver.Label("hello", "world")`, it will initially end up in
   your log with the key `labels.hello` and value `world`. Now if you have two
   labels, you could also have `labels.hi` with value `universe`. This works as-
   is, but for this to be correctly parsed by Stackdriver as true "labels", you
   need to use the Zapdriver core, so that both of these fields get rewritten,
   to use the namespace `labels`, and use the keys `hello` and `hi` within that
   namespace. This is done automatically.

2. If you don't want to use `zapdriver.SourceLocation()` on every log call, you
   can use this core for the source location to be automatically added to
   each log entry.

When building a logger, you can inject the Zapdriver core as follows:

```golang
config := &zap.Config{}
logger, err := config.Build(zapdriver.WrapCore())
```

### Using Error Reporting

To report errors using StackDriver's Error Reporting tool, a log line needs to follow a separate log format described in the [Error Reporting][errorreporting] documentation.

[errorreporting]: https://cloud.google.com/error-reporting/docs/formatting-error-messages

The simplest way to do this is by using `NewProductionWithCore`:

```golang
logger, err := zapdriver.NewProductionWithCore(zapdriver.WrapCore(
  zapdriver.ReportAllErrors(true),
  zapdriver.ServiceName("my service"),
))
```

For parity-sake, there's also `zapdriver.NewDevelopmentWithCore()`

If you are building a custom logger, you can use `WrapCore()` to configure the driver core:

```golang
config := &zap.Config{}
logger, err := config.Build(zapdriver.WrapCore(
  zapdriver.ReportAllErrors(true),
  zapdriver.ServiceName("my service"),
))
```

Configuring this way, every error log entry will be reported to Stackdriver's Error Reporting tool.

#### Reporting errors manually

If you do not want every error to be reported, you can attach `ErrorReport()` to log call manually:

```golang
logger.Error("An error to be reported!", zapdriver.ErrorReport(runtime.Caller(0)))
// Or get Caller details
pc, file, line, ok := runtime.Caller(0)
// do other stuff... and log elsewhere
logger.Error("Another error to be reported!", zapdriver.ErrorReport(pc, file, line, ok))
```

Please keep in mind that ErrorReport needs a ServiceContext attached to the log
entry. If you did not configure this using `WrapCore`, error reports will
get attached using service name as `unknown`. To prevent this from happeneing,
either configure your core or attach service context before (or when) using
the logger:

```golang
logger.Error(
  "An error to be reported!",
  zapdriver.ErrorReport(runtime.Caller(0)),
  zapdriver.ServiceContext("my service"),
)

// Or permanently attach it to your logger
logger = logger.With(zapdriver.ServiceContext("my service"))
// and then use it
logger.Error("An error to be reported!", zapdriver.ErrorReport(runtime.Caller(0)))
```
