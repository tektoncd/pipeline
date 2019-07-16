package kncloudevents

import (
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
)

func NewDefaultClient(target ...string) (client.Client, error) {
	tOpts := []http.Option{http.WithBinaryEncoding()}
	if len(target) > 0 && target[0] != "" {
		tOpts = append(tOpts, http.WithTarget(target[0]))
	}

	// Make an http transport for the CloudEvents client.
	t, err := http.New(tOpts...)
	if err != nil {
		return nil, err
	}
	// Use the transport to make a new CloudEvents client.
	c, err := client.New(t,
		client.WithUUIDs(),
		client.WithTimeNow(),
	)
	if err != nil {
		return nil, err
	}
	return c, nil
}
