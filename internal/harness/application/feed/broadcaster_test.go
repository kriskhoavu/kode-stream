package feed

import (
	"testing"

	"kode-stream/internal/harness/domain"
)

func TestBroadcasterDeliversAndDropsSlowSubscribers(t *testing.T) {
	broadcaster := NewBroadcaster()
	fast := broadcaster.Subscribe(4)
	slow := broadcaster.Subscribe(1)
	broadcaster.Publish(domain.Event{Seq: 1})
	broadcaster.Publish(domain.Event{Seq: 2})
	if got := (<-fast.Events).Seq; got != 1 {
		t.Fatalf("fast first = %d", got)
	}
	if got := (<-fast.Events).Seq; got != 2 {
		t.Fatalf("fast second = %d", got)
	}
	select {
	case <-slow.Behind:
	default:
		t.Fatal("slow subscriber was not marked behind")
	}
	if broadcaster.Subscribers() != 1 {
		t.Fatalf("subscribers = %d, want 1", broadcaster.Subscribers())
	}
	fast.Cancel()
	fast.Cancel()
	if broadcaster.Subscribers() != 0 {
		t.Fatalf("subscribers after cancel = %d", broadcaster.Subscribers())
	}
}
