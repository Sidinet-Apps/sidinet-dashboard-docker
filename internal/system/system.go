package system

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type FileProvider struct {
	key, path string
	ttl       time.Duration
	parse     func(string) (any, error)
}

func (p FileProvider) Key() string               { return p.key }
func (p FileProvider) DefaultTTL() time.Duration { return p.ttl }
func (p FileProvider) Collect(ctx context.Context) (any, error) {
	b, e := os.ReadFile(p.path)
	if e != nil {
		return nil, e
	}
	return p.parse(string(b))
}
func Uptime(root string) FileProvider {
	return FileProvider{"system.uptime", root + "/uptime", 30 * time.Second, func(s string) (any, error) {
		f := strings.Fields(s)
		if len(f) == 0 {
			return nil, fmt.Errorf("invalid uptime")
		}
		v, e := strconv.ParseFloat(f[0], 64)
		return map[string]any{"seconds": v}, e
	}}
}
func Load(root string) FileProvider {
	return FileProvider{"system.load", root + "/loadavg", 10 * time.Second, func(s string) (any, error) {
		f := strings.Fields(s)
		if len(f) < 3 {
			return nil, fmt.Errorf("invalid loadavg")
		}
		return map[string]string{"load1": f[0], "load5": f[1], "load15": f[2]}, nil
	}}
}
func Memory(root string) FileProvider {
	return FileProvider{"system.memory", root + "/meminfo", 5 * time.Second, func(s string) (any, error) {
		m := map[string]uint64{}
		sc := bufio.NewScanner(strings.NewReader(s))
		for sc.Scan() {
			f := strings.Fields(sc.Text())
			if len(f) >= 2 {
				v, _ := strconv.ParseUint(f[1], 10, 64)
				m[strings.TrimSuffix(f[0], ":")] = v * 1024
			}
		}
		total := m["MemTotal"]
		avail := m["MemAvailable"]
		used := total - avail
		pct := 0.0
		if total > 0 {
			pct = float64(used) * 100 / float64(total)
		}
		return map[string]any{"total": total, "available": avail, "used": used, "percent": pct}, nil
	}}
}

type CPU struct {
	path                string
	mu                  sync.Mutex
	prevTotal, prevIdle uint64
}

func NewCPU(root string) *CPU            { return &CPU{path: root + "/stat"} }
func (c *CPU) Key() string               { return "system.cpu" }
func (c *CPU) DefaultTTL() time.Duration { return 5 * time.Second }
func (c *CPU) Collect(ctx context.Context) (any, error) {
	b, e := os.ReadFile(c.path)
	if e != nil {
		return nil, e
	}
	line := strings.SplitN(string(b), "\n", 2)[0]
	f := strings.Fields(line)
	if len(f) < 5 {
		return nil, fmt.Errorf("invalid cpu stat")
	}
	var vals []uint64
	for _, x := range f[1:] {
		v, _ := strconv.ParseUint(x, 10, 64)
		vals = append(vals, v)
	}
	var total uint64
	for _, v := range vals {
		total += v
	}
	idle := vals[3]
	if len(vals) > 4 {
		idle += vals[4]
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	warming := c.prevTotal == 0
	pct := 0.0
	if !warming && total > c.prevTotal {
		dt := total - c.prevTotal
		di := idle - c.prevIdle
		pct = (1 - float64(di)/float64(dt)) * 100
	}
	c.prevTotal, c.prevIdle = total, idle
	return map[string]any{"percent": pct, "warming_up": warming}, nil
}

type Temp struct{ path string }

func NewTemp(root string) *Temp           { return &Temp{root + "/thermal_zone0/temp"} }
func (t *Temp) Key() string               { return "system.temperature" }
func (t *Temp) DefaultTTL() time.Duration { return 10 * time.Second }
func (t *Temp) Collect(ctx context.Context) (any, error) {
	b, e := os.ReadFile(t.path)
	if e != nil {
		return nil, e
	}
	v, e := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
	if e != nil {
		return nil, e
	}
	return map[string]any{"celsius": v / 1000}, nil
}
