package ports

import "context"

type Delivery struct {
	EventID string
	Attempt int
	Body    []byte
}

type Notifier interface {
	Deliver(ctx context.Context, delivery Delivery) error
}
