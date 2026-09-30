package providers

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type fake struct{ n atomic.Int32 }

func (f *fake) Key() string                          { return "x" }
func (f *fake) DefaultTTL() time.Duration            { return time.Minute }
func (f *fake) Collect(context.Context) (any, error) { f.n.Add(1); return 1, nil }
func TestCache(t *testing.T) {
	r := New()
	f := &fake{}
	r.Register(f)
	_, _ = r.Get(context.Background(), "x")
	_, _ = r.Get(context.Background(), "x")
	if f.n.Load() != 1 {
		t.Fatalf("collect count=%d", f.n.Load())
	}
}
