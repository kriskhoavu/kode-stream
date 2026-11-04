package scheduler

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestOrchestratorRunsWakesRecordsAndStops(t *testing.T) {
	var runs atomic.Int32
	wake := make(chan struct{}, 1)
	ran := make(chan struct{}, 10)
	orchestrator := New(nil,
		Loop{Name: "work", Interval: func() time.Duration { return time.Hour }, Wake: wake, Run: func(context.Context) error {
			runs.Add(1)
			ran <- struct{}{}
			return errors.New("boom")
		}},
		Loop{Name: "panics", Interval: func() time.Duration { return time.Hour }, Run: func(context.Context) error { panic("bad") }},
	)
	orchestrator.Start(context.Background())
	orchestrator.Start(context.Background())
	<-ran
	wake <- struct{}{}
	<-ran
	if runs.Load() != 2 {
		t.Fatalf("runs = %d", runs.Load())
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		health, ok := orchestrator.Health("panics")
		if ok && health.LastError != "" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("panic was not recorded")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if health, _ := orchestrator.Health("work"); health.LastError != "boom" || health.LastRunAt == nil {
		t.Fatalf("health = %#v", health)
	}
	if !orchestrator.Stop(time.Second) || orchestrator.Running() {
		t.Fatal("orchestrator did not stop")
	}
	if !orchestrator.Stop(time.Second) {
		t.Fatal("second stop failed")
	}
}
