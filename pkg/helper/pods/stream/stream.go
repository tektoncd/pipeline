package stream

import (
	"io"

	corev1 "k8s.io/api/core/v1"
	typedv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// Streamer provides Stream method
type Streamer interface {
	Stream() (io.ReadCloser, error)
}

// NewStreamerFunc must return and Streamer given the pod details
type NewStreamerFunc func(p typedv1.PodInterface, name string, o *corev1.PodLogOptions) Streamer
