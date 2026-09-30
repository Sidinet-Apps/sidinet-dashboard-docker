package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr         string
	DataDir            string
	DatabasePath       string
	Timezone           string
	LogLevel           string
	SafeMode           bool
	DockerEnabled      bool
	DockerHost         string
	ComposePath        string
	PerformanceProfile string
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	StoragePaths       []string
	HostResolvConf     string
	InternetTarget     string
}

func Load() Config {
	data := env("SIDINET_DATA_DIR", "/data")
	return Config{
		ListenAddr: env("SIDINET_LISTEN", ":8080"), DataDir: data,
		DatabasePath: env("SIDINET_DATABASE", data+"/database.sqlite"),
		Timezone:     env("TZ", "UTC"), LogLevel: env("SIDINET_LOG_LEVEL", "INFO"),
		SafeMode:           boolEnv("SIDINET_SAFE_MODE", false),
		DockerEnabled:      boolEnv("SIDINET_DOCKER_ENABLED", false),
		DockerHost:         env("SIDINET_DOCKER_HOST", "http://docker-proxy:2375"),
		ComposePath:        env("SIDINET_COMPOSE_PATH", "/compose"),
		PerformanceProfile: env("SIDINET_PERFORMANCE_PROFILE", "balanced"),
		ReadTimeout:        10 * time.Second, WriteTimeout: 15 * time.Second,
		StoragePaths: splitEnv("SIDINET_STORAGE_PATHS", "/data"), HostResolvConf: env("SIDINET_HOST_RESOLV_CONF", "/host/etc/resolv.conf"), InternetTarget: env("SIDINET_INTERNET_TARGET", "1.1.1.1:53"),
	}
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func boolEnv(k string, d bool) bool {
	v := os.Getenv(k)
	if v == "" {
		return d
	}
	b, e := strconv.ParseBool(v)
	if e != nil {
		return d
	}
	return b
}

func splitEnv(k, d string) []string {
	v := env(k, d)
	out := []string{}
	for _, x := range strings.Split(v, ",") {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}
