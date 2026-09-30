package providers

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

type Result struct {
	Data        any       `json:"data"`
	CollectedAt time.Time `json:"collected_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	Stale       bool      `json:"stale"`
}
type Provider interface {
	Key() string
	DefaultTTL() time.Duration
	Collect(context.Context) (any, error)
}
type entry struct{ r Result }
type call struct {
	done chan struct{}
	r    Result
	err  error
}
type Stats struct {
	Hits     uint64 `json:"hits"`
	Misses   uint64 `json:"misses"`
	Shared   uint64 `json:"shared"`
	Errors   uint64 `json:"errors"`
	Entries  int    `json:"entries"`
	Inflight int    `json:"inflight"`
}
type Registry struct {
	mu                         sync.Mutex
	p                          map[string]Provider
	c                          map[string]entry
	f                          map[string]*call
	hits, misses, shared, errs atomic.Uint64
	sem                        chan struct{}
}

func New() *Registry {
	return &Registry{p: map[string]Provider{}, c: map[string]entry{}, f: map[string]*call{}, sem: make(chan struct{}, 8)}
}
func (r *Registry) Register(p Provider) { r.mu.Lock(); defer r.mu.Unlock(); r.p[p.Key()] = p }
func (r *Registry) Get(ctx context.Context, key string) (Result, error) {
	now := time.Now()
	r.mu.Lock()
	if e, ok := r.c[key]; ok && now.Before(e.r.ExpiresAt) {
		r.hits.Add(1)
		r.mu.Unlock()
		return e.r, nil
	}
	if c, ok := r.f[key]; ok {
		r.shared.Add(1)
		r.mu.Unlock()
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		case <-c.done:
			return c.r, c.err
		}
	}
	p := r.p[key]
	if p == nil {
		r.mu.Unlock()
		return Result{}, errors.New("provider not found")
	}
	c := &call{done: make(chan struct{})}
	r.f[key] = c
	r.misses.Add(1)
	r.mu.Unlock()
	select {
	case r.sem <- struct{}{}:
		defer func() { <-r.sem }()
	case <-ctx.Done():
		r.finish(key, c, Result{}, ctx.Err())
		return Result{}, ctx.Err()
	}
	d, err := p.Collect(ctx)
	now = time.Now()
	ttl := p.DefaultTTL()
	if ttl <= 0 {
		ttl = 5 * time.Second
	}
	if err != nil {
		r.errs.Add(1)
		ttl = time.Second
		res := Result{CollectedAt: now, ExpiresAt: now.Add(ttl), Stale: true}
		r.finish(key, c, res, err)
		return res, err
	}
	res := Result{Data: d, CollectedAt: now, ExpiresAt: now.Add(ttl)}
	r.mu.Lock()
	r.c[key] = entry{res}
	r.mu.Unlock()
	r.finish(key, c, res, nil)
	return res, nil
}
func (r *Registry) finish(key string, c *call, res Result, err error) {
	r.mu.Lock()
	c.r = res
	c.err = err
	delete(r.f, key)
	close(c.done)
	r.mu.Unlock()
}
func (r *Registry) Keys() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	o := make([]string, 0, len(r.p))
	for k := range r.p {
		o = append(o, k)
	}
	return o
}
func (r *Registry) Stats() Stats {
	r.mu.Lock()
	defer r.mu.Unlock()
	return Stats{Hits: r.hits.Load(), Misses: r.misses.Load(), Shared: r.shared.Load(), Errors: r.errs.Load(), Entries: len(r.c), Inflight: len(r.f)}
}
