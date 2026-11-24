package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Loop struct {
	Name string
	// Interval is read before every wait, so a settings change applies on the next cycle.
	Interval func() time.Duration
	Wake     <-chan struct{}
	Run      func(ctx context.Context) error
}

type Health struct {
	LastRunAt *time.Time
	LastError string
}

type Orchestrator struct {
	loops []Loop
	now   func() time.Time

	mu      sync.Mutex
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	health  map[string]Health
	running bool
}

func New(now func() time.Time, loops ...Loop) *Orchestrator {
	if now == nil {
		now = time.Now
	}
	return &Orchestrator{loops: loops, now: now, health: map[string]Health{}}
}

func (o *Orchestrator) Start(ctx context.Context) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.running {
		return
	}
	ctx, o.cancel = context.WithCancel(ctx)
	o.running = true
	for _, loop := range o.loops {
		o.wg.Add(1)
		go o.run(ctx, loop)
	}
}

func (o *Orchestrator) run(ctx context.Context, loop Loop) {
	defer o.wg.Done()
	for {
		o.cycle(ctx, loop)
		interval := time.Minute
		if loop.Interval != nil {
			if value := loop.Interval(); value > 0 {
				interval = value
			}
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-loop.Wake:
			timer.Stop()
		case <-timer.C:
		}
	}
}

// A panic is recorded as an error, so one bad pass never takes the server down.
func (o *Orchestrator) cycle(ctx context.Context, loop Loop) {
	var err error
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("%s loop panicked: %v", loop.Name, recovered)
			}
		}()
		err = loop.Run(ctx)
	}()
	if ctx.Err() != nil {
		return
	}
	now := o.now().UTC()
	health := Health{LastRunAt: &now}
	if err != nil {
		health.LastError = err.Error()
	}
	o.mu.Lock()
	o.health[loop.Name] = health
	o.mu.Unlock()
}

func (o *Orchestrator) Stop(timeout time.Duration) bool {
	o.mu.Lock()
	if !o.running {
		o.mu.Unlock()
		return true
	}
	o.running = false
	cancel := o.cancel
	o.mu.Unlock()
	cancel()
	done := make(chan struct{})
	go func() {
		o.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

func (o *Orchestrator) Running() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.running
}

func (o *Orchestrator) Health(name string) (Health, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	health, ok := o.health[name]
	return health, ok
}
