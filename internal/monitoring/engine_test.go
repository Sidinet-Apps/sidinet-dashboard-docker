package monitoring

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPStates(t *testing.T) {
	for code, want := range map[int]string{200: Online, 302: Online, 404: Degraded, 503: Offline} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }))
		c := s.Client()
		c.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
		got := Check(context.Background(), c, Monitor{Type: "http", Target: s.URL, TimeoutSeconds: 1})
		s.Close()
		if got.Status != want {
			t.Fatalf("%d: %s", code, got.Status)
		}
	}
}
func TestTCP(t *testing.T) {
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer ln.Close()
	go func() {
		c, _ := ln.Accept()
		if c != nil {
			c.Close()
		}
	}()
	got := Check(context.Background(), &http.Client{}, Monitor{Type: "tcp", Target: ln.Addr().String(), TimeoutSeconds: 1})
	if got.Status != Online {
		t.Fatal(got.Status)
	}
}
func TestBackoff(t *testing.T) {
	if Backoff(30*time.Second, 1) != 30*time.Second || Backoff(30*time.Second, 2) != 60*time.Second || Backoff(30*time.Second, 3) != 120*time.Second || Backoff(30*time.Second, 9) != 300*time.Second {
		t.Fatal("backoff")
	}
}
