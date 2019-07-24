package kncloudevents

import (
	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
)

func NewDefaultClient(target ...string) (cloudevents.Client, error) {
	tOpts := []http.Option{cloudevents.WithBinaryEncoding()}
	if len(target) > 0 && target[0] != "" {
		tOpts = append(tOpts, cloudevents.WithTarget(target[0]))
	}

	// Make an http transport for the CloudEvents client.
	t, err := cloudevents.NewHTTPTransport(tOpts...)
	if err != nil {
		return nil, err
	}

	// Use the transport to make a new CloudEvents client.
	c, err := cloudevents.NewClient(t,
		cloudevents.WithUUIDs(),
		cloudevents.WithTimeNow(),
	)

	if err != nil {
		return nil, err
	}
	return c, nil
}
