# Knative CloudEvents SDK

This library produces CloudEvents in version 0.1 compatible form. To learn more
about CloudEvents, see the [Specification](https://github.com/cloudevents/spec).

There are two roles the SDK fulfills: the [producer](#producer) and the
[consumer](#consumer). The producer creates a cloud event in either
[Binary](#binary) or [Structured](#structured) request format. The producer
assembles and sends the event through an HTTP endpoint. The consumer will
inspect the incoming HTTP request and select the correct decode format.

This SDK should be wire-compatible with any other producer or consumer of the
supported versions of CloudEvents.

## Getting Started

CloudEvents acts as the envelope in which to send a custom object. Define a
CloudEvent type for the events you will be producing.

Example CloudEvent Type: `dev.knative.cloudevent.example`

Select a source to identify the originator of this CloudEvent. It should be a
valid URI which represents the subject which created the CloudEvent (cloud
bucket, git repo, etc).

Example CloudEvent Source: `https://github.com/knative/pkg#cloudevents-example`

And finally, create a struct that will be the data inside the CloudEvent,
example:

```go

type Example struct {
    Sequence int    `json:"id"`
    Message  string `json:"message"`
}

```

### Producer

The producer creates a new `cloudevent.Client,` and then sends 10 `Example`
events to `"http://localhost:8080"`.

```go

package main

import (
    "github.com/knative/pkg/cloudevents"
    "log"
)

type Example struct {
    Sequence int    `json:"id"`
    Message  string `json:"message"`
}

func main() {
    c := cloudevents.NewClient(
        "http://localhost:8080",
        cloudevents.Builder{
            Source:    "https://github.com/knative/pkg#cloudevents-example",
            EventType: "dev.knative.cloudevent.example",
        },
    )
    for i := 0; i < 10; i++ {
        data := Example{
            Message:  "hello, world!",
            Sequence: i,
        }
        if err := c.Send(data); err != nil {
            log.Printf("error sending: %v", err)
        }
    }
}

```

### Consumer

The consumer will listen for a post and then inspect the headers to understand
how to decode the request.

```go

package main

import (
    "context"
    "log"
    "net/http"
    "time"

    "github.com/knative/pkg/cloudevents"
)

type Example struct {
    Sequence int    `json:"id"`
    Message  string `json:"message"`
}

func handler(ctx context.Context, data *Example) {
    metadata := cloudevents.FromContext(ctx)
    log.Printf("[%s] %s %s: %d, %q", metadata.EventTime.Format(time.RFC3339), metadata.ContentType, metadata.Source, data.Sequence, data.Message)
}

func main() {
    log.Print("listening on port 8080")
    log.Fatal(http.ListenAndServe(":8080", cloudevents.Handler(handler)))
}

```

## Request Formats

### CloudEvents Version 0.1

#### Binary

This is default, but to leverage binary request format:

```go

    c := cloudevents.NewClient(
        "http://localhost:8080",
        cloudevents.Builder{
            Source:    "https://github.com/knative/pkg#cloudevents-example",
            EventType: "dev.knative.cloudevent.example",
            Encoding: cloudevents.BinaryV01,
        },
    )

```

#### Structured

To leverage structured request format:

```go

    c := cloudevents.NewClient(
        "http://localhost:8080",
        cloudevents.Builder{
            Source:    "https://github.com/knative/pkg#cloudevents-example",
            EventType: "dev.knative.cloudevent.example",
            Encoding: cloudevents.StructuredV01,
        },
    )

```
