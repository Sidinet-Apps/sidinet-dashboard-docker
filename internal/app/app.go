package app

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"github.com/sidinet/sidinet-dashboard-docker/internal/config"
	"github.com/sidinet/sidinet-dashboard-docker/internal/database"
	"github.com/sidinet/sidinet-dashboard-docker/internal/httpapi"
	"github.com/sidinet/sidinet-dashboard-docker/internal/monitoring"
	"github.com/sidinet/sidinet-dashboard-docker/internal/network"
	"github.com/sidinet/sidinet-dashboard-docker/internal/providers"
	"github.com/sidinet/sidinet-dashboard-docker/internal/recovery"
	"github.com/sidinet/sidinet-dashboard-docker/internal/storage"
	sys "github.com/sidinet/sidinet-dashboard-docker/internal/system"
	"github.com/sidinet/sidinet-dashboard-docker/internal/widgets"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type App struct {
	cfg     config.Config
	log     *slog.Logger
	db      *database.DB
	server  *httpapi.Server
	monitor *monitoring.Engine
}

func New(cfg config.Config, log *slog.Logger, web embed.FS) (*App, error) {
	if err := os.MkdirAll(cfg.DataDir, 0750); err != nil {
		return nil, err
	}
	if err := recovery.ApplyPending(cfg.DataDir); err != nil {
		return nil, fmt.Errorf("pending restore: %w", err)
	}
	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		return nil, err
	}
	if err = db.Migrate(); err != nil {
		db.Close()
		return nil, err
	}
	p := providers.New()
	p.Register(sys.NewCPU("/host/proc"))
	p.Register(sys.Memory("/host/proc"))
	p.Register(sys.Load("/host/proc"))
	p.Register(sys.Uptime("/host/proc"))
	p.Register(sys.NewTemp("/host/sys/class/thermal"))
	p.Register(network.NewInterfaces("/host/proc"))
	p.Register(network.NewIPs("/host/proc"))
	p.Register(network.NewDNS(cfg.HostResolvConf))
	p.Register(network.Internet{Target: cfg.InternetTarget})
	p.Register(storage.New(cfg.StoragePaths))
	w := widgets.New()
	assets, err := fs.Sub(web, "web")
	if err != nil {
		return nil, err
	}
	return &App{cfg: cfg, log: log, db: db, server: httpapi.New(cfg, log, db, p, w, assets), monitor: monitoring.New(db, 4)}, nil
}
func (a *App) Run() error {
	root, cancelRoot := context.WithCancel(context.Background())
	defer cancelRoot()
	if !a.cfg.SafeMode {
		a.monitor.Start(root)
	}
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	errch := make(chan error, 1)
	go func() {
		a.log.Info("server starting", "listen", a.cfg.ListenAddr, "safe_mode", a.cfg.SafeMode)
		errch <- a.server.ListenAndServe()
	}()
	select {
	case sig := <-ch:
		a.log.Info("shutdown requested", "signal", sig.String())
		cancelRoot()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = a.server.Shutdown(ctx)
		return a.db.Close()
	case err := <-errch:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		_ = a.db.Close()
		return err
	}
}
