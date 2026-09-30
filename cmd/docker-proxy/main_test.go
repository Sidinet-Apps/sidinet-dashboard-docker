package main

import "testing"

func TestAllowlist(t *testing.T) {
	good := []string{"/_ping", "/version", "/info", "/containers/json", "/v1.47/containers/json", "/containers/abc123/json", "/containers/abc123/stats"}
	bad := []string{"/containers/abc123/start", "/containers/abc123/stop", "/containers/create", "/exec/123/start", "/images/create", "/events", "/containers/abc123/archive", "/containers/abc123/logs", "/networks", "/volumes"}
	for _, p := range good {
		if !permitted(p) {
			t.Fatalf("expected allowed: %s", p)
		}
	}
	for _, p := range bad {
		if permitted(p) {
			t.Fatalf("expected blocked: %s", p)
		}
	}
}
