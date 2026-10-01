package discovery

import (
	"github.com/sidinet/sidinet-dashboard-docker/internal/dockerapi"
	"testing"
)

func TestNormalizeCompose(t *testing.T) {
	c := dockerapi.Container{ID: "abc", Names: []string{"/jellyfin-1"}, Image: "jellyfin/jellyfin", State: "running", Ports: []dockerapi.Port{{PrivatePort: 8096, PublicPort: 8096, Type: "tcp"}}, Labels: map[string]string{"com.docker.compose.project": "media", "com.docker.compose.service": "jellyfin"}}
	s := Normalize(c, "192.168.1.20:8585")
	if s.StableKey != "compose:media:jellyfin" {
		t.Fatal(s.StableKey)
	}
	if s.SuggestedURL != "http://192.168.1.20:8096" {
		t.Fatal(s.SuggestedURL)
	}
}
func TestSidinetOverrides(t *testing.T) {
	c := dockerapi.Container{Names: []string{"/x"}, Labels: map[string]string{"sidinet.name": "Mi App", "sidinet.url": "https://app.local", "sidinet.icon": "custom"}}
	s := Normalize(c, "host")
	if s.SuggestedName != "Mi App" || s.SuggestedURL != "https://app.local" || s.SuggestedIcon != "custom" {
		t.Fatalf("%+v", s)
	}
}

func TestSuggestedURLPrefersWebUIPort(t *testing.T) {
	c := dockerapi.Container{Names: []string{"/qbittorrent"}, Image: "linuxserver/qbittorrent", State: "running", Ports: []dockerapi.Port{
		{PrivatePort: 6881, PublicPort: 6881, Type: "tcp"},
		{PrivatePort: 8080, PublicPort: 8080, Type: "tcp"},
	}}
	s := Normalize(c, "192.168.1.20")
	if s.SuggestedURL != "http://192.168.1.20:8080" { t.Fatalf("got %s", s.SuggestedURL) }
}
