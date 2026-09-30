package system

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMemory(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "meminfo"), []byte("MemTotal: 1000 kB\nMemAvailable: 250 kB\n"), 0600); err != nil {
		t.Fatal(err)
	}
	v, e := Memory(d).Collect(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	m := v.(map[string]any)
	if m["percent"].(float64) != 75 {
		t.Fatalf("percent=%v", m["percent"])
	}
}

func TestCPUWarmup(t *testing.T) {
	d := t.TempDir()
	stat := filepath.Join(d, "stat")
	if err := os.WriteFile(stat, []byte("cpu  100 0 50 850 0 0 0 0 0 0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	p := NewCPU(d)
	v, err := p.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !v.(map[string]any)["warming_up"].(bool) {
		t.Fatal("first CPU sample must warm up")
	}
	if err := os.WriteFile(stat, []byte("cpu  150 0 50 900 0 0 0 0 0 0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	v, err = p.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v.(map[string]any)["warming_up"].(bool) {
		t.Fatal("second CPU sample must be usable")
	}
}
