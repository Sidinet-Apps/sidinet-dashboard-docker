package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sidinet/sidinet-dashboard-docker/internal/config"
	"github.com/sidinet/sidinet-dashboard-docker/internal/database"
	"github.com/sidinet/sidinet-dashboard-docker/internal/discovery"
	"github.com/sidinet/sidinet-dashboard-docker/internal/dockerapi"
	"github.com/sidinet/sidinet-dashboard-docker/internal/providers"
	"github.com/sidinet/sidinet-dashboard-docker/internal/recovery"
	"github.com/sidinet/sidinet-dashboard-docker/internal/themes"
	"github.com/sidinet/sidinet-dashboard-docker/internal/version"
	"github.com/sidinet/sidinet-dashboard-docker/internal/widgets"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type Server struct {
	cfg       config.Config
	log       *slog.Logger
	db        *database.DB
	providers *providers.Registry
	widgets   *widgets.Registry
	docker    *dockerapi.Client
	assets    fs.FS
	http      *http.Server
	started   time.Time
	requests  atomic.Uint64
	errors    atomic.Uint64
	active    atomic.Int64
}

func New(cfg config.Config, log *slog.Logger, db *database.DB, p *providers.Registry, w *widgets.Registry, assets fs.FS) *Server {
	s := &Server{cfg: cfg, log: log, db: db, providers: p, widgets: w, assets: assets, started: time.Now()}
	if cfg.DockerEnabled && !cfg.SafeMode {
		s.docker = dockerapi.New(cfg.DockerHost)
	}
	m := http.NewServeMux()
	m.HandleFunc("/api/v1/health", s.health)
	m.HandleFunc("/api/v1/ready", s.ready)
	m.HandleFunc("/api/v1/version", s.ver)
	m.HandleFunc("/api/v1/capabilities", s.capabilities)
	m.HandleFunc("/api/v1/diagnostics/performance", s.performance)
	m.HandleFunc("/api/v1/widgets/catalog", s.catalog)
	m.HandleFunc("/api/v1/runtime/system", s.runtimeSystem)
	m.HandleFunc("/api/v1/pages", s.pages)
	m.HandleFunc("/api/v1/widgets", s.widgetCRUD)
	m.HandleFunc("/api/v1/widgets/", s.widgetAction)
	m.HandleFunc("/api/v1/applications", s.applications)
	m.HandleFunc("/api/v1/monitors", s.monitors)
	m.HandleFunc("/api/v1/runtime/page/", s.runtimePage)
	m.HandleFunc("/api/v1/layouts", s.layouts)
	m.HandleFunc("/api/v1/themes", s.themeAPI)
	m.HandleFunc("/api/v1/themes/presets", s.themePresets)
	m.HandleFunc("/api/v1/recovery/status", s.recoveryStatus)
	m.HandleFunc("/api/v1/recovery/backup", s.recoveryBackup)
	m.HandleFunc("/api/v1/recovery/restore", s.recoveryRestore)
	m.HandleFunc("/api/v1/recovery/diagnostics", s.recoveryDiagnostics)
	m.HandleFunc("/api/v1/docker/status", s.dockerStatus)
	m.HandleFunc("/api/v1/docker/containers", s.dockerContainers)
	m.HandleFunc("/api/v1/discovery/docker", s.dockerDiscovery)
	m.HandleFunc("/api/v1/discovery/adopt", s.discoveryAdopt)
	m.HandleFunc("/api/v1/discovery/ignore", s.discoveryIgnore)
	m.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assets))))
	m.HandleFunc("/", s.index)
	s.http = &http.Server{Addr: cfg.ListenAddr, Handler: s.middleware(m), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: cfg.ReadTimeout, WriteTimeout: cfg.WriteTimeout, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	return s
}
func (s *Server) ListenAndServe() error              { return s.http.ListenAndServe() }
func (s *Server) Shutdown(ctx context.Context) error { return s.http.Shutdown(ctx) }
func (s *Server) json(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	s.json(w, 200, map[string]any{"status": "ok"})
}
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if e := s.db.Ping(); e != nil {
		s.json(w, 503, map[string]any{"status": "not_ready", "database": e.Error()})
		return
	}
	s.json(w, 200, map[string]any{"status": "ready"})
}
func (s *Server) ver(w http.ResponseWriter, r *http.Request) {
	s.json(w, 200, map[string]string{"version": version.Version, "commit": version.Commit, "build_date": version.BuildDate, "schema": version.SchemaVersion})
}

func (s *Server) performance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	s.json(w, 200, map[string]any{"uptime_seconds": int64(time.Since(s.started).Seconds()), "goroutines": runtime.NumGoroutine(), "go_heap_bytes": m.HeapAlloc, "go_sys_bytes": m.Sys, "gc_cycles": m.NumGC, "requests_total": s.requests.Load(), "requests_active": s.active.Load(), "errors_total": s.errors.Load(), "providers": s.providers.Stats()})
}
func (s *Server) capabilities(w http.ResponseWriter, r *http.Request) {
	s.json(w, 200, map[string]any{"docker.discovery": s.cfg.DockerEnabled && !s.cfg.SafeMode, "compose.discovery": s.cfg.ComposePath != "" && !s.cfg.SafeMode, "monitor.http": true, "monitor.tcp": true, "network.interfaces": true, "network.ips": true, "network.internet": true, "network.dns": true, "storage.disks": true, "themes": true, "theme.presets": true, "safe_mode": s.cfg.SafeMode})
}
func (s *Server) catalog(w http.ResponseWriter, r *http.Request) {
	s.json(w, 200, map[string]any{"widgets": s.widgets.All()})
}
func (s *Server) runtimeSystem(w http.ResponseWriter, r *http.Request) {
	keys := []string{"system.cpu", "system.memory", "system.load", "system.uptime", "system.temperature", "network.interfaces", "network.ips", "network.internet", "network.dns", "storage.disks"}
	out := map[string]any{}
	for _, k := range keys {
		res, e := s.providers.Get(r.Context(), k)
		if e != nil {
			out[k] = map[string]any{"error": e.Error()}
			continue
		}
		out[k] = res
	}
	s.json(w, 200, map[string]any{"providers": out})
}

func slugify(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.ReplaceAll(v, " ", "-")
	var b strings.Builder
	for _, r := range v {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "-")
}
func (s *Server) pages(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var in struct{ Name, Slug string }
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&in) != nil || strings.TrimSpace(in.Name) == "" {
			s.json(w, 400, map[string]any{"error": "invalid page"})
			return
		}
		if in.Slug == "" {
			in.Slug = slugify(in.Name)
		}
		if in.Slug == "" {
			s.json(w, 400, map[string]any{"error": "invalid slug"})
			return
		}
		q := `INSERT INTO pages(name,slug,position) VALUES(` + database.Quote(in.Name) + `,` + database.Quote(in.Slug) + `,COALESCE((SELECT MAX(position)+1 FROM pages),0));`
		if e := s.db.Exec(q); e != nil {
			s.json(w, 409, map[string]any{"error": e.Error()})
			return
		}
		s.json(w, 201, map[string]any{"ok": true, "slug": in.Slug})
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	q := `SELECT COALESCE(json_group_array(json_object('id',id,'name',name,'slug',slug,'position',position,'is_default',is_default)), '[]') FROM (SELECT * FROM pages WHERE enabled=1 ORDER BY position,id);`
	v, err := s.db.QueryText(q)
	if err != nil {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	var rows any
	_ = json.Unmarshal([]byte(v), &rows)
	s.json(w, 200, map[string]any{"pages": rows})
}
func (s *Server) applications(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPut {
		var in struct {
			ID int64 `json:"id"`
			Name, URL, Description, IconType, IconValue, OpenMode string
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&in) != nil || in.ID < 1 || strings.TrimSpace(in.Name) == "" || !(strings.HasPrefix(in.URL, "http://") || strings.HasPrefix(in.URL, "https://")) {
			s.json(w, 400, map[string]any{"error": "invalid application"})
			return
		}
		if in.IconType == "" { in.IconType = "auto" }
		if in.OpenMode == "" { in.OpenMode = "new_tab" }
		q := `UPDATE applications SET name=`+database.Quote(in.Name)+`,url=`+database.Quote(in.URL)+`,description=`+database.Quote(in.Description)+`,icon_type=`+database.Quote(in.IconType)+`,icon_value=`+database.Quote(in.IconValue)+`,open_mode=`+database.Quote(in.OpenMode)+`,updated_at=CURRENT_TIMESTAMP WHERE id=`+strconv.FormatInt(in.ID,10)+`;`
		if e := s.db.Exec(q); e != nil { s.json(w,500,map[string]any{"error":e.Error()}); return }
		s.json(w,200,map[string]any{"ok":true})
		return
	}
	if r.Method == http.MethodGet {
		v, e := s.db.QueryText(`SELECT COALESCE(json_group_array(json_object('id',id,'name',name,'url',url,'description',COALESCE(description,''),'open_mode',open_mode,'icon_type',icon_type,'icon_value',COALESCE(icon_value,''),'source_type',source_type)), '[]') FROM (SELECT * FROM applications WHERE enabled=1 ORDER BY name);`)
		if e != nil {
			s.json(w, 500, map[string]any{"error": e.Error()})
			return
		}
		var a any
		_ = json.Unmarshal([]byte(v), &a)
		s.json(w, 200, map[string]any{"applications": a})
		return
	}
	if r.Method == http.MethodPost {
		var in struct{ Name, URL, Description string }
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&in) != nil || strings.TrimSpace(in.Name) == "" || !(strings.HasPrefix(in.URL, "http://") || strings.HasPrefix(in.URL, "https://")) {
			s.json(w, 400, map[string]any{"error": "invalid application"})
			return
		}
		q := `INSERT INTO applications(name,url,description) VALUES(` + database.Quote(in.Name) + `,` + database.Quote(in.URL) + `,` + database.Quote(in.Description) + `);`
		if e := s.db.Exec(q); e != nil {
			s.json(w, 500, map[string]any{"error": e.Error()})
			return
		}
		id, _ := s.db.QueryText(`SELECT CAST(last_insert_rowid() AS TEXT);`)
		s.json(w, 201, map[string]any{"ok": true, "id": id})
		return
	}
	http.Error(w, "method not allowed", 405)
}
func (s *Server) widgetCRUD(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var in struct {
		PageID         int64 `json:"page_id"`
		Type, Title    string
		ApplicationID  int64          `json:"application_id"`
		Config         map[string]any `json:"config"`
		ParentWidgetID int64          `json:"parent_widget_id"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&in) != nil || in.PageID < 1 {
		s.json(w, 400, map[string]any{"error": "invalid widget"})
		return
	}
	d, ok := s.widgets.Get(in.Type)
	if !ok {
		s.json(w, 400, map[string]any{"error": "unknown widget type"})
		return
	}
	cfg := "{}"
	if len(in.Config) > 0 {
		if b, e := json.Marshal(in.Config); e == nil {
			cfg = string(b)
		}
	}
	if in.Type == "application.shortcut" {
		cfg = fmt.Sprintf(`{"application_id":%d}`, in.ApplicationID)
	}
	if in.Title == "" {
		in.Title = d.Name
	}
	q := `INSERT INTO widgets(page_id,widget_type,provider_type,title,config,parent_widget_id) VALUES(` + strconv.FormatInt(in.PageID, 10) + `,` + database.Quote(in.Type) + `,` + database.Quote(d.Provider) + `,` + database.Quote(in.Title) + `,` + database.Quote(cfg) + `,` + func() string {
		if in.ParentWidgetID > 0 {
			return strconv.FormatInt(in.ParentWidgetID, 10)
		}
		return "NULL"
	}() + `);`
	if e := s.db.Exec(q); e != nil {
		s.json(w, 500, map[string]any{"error": e.Error()})
		return
	}
	id, _ := s.db.QueryText(`SELECT CAST(last_insert_rowid() AS TEXT);`)
	for _, bp := range []string{"desktop", "tablet", "mobile"} {
		ww := 3
		if bp != "desktop" {
			ww = 4
		}
		_ = s.db.Exec(`INSERT OR IGNORE INTO widget_layouts(widget_id,breakpoint,x,y,width,height) VALUES(` + id + `,` + database.Quote(bp) + `,0,0,` + strconv.Itoa(ww) + `,2);`)
	}
	s.json(w, 201, map[string]any{"ok": true, "id": id})
}
func (s *Server) widgetAction(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/widgets/"), "/"), "/")
	if len(parts) < 1 {
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}
	switch {
	case r.Method == http.MethodDelete:
		appID, _ := s.db.QueryText(`SELECT CASE WHEN widget_type='application.shortcut' THEN COALESCE(json_extract(config,'$.application_id'),'') ELSE '' END FROM widgets WHERE id=`+strconv.FormatInt(id,10)+`;`)
		err = s.db.Exec(`DELETE FROM widgets WHERE id=` + strconv.FormatInt(id, 10) + `;`)
		if err == nil && appID != "" {
			refs, _ := s.db.QueryText(`SELECT COUNT(*) FROM widgets WHERE widget_type='application.shortcut' AND json_extract(config,'$.application_id')=`+appID+`;`)
			if refs == "0" {
				sourceID, _ := s.db.QueryText(`SELECT COALESCE(source_id,'') FROM applications WHERE id=`+appID+`;`)
				_ = s.db.Exec(`DELETE FROM applications WHERE id=`+appID+`;`)
				if sourceID != "" { _ = s.db.Exec(`UPDATE docker_services SET dashboard_status='AVAILABLE',updated_at=CURRENT_TIMESTAMP WHERE stable_key=`+database.Quote(sourceID)+`;`) }
			}
		}
	case r.Method == http.MethodPost && action == "duplicate":
		err = s.db.Exec(`INSERT INTO widgets(page_id,widget_type,provider_type,title,subtitle,enabled,refresh_mode,refresh_interval,visibility,style_override,config,parent_widget_id) SELECT page_id,widget_type,provider_type,title||' copia',subtitle,enabled,refresh_mode,refresh_interval,visibility,style_override,config,parent_widget_id FROM widgets WHERE id=` + strconv.FormatInt(id, 10) + `;`)
		if err == nil {
			newID, _ := s.db.QueryText(`SELECT CAST(last_insert_rowid() AS TEXT);`)
			err = s.db.Exec(`INSERT INTO widget_layouts(widget_id,breakpoint,x,y,width,height,min_width,min_height,max_width,max_height) SELECT `+newID+`,breakpoint,x,y+1,width,height,min_width,min_height,max_width,max_height FROM widget_layouts WHERE widget_id=`+strconv.FormatInt(id,10)+`;`)
		}
	case r.Method == http.MethodPost && (action == "hide" || action == "show"):
		val := "0"
		if action == "show" {
			val = "1"
		}
		err = s.db.Exec(`UPDATE widgets SET enabled=` + val + `,updated_at=CURRENT_TIMESTAMP WHERE id=` + strconv.FormatInt(id, 10) + `;`)
	case r.Method == http.MethodPut:
		var in struct {
			Title, Subtitle        string
			PageID, ParentWidgetID int64
			Config                 map[string]any
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&in) != nil {
			err = fmt.Errorf("invalid json")
		} else {
			sets := []string{"updated_at=CURRENT_TIMESTAMP"}
			if in.Title != "" {
				sets = append(sets, "title="+database.Quote(in.Title))
			}
			sets = append(sets, "subtitle="+database.Quote(in.Subtitle))
			if in.PageID > 0 {
				sets = append(sets, "page_id="+strconv.FormatInt(in.PageID, 10))
			}
			if in.ParentWidgetID > 0 {
				sets = append(sets, "parent_widget_id="+strconv.FormatInt(in.ParentWidgetID, 10))
			} else {
				sets = append(sets, "parent_widget_id=NULL")
			}
			if in.Config != nil {
				if b, e := json.Marshal(in.Config); e == nil {
					sets = append(sets, "config="+database.Quote(string(b)))
				}
			}
			err = s.db.Exec(`UPDATE widgets SET ` + strings.Join(sets, ",") + ` WHERE id=` + strconv.FormatInt(id, 10) + `;`)
		}
	default:
		http.Error(w, "method not allowed", 405)
		return
	}
	if err != nil {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	s.json(w, 200, map[string]any{"ok": true})
}

func (s *Server) monitors(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		raw, err := s.db.QueryText(`SELECT COALESCE(json_group_array(json_object('id',id,'name',name,'type',monitor_type,'target',target,'port',COALESCE(port,0),'interval_seconds',interval_seconds,'timeout_seconds',timeout_seconds,'enabled',enabled,'status',COALESCE(last_status,'UNKNOWN'),'latency_ms',COALESCE(last_latency_ms,0),'checked_at',COALESCE(last_checked_at,''),'error',COALESCE(last_error,''),'failures',consecutive_failures,'next_check_at',COALESCE(next_check_at,''),'http_status',COALESCE(last_http_status,0))),'[]') FROM monitors ORDER BY id;`)
		if err != nil {
			s.json(w, 500, map[string]any{"error": err.Error()})
			return
		}
		var v any
		_ = json.Unmarshal([]byte(raw), &v)
		s.json(w, 200, map[string]any{"monitors": v})
	case http.MethodPost:
		var in struct {
			Name, Type, Target                    string
			Port, IntervalSeconds, TimeoutSeconds int
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&in) != nil || strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Target) == "" || (in.Type != "http" && in.Type != "https" && in.Type != "tcp") {
			s.json(w, 400, map[string]any{"error": "invalid monitor"})
			return
		}
		if in.IntervalSeconds < 5 {
			in.IntervalSeconds = 30
		}
		if in.TimeoutSeconds < 1 || in.TimeoutSeconds > 30 {
			in.TimeoutSeconds = 5
		}
		if in.Type == "tcp" && (in.Port < 0 || in.Port > 65535) {
			s.json(w, 400, map[string]any{"error": "invalid port"})
			return
		}
		q := `INSERT INTO monitors(name,monitor_type,target,port,interval_seconds,timeout_seconds,enabled,last_status,next_check_at) VALUES(` + database.Quote(in.Name) + `,` + database.Quote(in.Type) + `,` + database.Quote(in.Target) + `,` + strconv.Itoa(in.Port) + `,` + strconv.Itoa(in.IntervalSeconds) + `,` + strconv.Itoa(in.TimeoutSeconds) + `,1,'UNKNOWN',CURRENT_TIMESTAMP);`
		if err := s.db.Exec(q); err != nil {
			s.json(w, 500, map[string]any{"error": err.Error()})
			return
		}
		id, _ := s.db.QueryText(`SELECT CAST(last_insert_rowid() AS TEXT);`)
		s.json(w, 201, map[string]any{"ok": true, "id": id})
	default:
		http.Error(w, "method not allowed", 405)
	}
}

func (s *Server) runtimePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	slug := strings.TrimPrefix(r.URL.Path, "/api/v1/runtime/page/")
	if slug == "" {
		http.NotFound(w, r)
		return
	}
	bp := r.URL.Query().Get("breakpoint")
	if bp != "tablet" && bp != "mobile" {
		bp = "desktop"
	}
	pageJSON, err := s.db.QueryText(`SELECT json_object('id',id,'name',name,'slug',slug) FROM pages WHERE enabled=1 AND slug=` + database.Quote(slug) + ` LIMIT 1;`)
	if err != nil || pageJSON == "" {
		http.NotFound(w, r)
		return
	}
	q := `SELECT COALESCE(json_group_array(json_object('id',w.id,'type',w.widget_type,'provider',COALESCE(w.provider_type,''),'title',COALESCE(w.title,''),'subtitle',COALESCE(w.subtitle,''),'config',json(w.config),'parent_widget_id',COALESCE(w.parent_widget_id,0),'layout',json_object('x',COALESCE(l.x,0),'y',COALESCE(l.y,0),'w',COALESCE(l.width,3),'h',COALESCE(l.height,2)))), '[]') FROM widgets w JOIN pages p ON p.id=w.page_id LEFT JOIN widget_layouts l ON l.widget_id=w.id AND l.breakpoint=` + database.Quote(bp) + ` WHERE p.slug=` + database.Quote(slug) + ` AND w.enabled=1;`
	wj, err := s.db.QueryText(q)
	if err != nil {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	var page any
	var ws []map[string]any
	_ = json.Unmarshal([]byte(pageJSON), &page)
	_ = json.Unmarshal([]byte(wj), &ws)
	for _, x := range ws {
		if x["type"] == "application.shortcut" {
			if cfg, ok := x["config"].(map[string]any); ok {
				if aid, ok := cfg["application_id"].(float64); ok {
					aj, _ := s.db.QueryText(`SELECT json_object('id',id,'name',name,'url',url,'description',COALESCE(description,''),'open_mode',open_mode,'monitor_id',COALESCE(monitor_id,0),'icon_type',icon_type,'icon_value',COALESCE(icon_value,''),'source_type',source_type) FROM applications WHERE enabled=1 AND id=` + strconv.FormatInt(int64(aid), 10) + ` LIMIT 1;`)
					var app any
					if aj != "" {
						_ = json.Unmarshal([]byte(aj), &app)
						x["application"] = app
						if am, ok := app.(map[string]any); ok {
							if mid, ok := am["monitor_id"].(float64); ok && mid > 0 {
								mj, _ := s.db.QueryText(`SELECT json_object('id',id,'status',COALESCE(last_status,'UNKNOWN'),'latency_ms',COALESCE(last_latency_ms,0),'checked_at',COALESCE(last_checked_at,''),'error',COALESCE(last_error,''),'http_status',COALESCE(last_http_status,0)) FROM monitors WHERE id=` + strconv.FormatInt(int64(mid), 10) + ` LIMIT 1;`)
								if mj != "" {
									var mv any
									_ = json.Unmarshal([]byte(mj), &mv)
									x["monitor"] = mv
								}
							}
						}
					}
				}
			}
		}
		if key, _ := x["provider"].(string); key != "" {
			if res, e := s.providers.Get(r.Context(), key); e == nil {
				x["data"] = res
			} else {
				x["error"] = e.Error()
			}
		}
	}
	s.json(w, 200, map[string]any{"page": page, "breakpoint": bp, "widgets": ws})
}

type layoutInput struct {
	WidgetID   int64  `json:"widget_id"`
	Breakpoint string `json:"breakpoint"`
	X          int    `json:"x"`
	Y          int    `json:"y"`
	W          int    `json:"w"`
	H          int    `json:"h"`
}

func (s *Server) layouts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", 405)
		return
	}
	var in []layoutInput
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	if dec.Decode(&in) != nil {
		s.json(w, 400, map[string]any{"error": "invalid json"})
		return
	}
	if len(in) > 200 {
		s.json(w, 400, map[string]any{"error": "too many layouts"})
		return
	}
	if err := s.db.Exec("BEGIN IMMEDIATE;"); err != nil {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	ok := false
	defer func() {
		if !ok {
			_ = s.db.Exec("ROLLBACK;")
		}
	}()
	for _, v := range in {
		if v.Breakpoint != "desktop" && v.Breakpoint != "tablet" && v.Breakpoint != "mobile" {
			s.json(w, 400, map[string]any{"error": "invalid breakpoint"})
			return
		}
		if v.W < 1 || v.H < 1 || v.X < 0 || v.Y < 0 {
			s.json(w, 400, map[string]any{"error": "invalid geometry"})
			return
		}
		q := `INSERT INTO widget_layouts(widget_id,breakpoint,x,y,width,height) VALUES(` + strconv.FormatInt(v.WidgetID, 10) + `,` + database.Quote(v.Breakpoint) + `,` + strconv.Itoa(v.X) + `,` + strconv.Itoa(v.Y) + `,` + strconv.Itoa(v.W) + `,` + strconv.Itoa(v.H) + `) ON CONFLICT(widget_id,breakpoint) DO UPDATE SET x=excluded.x,y=excluded.y,width=excluded.width,height=excluded.height;`
		if err := s.db.Exec(q); err != nil {
			s.json(w, 500, map[string]any{"error": err.Error()})
			return
		}
	}
	if err := s.db.Exec("COMMIT;"); err != nil {
		s.json(w, 500, map[string]any{"error": err.Error()})
		return
	}
	ok = true
	s.json(w, 200, map[string]any{"ok": true, "updated": len(in)})
}

func (s *Server) dockerStatus(w http.ResponseWriter, r *http.Request) {
	if s.docker == nil {
		s.json(w, 200, map[string]any{"enabled": false, "available": false})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	err := s.docker.Ping(ctx)
	if err != nil {
		s.json(w, 200, map[string]any{"enabled": true, "available": false, "error": err.Error()})
		return
	}
	s.json(w, 200, map[string]any{"enabled": true, "available": true, "mode": "read-only-proxy"})
}
func (s *Server) dockerContainers(w http.ResponseWriter, r *http.Request) {
	if s.docker == nil {
		s.json(w, 503, map[string]any{"error": "docker integration disabled"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	v, err := s.docker.Containers(ctx)
	if err != nil {
		s.json(w, 502, map[string]any{"error": err.Error()})
		return
	}
	s.json(w, 200, map[string]any{"containers": v, "count": len(v)})
}

func (s *Server) dockerDiscovery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	if s.docker == nil {
		s.json(w, 503, map[string]any{"error": "docker integration disabled"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	cc, err := s.docker.Containers(ctx)
	if err != nil {
		s.json(w, 502, map[string]any{"error": err.Error()})
		return
	}
	host := r.Host
	out := make([]discovery.Service, 0, len(cc))
	for _, c := range cc {
		d := discovery.Normalize(c, host)
		out = append(out, d)
		ports, _ := json.Marshal(d.Ports)
		labels, _ := json.Marshal(d.Labels)
		q := `INSERT INTO docker_services(stable_key,service_name,container_name,container_id,image,state,health,ports,labels,suggested_url,suggested_icon,dashboard_status,last_seen_at,updated_at) VALUES(` + database.Quote(d.StableKey) + `,` + database.Quote(d.SuggestedName) + `,` + database.Quote(d.ContainerName) + `,` + database.Quote(d.ContainerID) + `,` + database.Quote(d.Image) + `,` + database.Quote(d.State) + `,` + database.Quote(d.Health) + `,` + database.Quote(string(ports)) + `,` + database.Quote(string(labels)) + `,` + database.Quote(d.SuggestedURL) + `,` + database.Quote(d.SuggestedIcon) + `,'AVAILABLE',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP) ON CONFLICT(stable_key) DO UPDATE SET service_name=excluded.service_name,container_name=excluded.container_name,container_id=excluded.container_id,image=excluded.image,state=excluded.state,health=excluded.health,ports=excluded.ports,labels=excluded.labels,suggested_url=excluded.suggested_url,suggested_icon=excluded.suggested_icon,last_seen_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP;`
		_ = s.db.Exec(q)
	}
	s.json(w, 200, map[string]any{"services": out, "count": len(out)})
}
func (s *Server) discoveryAdopt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var in struct {
		StableKey string `json:"stable_key"`
		PageID    int64  `json:"page_id"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&in) != nil || in.StableKey == "" || in.PageID < 1 {
		s.json(w, 400, map[string]any{"error": "invalid adoption"})
		return
	}
	raw, e := s.db.QueryText(`SELECT COALESCE(json_object('name',service_name,'url',COALESCE(suggested_url,''),'icon',COALESCE(suggested_icon,''),'image',COALESCE(image,'')),'') FROM docker_services WHERE stable_key=` + database.Quote(in.StableKey) + `;`)
	if e != nil || raw == "" {
		s.json(w, 404, map[string]any{"error": "service not found"})
		return
	}
	var d struct {
		Name  string `json:"name"`
		URL   string `json:"url"`
		Icon  string `json:"icon"`
		Image string `json:"image"`
	}
	_ = json.Unmarshal([]byte(raw), &d)
	if !discovery.ValidSuggestedURL(d.URL) {
		s.json(w, 409, map[string]any{"error": "service has no usable published URL"})
		return
	}
	source := "docker:" + in.StableKey
	appID, _ := s.db.QueryText(`SELECT COALESCE(CAST(id AS TEXT),'') FROM applications WHERE source_type='docker' AND source_id=` + database.Quote(in.StableKey) + ` LIMIT 1;`)
	if appID == "" {
		if e := s.db.Exec(`INSERT INTO applications(name,url,icon_type,icon_value,source_type,source_id) VALUES(` + database.Quote(d.Name) + `,` + database.Quote(d.URL) + `,'auto',` + database.Quote(firstNonEmptyString(d.Icon, d.Image)) + `,'docker',` + database.Quote(in.StableKey) + `);`); e != nil {
			s.json(w, 500, map[string]any{"error": e.Error()})
			return
		}
		appID, _ = s.db.QueryText(`SELECT CAST(last_insert_rowid() AS TEXT);`)
	}
	monitorID, _ := s.db.QueryText(`SELECT COALESCE(CAST(monitor_id AS TEXT),'') FROM applications WHERE id=` + appID + ` LIMIT 1;`)
	if monitorID == "" || monitorID == "0" {
		_ = s.db.Exec(`INSERT INTO monitors(name,monitor_type,target,interval_seconds,timeout_seconds,enabled,last_status,next_check_at) VALUES(` + database.Quote(d.Name) + `,'http',` + database.Quote(d.URL) + `,30,5,1,'UNKNOWN',CURRENT_TIMESTAMP);`)
		monitorID, _ = s.db.QueryText(`SELECT CAST(last_insert_rowid() AS TEXT);`)
		_ = s.db.Exec(`UPDATE applications SET monitor_id=` + monitorID + `,updated_at=CURRENT_TIMESTAMP WHERE id=` + appID + `;`)
	}
	exists, _ := s.db.QueryText(`SELECT COALESCE(CAST(id AS TEXT),'') FROM widgets WHERE page_id=` + strconv.FormatInt(in.PageID, 10) + ` AND widget_type='application.shortcut' AND config LIKE ` + database.Quote(`%"application_id":`+appID+`%`) + ` LIMIT 1;`)
	if exists == "" {
		_ = s.db.Exec(`INSERT INTO widgets(page_id,widget_type,provider_type,title,config) VALUES(` + strconv.FormatInt(in.PageID, 10) + `,'application.shortcut','',` + database.Quote(d.Name) + `,` + database.Quote(`{"application_id":`+appID+`}`) + `);`)
		wid, _ := s.db.QueryText(`SELECT CAST(last_insert_rowid() AS TEXT);`)
		for _, bp := range []string{"desktop", "tablet", "mobile"} {
			ww := 3
			if bp != "desktop" {
				ww = 4
			}
			_ = s.db.Exec(`INSERT OR IGNORE INTO widget_layouts(widget_id,breakpoint,x,y,width,height) VALUES(` + wid + `,` + database.Quote(bp) + `,0,0,` + strconv.Itoa(ww) + `,2);`)
		}
		exists = wid
	}
	_ = s.db.Exec(`UPDATE docker_services SET dashboard_status='ADDED',updated_at=CURRENT_TIMESTAMP WHERE stable_key=` + database.Quote(in.StableKey) + `;`)
	s.json(w, 200, map[string]any{"ok": true, "application_id": appID, "widget_id": exists, "monitor_id": monitorID, "source": source})
}
func (s *Server) discoveryIgnore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var in struct {
		StableKey string `json:"stable_key"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&in) != nil || in.StableKey == "" {
		s.json(w, 400, map[string]any{"error": "invalid service"})
		return
	}
	if e := s.db.Exec(`UPDATE docker_services SET dashboard_status='IGNORED',updated_at=CURRENT_TIMESTAMP WHERE stable_key=` + database.Quote(in.StableKey) + `;`); e != nil {
		s.json(w, 500, map[string]any{"error": e.Error()})
		return
	}
	s.json(w, 200, map[string]any{"ok": true})
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/page/inicio" {
		http.NotFound(w, r)
		return
	}
	b, e := fs.ReadFile(s.assets, "index.html")
	if e != nil {
		http.Error(w, "frontend unavailable", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) { w.status = code; w.ResponseWriter.WriteHeader(code) }
func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		s.requests.Add(1)
		s.active.Add(1)
		defer s.active.Add(-1)
		sw := &statusWriter{ResponseWriter: w, status: 200}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: https://cdn.jsdelivr.net; style-src 'self'; script-src 'self'; connect-src 'self'")
		next.ServeHTTP(sw, r)
		if sw.status >= 500 {
			s.errors.Add(1)
		}
		if !strings.HasPrefix(r.URL.Path, "/assets/") {
			s.log.Debug("request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
		}
	})
}

var _ = fmt.Sprintf

func (s *Server) themePresets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	s.json(w, 200, map[string]any{"presets": themes.Presets()})
}
func (s *Server) themeAPI(w http.ResponseWriter, r *http.Request) {
	key := "theme.global"
	if pg := strings.TrimSpace(r.URL.Query().Get("page")); pg != "" {
		key = "theme.page." + slugify(pg)
	}
	if r.Method == http.MethodGet {
		v, _ := s.db.QueryText(`SELECT value FROM settings WHERE key=` + database.Quote(key) + `;`)
		if v == "" && key != "theme.global" {
			v, _ = s.db.QueryText(`SELECT value FROM settings WHERE key='theme.global';`)
		}
		if v == "" {
			v = themes.Encode(themes.Presets()["sidinet-dark"])
		}
		var out any
		if json.Unmarshal([]byte(v), &out) != nil {
			s.json(w, 500, map[string]any{"error": "invalid stored theme"})
			return
		}
		s.json(w, 200, map[string]any{"theme": out, "scope": key})
		return
	}
	if r.Method == http.MethodPut {
		var in struct {
			Name   string            `json:"name"`
			Values map[string]string `json:"values"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&in) != nil {
			s.json(w, 400, map[string]any{"error": "invalid theme"})
			return
		}
		vals, e := themes.Sanitize(in.Values)
		if e != nil {
			s.json(w, 400, map[string]any{"error": e.Error()})
			return
		}
		if strings.TrimSpace(in.Name) == "" {
			in.Name = "Custom"
		}
		raw := themes.Encode(themes.Theme{Name: in.Name, Values: vals})
		q := `INSERT INTO settings(key,value,type,updated_at) VALUES(` + database.Quote(key) + `,` + database.Quote(raw) + `,'json',CURRENT_TIMESTAMP) ON CONFLICT(key) DO UPDATE SET value=excluded.value,type='json',updated_at=CURRENT_TIMESTAMP;`
		if e = s.db.Exec(q); e != nil {
			s.json(w, 500, map[string]any{"error": e.Error()})
			return
		}
		s.json(w, 200, map[string]any{"ok": true, "theme": themes.Theme{Name: in.Name, Values: vals}})
		return
	}
	http.Error(w, "method not allowed", 405)
}

func (s *Server) recoveryStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	st := map[string]any{"safe_mode": s.cfg.SafeMode, "database": "ok", "data_dir": s.cfg.DataDir, "version": version.Version, "schema": version.SchemaVersion}
	if e := s.db.Ping(); e != nil {
		st["database"] = e.Error()
	}
	s.json(w, 200, st)
}
func (s *Server) recoveryBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	p, e := recovery.Create(s.db, s.cfg.DataDir, version.Version, version.SchemaVersion)
	if e != nil {
		s.json(w, 500, map[string]any{"error": e.Error()})
		return
	}
	s.json(w, 201, map[string]any{"status": "created", "file": filepath.Base(p)})
}
func (s *Server) recoveryRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, recovery.MaxArchive+1)
	_, m, e := recovery.Stage(r.Body, s.cfg.DataDir)
	if e != nil {
		s.json(w, 400, map[string]any{"error": e.Error()})
		return
	}
	s.json(w, 202, map[string]any{"status": "staged", "restart_required": true, "backup_version": m.Version, "backup_schema": m.Schema})
}
func (s *Server) recoveryDiagnostics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	dbs, _ := os.Stat(s.cfg.DatabasePath)
	ups := int64(0)
	_ = filepath.Walk(filepath.Join(s.cfg.DataDir, "uploads"), func(_ string, i os.FileInfo, e error) error {
		if e == nil && i != nil && i.Mode().IsRegular() {
			ups += i.Size()
		}
		return nil
	})
	mon, _ := s.db.QueryText(`SELECT COUNT(*) FROM monitors;`)
	svc, _ := s.db.QueryText(`SELECT COUNT(*) FROM docker_services;`)
	s.json(w, 200, map[string]any{"version": version.Version, "schema": version.SchemaVersion, "safe_mode": s.cfg.SafeMode, "database_bytes": func() int64 {
		if dbs != nil {
			return dbs.Size()
		}
		return 0
	}(), "uploads_bytes": ups, "monitors": mon, "docker_services": svc, "docker_enabled": s.cfg.DockerEnabled && !s.cfg.SafeMode, "storage_paths": s.cfg.StoragePaths})
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" { return value }
	}
	return ""
}
