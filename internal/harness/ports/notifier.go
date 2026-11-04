package ports

import "context"

// Delivery is one signed webhook attempt.
type Delivery struct {
	EventID string
	Attempt int
	Body    []byte
}

// Notifier delivers harness events to the chat surface. A nil error means the
// receiver accepted the event.
type Notifier interface {
	Deliver(ctx context.Context, delivery Delivery) error
}
