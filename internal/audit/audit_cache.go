package audit

import (
	"context"
	"sync"
	"time"

	"kode-stream/internal/common/models"
)

type CachedEventReader struct {
	source        Repository
	ttl           time.Duration
	now           func() time.Time
	mu            sync.Mutex
	items         map[Query]cachedEvents
	hits          int
	misses        int
	invalidations int
}

type cachedEvents struct {
	expiresAt time.Time
	events    []models.AuditEvent
}

type CacheStats struct {
	Hits          int
	Misses        int
	Invalidations int
}

func NewCachedEventReader(source Repository, ttl time.Duration, now func() time.Time) *CachedEventReader {
	if now == nil {
		now = time.Now
	}
	return &CachedEventReader{source: source, ttl: ttl, now: now, items: make(map[Query]cachedEvents)}
}

func (r *CachedEventReader) RecentContext(ctx context.Context, limit int) ([]models.AuditEvent, error) {
	return r.QueryContext(ctx, Query{Limit: limit})
}

func (r *CachedEventReader) QueryContext(ctx context.Context, query Query) ([]models.AuditEvent, error) {
	if r == nil || r.source == nil {
		return []models.AuditEvent{}, nil
	}
	if r.ttl <= 0 {
		return r.source.QueryContext(ctx, query)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	now := r.now()
	r.mu.Lock()
	entry, ok := r.items[query]
	if ok && now.Before(entry.expiresAt) {
		r.hits++
		events := cloneEvents(entry.events)
		r.mu.Unlock()
		return events, nil
	}
	r.misses++
	r.mu.Unlock()

	events, err := r.source.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.items[query] = cachedEvents{expiresAt: now.Add(r.ttl), events: cloneEvents(events)}
	r.mu.Unlock()
	return events, nil
}

func (r *CachedEventReader) Invalidate() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = make(map[Query]cachedEvents)
	r.invalidations++
}

func (r *CachedEventReader) Stats() CacheStats {
	if r == nil {
		return CacheStats{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return CacheStats{Hits: r.hits, Misses: r.misses, Invalidations: r.invalidations}
}

func cloneEvents(events []models.AuditEvent) []models.AuditEvent {
	if events == nil {
		return nil
	}
	clone := make([]models.AuditEvent, len(events))
	copy(clone, events)
	return clone
}
