package providers

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type slowP struct{ n atomic.Int32 }

func (p *slowP) Key() string               { return "slow" }
func (p *slowP) DefaultTTL() time.Duration { return time.Second }
func (p *slowP) Collect(context.Context) (any, error) {
	p.n.Add(1)
	time.Sleep(30 * time.Millisecond)
	return 7, nil
}
func TestSingleflightAndCache(t *testing.T) {
	r := New()
	p := &slowP{}
	r.Register(p)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, e := r.Get(context.Background(), "slow"); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	if p.n.Load() != 1 {
		t.Fatalf("collects=%d", p.n.Load())
	}
	if _, e := r.Get(context.Background(), "slow"); e != nil {
		t.Fatal(e)
	}
	if p.n.Load() != 1 {
		t.Fatal("cache miss")
	}
	if r.Stats().Shared == 0 {
		t.Fatal("expected shared calls")
	}
}
