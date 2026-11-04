// Package feed fans harness events out to in-process subscribers: the
// dashboard's event streams and the notifier.
package feed

import (
	"sync"

	"kode-stream/internal/harness/domain"
)

// Subscription receives events until it is cancelled or falls behind. When a
// subscriber's buffer is full the broadcaster closes Behind and drops it; the
// subscriber then refetches its snapshot instead of missing events silently.
type Subscription struct {
	Events <-chan domain.Event
	Behind <-chan struct{}
	cancel func()
}

func (s *Subscription) Cancel() {
	if s != nil && s.cancel != nil {
		s.cancel()
	}
}

type subscriber struct {
	events chan domain.Event
	behind chan struct{}
	once   sync.Once
}

func (s *subscriber) drop() {
	s.once.Do(func() { close(s.behind) })
}

type Broadcaster struct {
	mu          sync.Mutex
	subscribers map[int]*subscriber
	next        int
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{subscribers: map[int]*subscriber{}}
}

func (b *Broadcaster) Subscribe(buffer int) *Subscription {
	if buffer <= 0 {
		buffer = 64
	}
	sub := &subscriber{events: make(chan domain.Event, buffer), behind: make(chan struct{})}
	b.mu.Lock()
	id := b.next
	b.next++
	b.subscribers[id] = sub
	b.mu.Unlock()
	return &Subscription{Events: sub.events, Behind: sub.behind, cancel: func() {
		b.mu.Lock()
		delete(b.subscribers, id)
		b.mu.Unlock()
		sub.drop()
	}}
}

// Publish never blocks: a subscriber that cannot take the event is dropped.
func (b *Broadcaster) Publish(event domain.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for id, sub := range b.subscribers {
		select {
		case sub.events <- event:
		default:
			delete(b.subscribers, id)
			sub.drop()
		}
	}
}

func (b *Broadcaster) Subscribers() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subscribers)
}
