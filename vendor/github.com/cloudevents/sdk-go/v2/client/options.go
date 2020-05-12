package client

import (
	"fmt"
	"github.com/cloudevents/sdk-go/v2/binding"
)

// Option is the function signature required to be considered an client.Option.
type Option func(interface{}) error

// WithEventDefaulter adds an event defaulter to the end of the defaulter chain.
func WithEventDefaulter(fn EventDefaulter) Option {
	return func(i interface{}) error {
		if c, ok := i.(*ceClient); ok {
			if fn == nil {
				return fmt.Errorf("client option was given an nil event defaulter")
			}
			c.eventDefaulterFns = append(c.eventDefaulterFns, fn)
		}
		return nil
	}
}

func WithForceBinary() Option {
	return func(i interface{}) error {
		if c, ok := i.(*ceClient); ok {
			c.outboundContextDecorators = append(c.outboundContextDecorators, binding.WithForceBinary)
		}
		return nil
	}
}

func WithForceStructured() Option {
	return func(i interface{}) error {
		if c, ok := i.(*ceClient); ok {
			c.outboundContextDecorators = append(c.outboundContextDecorators, binding.WithForceStructured)
		}
		return nil
	}
}

// WithUUIDs adds DefaultIDToUUIDIfNotSet event defaulter to the end of the
// defaulter chain.
func WithUUIDs() Option {
	return func(i interface{}) error {
		if c, ok := i.(*ceClient); ok {
			c.eventDefaulterFns = append(c.eventDefaulterFns, DefaultIDToUUIDIfNotSet)
		}
		return nil
	}
}

// WithTimeNow adds DefaultTimeToNowIfNotSet event defaulter to the end of the
// defaulter chain.
func WithTimeNow() Option {
	return func(i interface{}) error {
		if c, ok := i.(*ceClient); ok {
			c.eventDefaulterFns = append(c.eventDefaulterFns, DefaultTimeToNowIfNotSet)
		}
		return nil
	}
}

// WithTracePropagation enables trace propagation via the distributed tracing
// extension.
func WithTracePropagation() Option {
	return func(i interface{}) error {
		if c, ok := i.(*obsClient); ok {
			c.addTracing = true
		}
		return nil
	}
}

// WithPollGoroutines configures how much goroutines should be used to
// poll the Receiver/Responder/Protocol implementations.
// Default value is GOMAXPROCS
func WithPollGoroutines(pollGoroutines int) Option {
	return func(i interface{}) error {
		if c, ok := i.(*ceClient); ok {
			c.pollGoroutines = pollGoroutines
		}
		return nil
	}
}
