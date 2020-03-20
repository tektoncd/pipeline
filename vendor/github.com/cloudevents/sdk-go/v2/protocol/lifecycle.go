package protocol

import (
	"context"
)

// Opener is the common interface for things that need to be opened.
type Opener interface {
	// Blocking call. Context is used to cancel.
	OpenInbound(ctx context.Context) error
}

// Closer is the common interface for things that can be closed.
type Closer interface {
	Close(ctx context.Context) error
}
