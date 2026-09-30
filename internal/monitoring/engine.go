package monitoring

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sidinet/sidinet-dashboard-docker/internal/database"
)

const (
	Online   = "ONLINE"
	Offline  = "OFFLINE"
	Degraded = "DEGRADED"
	Unknown  = "UNKNOWN"
)

type Monitor struct {
	ID                  int64  `json:"id"`
	Name                string `json:"name"`
	Type                string `json:"type"`
	Target              string `json:"target"`
	Port                int    `json:"port,omitempty"`
	IntervalSeconds     int    `json:"interval_seconds"`
	TimeoutSeconds      int    `json:"timeout_seconds"`
	Enabled             bool   `json:"enabled"`
	LastStatus          string `json:"last_status"`
	LastLatencyMS       int64  `json:"last_latency_ms"`
	LastCheckedAt       string `json:"last_checked_at"`
	LastError           string `json:"last_error"`
	ConsecutiveFailures int    `json:"consecutive_failures"`
	NextCheckAt         string `json:"next_check_at"`
	LastHTTPStatus      int    `json:"last_http_status,omitempty"`
}
type Result struct {
	Status     string
	LatencyMS  int64
	HTTPStatus int
	Err        error
}

type Engine struct {
	db      *database.DB
	workers int
	tick    time.Duration
	client  *http.Client
	wg      sync.WaitGroup
}

func New(db *database.DB, workers int) *Engine {
	if workers < 1 {
		workers = 1
	}
	if workers > 8 {
		workers = 8
	}
	return &Engine{db: db, workers: workers, tick: time.Second, client: &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return http.ErrUseLastResponse
		}
		return nil
	}, Transport: &http.Transport{Proxy: http.ProxyFromEnvironment, DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, IdleConnTimeout: 30 * time.Second, MaxIdleConns: 8}}}
}
func (e *Engine) Start(ctx context.Context) { e.wg.Add(1); go e.loop(ctx) }
func (e *Engine) Wait()                     { e.wg.Wait() }
func (e *Engine) loop(ctx context.Context) {
	defer e.wg.Done()
	t := time.NewTicker(e.tick)
	defer t.Stop()
	sem := make(chan struct{}, e.workers)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			ms, _ := e.due()
			for _, m := range ms {
				select {
				case sem <- struct{}{}:
					e.wg.Add(1)
					go func(x Monitor) { defer e.wg.Done(); defer func() { <-sem }(); e.run(ctx, x) }(m)
				default:
					return
				}
			}
		}
	}
}
func (e *Engine) due() ([]Monitor, error) {
	raw, err := e.db.QueryText(`SELECT COALESCE(json_group_array(json_object('id',id,'name',name,'type',monitor_type,'target',target,'port',COALESCE(port,0),'interval_seconds',interval_seconds,'timeout_seconds',timeout_seconds,'enabled',enabled,'last_status',COALESCE(last_status,'UNKNOWN'),'last_latency_ms',COALESCE(last_latency_ms,0),'last_checked_at',COALESCE(last_checked_at,''),'last_error',COALESCE(last_error,''),'consecutive_failures',consecutive_failures,'next_check_at',COALESCE(next_check_at,''),'last_http_status',COALESCE(last_http_status,0))),'[]') FROM monitors WHERE enabled=1 AND (next_check_at IS NULL OR next_check_at='' OR datetime(next_check_at)<=datetime('now')) LIMIT 100;`)
	if err != nil {
		return nil, err
	}
	var out []Monitor
	err = json.Unmarshal([]byte(raw), &out)
	return out, err
}
func (e *Engine) run(ctx context.Context, m Monitor) {
	timeout := time.Duration(m.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	r := Check(c, e.client, m)
	failures := m.ConsecutiveFailures
	if r.Status == Online || r.Status == Degraded {
		failures = 0
	} else {
		failures++
	}
	next := time.Now().UTC().Add(Backoff(time.Duration(m.IntervalSeconds)*time.Second, failures))
	errtxt := ""
	if r.Err != nil {
		errtxt = r.Err.Error()
		if len(errtxt) > 500 {
			errtxt = errtxt[:500]
		}
	}
	q := `UPDATE monitors SET last_status=` + database.Quote(r.Status) + `,last_latency_ms=` + strconv.FormatInt(r.LatencyMS, 10) + `,last_http_status=` + strconv.Itoa(r.HTTPStatus) + `,last_checked_at=CURRENT_TIMESTAMP,last_error=` + database.Quote(errtxt) + `,consecutive_failures=` + strconv.Itoa(failures) + `,next_check_at=` + database.Quote(next.Format("2006-01-02 15:04:05")) + `,updated_at=CURRENT_TIMESTAMP WHERE id=` + strconv.FormatInt(m.ID, 10) + `;`
	_ = e.db.Exec(q)
}
func Backoff(base time.Duration, failures int) time.Duration {
	if base <= 0 {
		base = 30 * time.Second
	}
	if failures <= 0 {
		return base
	}
	d := 30 * time.Second
	for i := 1; i < failures; i++ {
		d *= 2
		if d >= 300*time.Second {
			return 300 * time.Second
		}
	}
	if d < base {
		return base
	}
	if d > 300*time.Second {
		return 300 * time.Second
	}
	return d
}
func Check(ctx context.Context, client *http.Client, m Monitor) Result {
	start := time.Now()
	switch strings.ToLower(m.Type) {
	case "http", "https":
		u, err := url.Parse(m.Target)
		if err != nil || !(u.Scheme == "http" || u.Scheme == "https") {
			return Result{Status: Offline, Err: fmt.Errorf("invalid http target")}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return Result{Status: Offline, Err: err}
		}
		req.Header.Set("User-Agent", "SIDINET-Dashboard-Monitor/0.7")
		resp, err := client.Do(req)
		lat := time.Since(start).Milliseconds()
		if err != nil {
			return Result{Status: Offline, LatencyMS: lat, Err: err}
		}
		defer resp.Body.Close()
		_, _ = io.CopyN(io.Discard, resp.Body, 4096)
		status := Online
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			status = Degraded
		} else if resp.StatusCode >= 500 {
			status = Offline
		}
		var re error
		if status != Online {
			re = fmt.Errorf("http status %d", resp.StatusCode)
		}
		return Result{Status: status, LatencyMS: lat, HTTPStatus: resp.StatusCode, Err: re}
	case "tcp":
		host := m.Target
		if strings.Contains(host, ":") && m.Port == 0 {
		} else {
			host = net.JoinHostPort(strings.Trim(host, "[]"), strconv.Itoa(m.Port))
		}
		d := net.Dialer{}
		conn, err := d.DialContext(ctx, "tcp", host)
		lat := time.Since(start).Milliseconds()
		if err != nil {
			return Result{Status: Offline, LatencyMS: lat, Err: err}
		}
		_ = conn.Close()
		return Result{Status: Online, LatencyMS: lat}
	default:
		return Result{Status: Unknown, Err: fmt.Errorf("unsupported monitor type")}
	}
}
