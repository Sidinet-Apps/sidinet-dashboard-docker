package storage

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type Path struct {
	Label     string  `json:"label"`
	Path      string  `json:"path"`
	Available bool    `json:"available"`
	Total     uint64  `json:"total"`
	Free      uint64  `json:"free"`
	Used      uint64  `json:"used"`
	Percent   float64 `json:"percent"`
	Error     string  `json:"error,omitempty"`
}
type Provider struct{ paths []string }

func New(paths []string) *Provider {
	seen := map[string]bool{}
	out := []string{}
	for _, p := range paths {
		p = filepath.Clean(p)
		if p != "." && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		out = []string{"/data"}
	}
	return &Provider{paths: out}
}
func (p *Provider) Key() string               { return "storage.disks" }
func (p *Provider) DefaultTTL() time.Duration { return 30 * time.Second }
func (p *Provider) Collect(ctx context.Context) (any, error) {
	out := []Path{}
	for _, x := range p.paths {
		r := Path{Label: filepath.Base(x), Path: x}
		if _, e := os.Stat(x); e != nil {
			r.Error = e.Error()
			out = append(out, r)
			continue
		}
		var s syscall.Statfs_t
		if e := syscall.Statfs(x, &s); e != nil {
			r.Error = e.Error()
			out = append(out, r)
			continue
		}
		r.Available = true
		r.Total = s.Blocks * uint64(s.Bsize)
		r.Free = s.Bavail * uint64(s.Bsize)
		r.Used = r.Total - r.Free
		if r.Total > 0 {
			r.Percent = float64(r.Used) * 100 / float64(r.Total)
		}
		out = append(out, r)
	}
	return map[string]any{"disks": out}, nil
}
