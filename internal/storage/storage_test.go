package storage

import (
	"context"
	"testing"
)

func TestStorage(t *testing.T) {
	p := New([]string{"/", "/definitely-missing-sidinet"})
	v, e := p.Collect(context.Background())
	if e != nil || v == nil {
		t.Fatal(e)
	}
}
