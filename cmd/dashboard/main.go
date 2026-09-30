package main

import (
	"fmt"
	"github.com/sidinet/sidinet-dashboard-docker/internal/app"
	"github.com/sidinet/sidinet-dashboard-docker/internal/config"
	"github.com/sidinet/sidinet-dashboard-docker/internal/logging"
	"github.com/sidinet/sidinet-dashboard-docker/internal/version"
	"github.com/sidinet/sidinet-dashboard-docker/internal/webassets"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(version.Version)
		return
	}
	cfg := config.Load()
	log := logging.New(cfg.LogLevel)
	a, err := app.New(cfg, log, webassets.FS)
	if err != nil {
		log.Error("startup failed", "error", err)
		os.Exit(1)
	}
	if err := a.Run(); err != nil {
		log.Error("application stopped", "error", err)
		os.Exit(1)
	}
}
