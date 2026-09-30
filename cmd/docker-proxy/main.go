package main

import (
	"context"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"
)

var allowed = []*regexp.Regexp{
	regexp.MustCompile(`^/(v[0-9.]+/)?_ping$`),
	regexp.MustCompile(`^/(v[0-9.]+/)?version$`),
	regexp.MustCompile(`^/(v[0-9.]+/)?info$`),
	regexp.MustCompile(`^/(v[0-9.]+/)?containers/json$`),
	regexp.MustCompile(`^/(v[0-9.]+/)?containers/[A-Za-z0-9_.-]+/json$`),
	regexp.MustCompile(`^/(v[0-9.]+/)?containers/[A-Za-z0-9_.-]+/stats$`),
}

func permitted(path string) bool {
	for _, r := range allowed {
		if r.MatchString(path) {
			return true
		}
	}
	return false
}

func main() {
	listen := flag.String("listen", ":2375", "listen address")
	socket := flag.String("socket", "/var/run/docker.sock", "docker unix socket")
	flag.Parse()
	tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "unix", *socket)
	}}
	client := &http.Client{Transport: tr, Timeout: 10 * time.Second}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "read-only", http.StatusMethodNotAllowed)
			return
		}
		if !permitted(r.URL.Path) {
			http.Error(w, "endpoint not allowed", http.StatusForbidden)
			return
		}
		u := "http://docker" + r.URL.RequestURI()
		req, err := http.NewRequestWithContext(r.Context(), r.Method, u, nil)
		if err != nil {
			http.Error(w, "bad request", 400)
			return
		}
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "docker unavailable", 502)
			return
		}
		defer resp.Body.Close()
		for k, vv := range resp.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	})
	srv := &http.Server{Addr: *listen, Handler: h, ReadHeaderTimeout: 3 * time.Second, IdleTimeout: 30 * time.Second}
	go func() {
		log.Printf("SIDINET Docker read-only proxy listening on %s", *listen)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
