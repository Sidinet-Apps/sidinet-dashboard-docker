package discovery

import (
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"

	"github.com/sidinet/sidinet-dashboard-docker/internal/dockerapi"
)

type Service struct {
	StableKey     string            `json:"stable_key"`
	Project       string            `json:"project"`
	Service       string            `json:"service"`
	ContainerName string            `json:"container_name"`
	ContainerID   string            `json:"container_id"`
	Image         string            `json:"image"`
	State         string            `json:"state"`
	Status        string            `json:"status"`
	Health        string            `json:"health,omitempty"`
	Ports         []dockerapi.Port  `json:"ports"`
	Labels        map[string]string `json:"labels"`
	SuggestedName string            `json:"suggested_name"`
	SuggestedURL  string            `json:"suggested_url,omitempty"`
	SuggestedIcon string            `json:"suggested_icon,omitempty"`
	WorkingDir    string            `json:"working_dir,omitempty"`
	ConfigFiles   string            `json:"config_files,omitempty"`
}

func Normalize(c dockerapi.Container, dashboardHost string) Service {
	name := strings.TrimPrefix(first(c.Names), "/")
	project := c.Labels["com.docker.compose.project"]
	svc := c.Labels["com.docker.compose.service"]
	if svc == "" {
		svc = name
	}
	stable := "container:" + name
	if project != "" {
		stable = "compose:" + project + ":" + svc
	}
	display := firstNonEmpty(c.Labels["sidinet.name"], svc, name, c.Image)
	u := c.Labels["sidinet.url"]
	if u == "" {
		u = suggestedURL(dashboardHost, c.Ports)
	}
	return Service{StableKey: stable, Project: project, Service: svc, ContainerName: name, ContainerID: c.ID, Image: c.Image, State: c.State, Status: c.Status, Ports: c.Ports, Labels: c.Labels, SuggestedName: display, SuggestedURL: u, SuggestedIcon: c.Labels["sidinet.icon"], WorkingDir: c.Labels["com.docker.compose.project.working_dir"], ConfigFiles: c.Labels["com.docker.compose.project.config_files"]}
}
func suggestedURL(host string, ports []dockerapi.Port) string {
	var pp []dockerapi.Port
	for _, p := range ports {
		if p.PublicPort > 0 && strings.EqualFold(p.Type, "tcp") {
			pp = append(pp, p)
		}
	}
	if len(pp) == 0 {
		return ""
	}
	sort.Slice(pp, func(i, j int) bool { return pp[i].PublicPort < pp[j].PublicPort })
	h := host
	if x, _, e := net.SplitHostPort(host); e == nil {
		h = x
	}
	h = strings.Trim(h, "[]")
	// Prefer ports that commonly expose a Web UI. Never choose a lower peer/database port merely because it is numerically smaller.
	preferred := map[int]int{80:0,443:0,8080:1,8081:1,8000:2,8123:2,8096:2,32400:2,9000:2,9443:2}
	sort.SliceStable(pp, func(i, j int) bool {
		ri, iok := preferred[pp[i].PrivatePort]; rj, jok := preferred[pp[j].PrivatePort]
		if iok != jok { return iok }
		if iok && ri != rj { return ri < rj }
		return pp[i].PublicPort < pp[j].PublicPort
	})
	scheme := "http"
	if pp[0].PublicPort == 443 || pp[0].PrivatePort == 443 {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, h, pp[0].PublicPort)
}
func ValidSuggestedURL(v string) bool {
	u, e := url.Parse(v)
	return e == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
func first(v []string) string {
	if len(v) > 0 {
		return v[0]
	}
	return ""
}
func firstNonEmpty(v ...string) string {
	for _, x := range v {
		if strings.TrimSpace(x) != "" {
			return x
		}
	}
	return "Servicio"
}
