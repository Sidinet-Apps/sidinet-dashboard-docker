package network

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Interface struct {
	Name    string  `json:"name"`
	RXBytes uint64  `json:"rx_bytes"`
	TXBytes uint64  `json:"tx_bytes"`
	RXBps   float64 `json:"rx_bps"`
	TXBps   float64 `json:"tx_bps"`
}
type sample struct {
	rx, tx uint64
	at     time.Time
}
type Interfaces struct {
	path string
	mu   sync.Mutex
	prev map[string]sample
}

func NewInterfaces(proc string) *Interfaces {
	return &Interfaces{path: proc + "/net/dev", prev: map[string]sample{}}
}
func (p *Interfaces) Key() string               { return "network.interfaces" }
func (p *Interfaces) DefaultTTL() time.Duration { return 5 * time.Second }
func (p *Interfaces) Collect(ctx context.Context) (any, error) {
	b, e := os.ReadFile(p.path)
	if e != nil {
		return nil, e
	}
	now := time.Now()
	rows, e := ParseDev(string(b))
	if e != nil {
		return nil, e
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range rows {
		if old, ok := p.prev[rows[i].Name]; ok {
			dt := now.Sub(old.at).Seconds()
			if dt > 0 && rows[i].RXBytes >= old.rx {
				rows[i].RXBps = float64(rows[i].RXBytes-old.rx) / dt
			}
			if dt > 0 && rows[i].TXBytes >= old.tx {
				rows[i].TXBps = float64(rows[i].TXBytes-old.tx) / dt
			}
		}
		p.prev[rows[i].Name] = sample{rows[i].RXBytes, rows[i].TXBytes, now}
	}
	return map[string]any{"interfaces": rows}, nil
}
func ParseDev(s string) ([]Interface, error) {
	out := []Interface{}
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.Contains(line, ":") {
			continue
		}
		a := strings.SplitN(line, ":", 2)
		f := strings.Fields(a[1])
		if len(f) < 16 {
			continue
		}
		rx, _ := strconv.ParseUint(f[0], 10, 64)
		tx, _ := strconv.ParseUint(f[8], 10, 64)
		out = append(out, Interface{Name: strings.TrimSpace(a[0]), RXBytes: rx, TXBytes: tx})
	}
	if len(out) == 0 {
		return out, fmt.Errorf("no network interfaces")
	}
	return out, sc.Err()
}

type IPs struct{ fib, inet6 string }

func NewIPs(proc string) *IPs {
	return &IPs{fib: proc + "/net/fib_trie", inet6: proc + "/net/if_inet6"}
}
func (p *IPs) Key() string               { return "network.ips" }
func (p *IPs) DefaultTTL() time.Duration { return 30 * time.Second }
func (p *IPs) Collect(ctx context.Context) (any, error) {
	b, e := os.ReadFile(p.fib)
	if e != nil {
		return nil, e
	}
	seen := map[string]bool{}
	ips := []string{}
	lines := strings.Split(string(b), "\n")
	for i, l := range lines {
		if strings.Contains(l, "/32 host LOCAL") && i > 0 {
			f := strings.Fields(lines[i-1])
			if len(f) > 0 {
				ip := f[len(f)-1]
				if net.ParseIP(ip) != nil && ip != "127.0.0.1" && !seen[ip] {
					seen[ip] = true
					ips = append(ips, ip)
				}
			}
		}
	}
	// Linux exposes host IPv6 addresses in /proc/net/if_inet6 as 32 hex digits.
	if b6, err := os.ReadFile(p.inet6); err == nil {
		sc := bufio.NewScanner(strings.NewReader(string(b6)))
		for sc.Scan() {
			f := strings.Fields(sc.Text())
			if len(f) < 6 || len(f[0]) != 32 {
				continue
			}
			raw, err := hex.DecodeString(f[0])
			if err != nil {
				continue
			}
			ip := net.IP(raw)
			if ip.IsLoopback() || ip.IsUnspecified() {
				continue
			}
			text := ip.String()
			if text != "" && !seen[text] {
				seen[text] = true
				ips = append(ips, text)
			}
		}
	}
	return map[string]any{"addresses": ips}, nil
}

type DNS struct{ path string }

func NewDNS(path string) *DNS            { return &DNS{path: path} }
func (p *DNS) Key() string               { return "network.dns" }
func (p *DNS) DefaultTTL() time.Duration { return 60 * time.Second }
func (p *DNS) Collect(ctx context.Context) (any, error) {
	b, e := os.ReadFile(p.path)
	servers := []string{}
	if e == nil {
		sc := bufio.NewScanner(strings.NewReader(string(b)))
		for sc.Scan() {
			f := strings.Fields(sc.Text())
			if len(f) >= 2 && f[0] == "nameserver" {
				servers = append(servers, f[1])
			}
		}
	}
	start := time.Now()
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	resolver := net.DefaultResolver
	if len(servers) > 0 {
		server := servers[0]
		resolver = &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "udp", net.JoinHostPort(server, "53"))
		}}
	}
	addrs, re := resolver.LookupHost(cctx, "example.com")
	ok := re == nil && len(addrs) > 0
	errtxt := ""
	if re != nil {
		errtxt = re.Error()
	}
	return map[string]any{"servers": servers, "tested_server": first(servers), "ok": ok, "latency_ms": time.Since(start).Milliseconds(), "error": errtxt}, nil
}

type Internet struct{ Target string }

func (p Internet) Key() string               { return "network.internet" }
func (p Internet) DefaultTTL() time.Duration { return 30 * time.Second }
func (p Internet) Collect(ctx context.Context) (any, error) {
	target := p.Target
	if target == "" {
		target = "1.1.1.1:53"
	}
	c := net.Dialer{Timeout: 2 * time.Second}
	start := time.Now()
	conn, e := c.DialContext(ctx, "tcp", target)
	if e == nil {
		conn.Close()
	}
	errtxt := ""
	if e != nil {
		errtxt = e.Error()
	}
	return map[string]any{"online": e == nil, "target": target, "latency_ms": time.Since(start).Milliseconds(), "error": errtxt}, nil
}

func first(v []string) string {
	if len(v) == 0 {
		return ""
	}
	return v[0]
}
