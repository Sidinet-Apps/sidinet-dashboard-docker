package database

/*
#cgo LDFLAGS: -lsqlite3
#include <sqlite3.h>
#include <stdlib.h>
*/
import "C"
import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"
)

type DB struct {
	ptr  *C.sqlite3
	path string
}

func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return nil, err
	}
	cp := C.CString(path)
	defer C.free(unsafe.Pointer(cp))
	var p *C.sqlite3
	if rc := C.sqlite3_open(cp, &p); rc != C.SQLITE_OK {
		if p != nil {
			C.sqlite3_close(p)
		}
		return nil, fmt.Errorf("sqlite open rc=%d", int(rc))
	}
	d := &DB{ptr: p, path: path}
	for _, q := range []string{"PRAGMA journal_mode=WAL;", "PRAGMA synchronous=NORMAL;", "PRAGMA foreign_keys=ON;", "PRAGMA busy_timeout=5000;", "PRAGMA wal_autocheckpoint=1000;", "PRAGMA temp_store=MEMORY;"} {
		if err := d.Exec(q); err != nil {
			d.Close()
			return nil, err
		}
	}
	return d, nil
}
func (d *DB) Close() error {
	if d == nil || d.ptr == nil {
		return nil
	}
	rc := C.sqlite3_close(d.ptr)
	d.ptr = nil
	if rc != C.SQLITE_OK {
		return fmt.Errorf("sqlite close rc=%d", int(rc))
	}
	return nil
}
func (d *DB) Exec(sql string) error {
	if d == nil || d.ptr == nil {
		return errors.New("database closed")
	}
	q := C.CString(sql)
	defer C.free(unsafe.Pointer(q))
	var msg *C.char
	rc := C.sqlite3_exec(d.ptr, q, nil, nil, &msg)
	if rc != C.SQLITE_OK {
		m := "sqlite error"
		if msg != nil {
			m = C.GoString(msg)
			C.sqlite3_free(unsafe.Pointer(msg))
		}
		return fmt.Errorf("%s", m)
	}
	return nil
}
func (d *DB) Ping() error  { return d.Exec("SELECT 1;") }
func (d *DB) Path() string { return d.path }
func (d *DB) Migrate() error {
	if err := d.Exec(schema); err != nil {
		return err
	}
	// Additive v0.7 migration for existing installations. Duplicate-column errors mean it was already applied.
	for _, q := range []string{
		"ALTER TABLE monitors ADD COLUMN next_check_at TEXT;",
		"ALTER TABLE monitors ADD COLUMN last_http_status INTEGER;",
		"ALTER TABLE applications ADD COLUMN monitor_id INTEGER REFERENCES monitors(id) ON DELETE SET NULL;",
	} {
		if err := d.Exec(q); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return err
		}
	}
	if err := d.Exec("INSERT OR IGNORE INTO schema_migrations(version,name,applied_at) VALUES(7,'monitoring engine',CURRENT_TIMESTAMP);"); err != nil {
		return err
	}
	if err := d.Exec("INSERT OR IGNORE INTO schema_migrations(version,name,applied_at) VALUES(9,'theme engine',CURRENT_TIMESTAMP);"); err != nil {
		return err
	}
	if err := d.Exec("ALTER TABLE widgets ADD COLUMN parent_widget_id INTEGER REFERENCES widgets(id) ON DELETE SET NULL;"); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
		return err
	}
	if err := d.Exec("CREATE INDEX IF NOT EXISTS idx_widgets_parent ON widgets(parent_widget_id);"); err != nil {
		return err
	}
	return d.Exec("INSERT OR IGNORE INTO schema_migrations(version,name,applied_at) VALUES(12,'dashboard editor',CURRENT_TIMESTAMP);")
}

const schema = `
CREATE TABLE IF NOT EXISTS schema_migrations(version INTEGER PRIMARY KEY,name TEXT NOT NULL,applied_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS settings(key TEXT PRIMARY KEY,value TEXT,type TEXT NOT NULL DEFAULT 'string',updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS users(id INTEGER PRIMARY KEY AUTOINCREMENT,username TEXT NOT NULL COLLATE NOCASE UNIQUE,password_hash TEXT NOT NULL,display_name TEXT,role TEXT NOT NULL DEFAULT 'admin',enabled INTEGER NOT NULL DEFAULT 1,last_login_at TEXT,created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS pages(id INTEGER PRIMARY KEY AUTOINCREMENT,name TEXT NOT NULL,slug TEXT NOT NULL UNIQUE,icon TEXT,position INTEGER NOT NULL DEFAULT 0,enabled INTEGER NOT NULL DEFAULT 1,is_default INTEGER NOT NULL DEFAULT 0,background_mode TEXT NOT NULL DEFAULT 'inherit',revision INTEGER NOT NULL DEFAULT 1,created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS widgets(id INTEGER PRIMARY KEY AUTOINCREMENT,page_id INTEGER NOT NULL REFERENCES pages(id) ON DELETE CASCADE,widget_type TEXT NOT NULL,provider_type TEXT,title TEXT,subtitle TEXT,enabled INTEGER NOT NULL DEFAULT 1,refresh_mode TEXT NOT NULL DEFAULT 'profile',refresh_interval INTEGER,visibility TEXT NOT NULL DEFAULT 'all',style_override TEXT,config TEXT NOT NULL DEFAULT '{}',revision INTEGER NOT NULL DEFAULT 1,created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS widget_layouts(id INTEGER PRIMARY KEY AUTOINCREMENT,widget_id INTEGER NOT NULL REFERENCES widgets(id) ON DELETE CASCADE,breakpoint TEXT NOT NULL,x INTEGER NOT NULL,y INTEGER NOT NULL,width INTEGER NOT NULL,height INTEGER NOT NULL,min_width INTEGER,min_height INTEGER,max_width INTEGER,max_height INTEGER,UNIQUE(widget_id,breakpoint));
CREATE TABLE IF NOT EXISTS applications(id INTEGER PRIMARY KEY AUTOINCREMENT,name TEXT NOT NULL,description TEXT,url TEXT NOT NULL,icon_type TEXT NOT NULL DEFAULT 'auto',icon_value TEXT,source_type TEXT NOT NULL DEFAULT 'manual',source_id TEXT,open_mode TEXT NOT NULL DEFAULT 'new_tab',enabled INTEGER NOT NULL DEFAULT 1,monitor_id INTEGER REFERENCES monitors(id) ON DELETE SET NULL,created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS themes(id INTEGER PRIMARY KEY AUTOINCREMENT,name TEXT NOT NULL,slug TEXT NOT NULL UNIQUE,type TEXT NOT NULL DEFAULT 'custom',enabled INTEGER NOT NULL DEFAULT 1,revision INTEGER NOT NULL DEFAULT 1,created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS theme_settings(theme_id INTEGER NOT NULL REFERENCES themes(id) ON DELETE CASCADE,key TEXT NOT NULL,value TEXT,PRIMARY KEY(theme_id,key));
CREATE TABLE IF NOT EXISTS docker_services(id INTEGER PRIMARY KEY AUTOINCREMENT,stable_key TEXT NOT NULL UNIQUE,service_name TEXT NOT NULL,container_name TEXT,container_id TEXT,image TEXT,state TEXT,health TEXT,ports TEXT NOT NULL DEFAULT '[]',labels TEXT NOT NULL DEFAULT '{}',suggested_url TEXT,suggested_icon TEXT,dashboard_status TEXT NOT NULL DEFAULT 'unreviewed',discovered_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,last_seen_at TEXT,updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS monitors(id INTEGER PRIMARY KEY AUTOINCREMENT,name TEXT NOT NULL,monitor_type TEXT NOT NULL,target TEXT NOT NULL,port INTEGER,interval_seconds INTEGER NOT NULL DEFAULT 30,timeout_seconds INTEGER NOT NULL DEFAULT 5,enabled INTEGER NOT NULL DEFAULT 1,last_status TEXT,last_latency_ms INTEGER,last_checked_at TEXT,last_error TEXT,consecutive_failures INTEGER NOT NULL DEFAULT 0,created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,next_check_at TEXT,last_http_status INTEGER);
CREATE INDEX IF NOT EXISTS idx_widgets_page ON widgets(page_id,enabled);
CREATE INDEX IF NOT EXISTS idx_layout_breakpoint ON widget_layouts(breakpoint);
INSERT OR IGNORE INTO schema_migrations(version,name,applied_at) VALUES(1,'initial',CURRENT_TIMESTAMP);
INSERT OR IGNORE INTO widget_layouts(widget_id,breakpoint,x,y,width,height) SELECT id,'desktop',0,(id-1)*2,3,2 FROM widgets;
INSERT OR IGNORE INTO widget_layouts(widget_id,breakpoint,x,y,width,height) SELECT id,'tablet',0,(id-1)*2,4,2 FROM widgets;
INSERT OR IGNORE INTO widget_layouts(widget_id,breakpoint,x,y,width,height) SELECT id,'mobile',0,(id-1)*2,4,2 FROM widgets;
INSERT OR IGNORE INTO pages(id,name,slug,position,enabled,is_default) VALUES(1,'Inicio','inicio',0,1,1);
INSERT INTO widgets(page_id,widget_type,provider_type,title) SELECT 1,'system.cpu','system.cpu','CPU' WHERE NOT EXISTS(SELECT 1 FROM widgets);
INSERT INTO widgets(page_id,widget_type,provider_type,title) SELECT 1,'system.memory','system.memory','RAM' WHERE (SELECT COUNT(*) FROM widgets)=1;
INSERT INTO widgets(page_id,widget_type,provider_type,title) SELECT 1,'system.temperature','system.temperature','Temperatura' WHERE (SELECT COUNT(*) FROM widgets)=2;
INSERT INTO widgets(page_id,widget_type,provider_type,title) SELECT 1,'system.uptime','system.uptime','Uptime' WHERE (SELECT COUNT(*) FROM widgets)=3;
INSERT OR IGNORE INTO widget_layouts(widget_id,breakpoint,x,y,width,height) SELECT id,'desktop',((id-1)%4)*3,0,3,2 FROM widgets;
INSERT OR IGNORE INTO widget_layouts(widget_id,breakpoint,x,y,width,height) SELECT id,'tablet',((id-1)%2)*4,((id-1)/2)*2,4,2 FROM widgets;
INSERT OR IGNORE INTO widget_layouts(widget_id,breakpoint,x,y,width,height) SELECT id,'mobile',0,(id-1)*2,4,2 FROM widgets;
INSERT OR IGNORE INTO schema_migrations(version,name,applied_at) VALUES(2,'responsive layouts',CURRENT_TIMESTAMP);
`

func (d *DB) QueryText(sql string) (string, error) {
	if d == nil || d.ptr == nil {
		return "", errors.New("database closed")
	}
	q := C.CString(sql)
	defer C.free(unsafe.Pointer(q))
	var stmt *C.sqlite3_stmt
	if rc := C.sqlite3_prepare_v2(d.ptr, q, -1, &stmt, nil); rc != C.SQLITE_OK {
		return "", fmt.Errorf("sqlite prepare rc=%d", int(rc))
	}
	defer C.sqlite3_finalize(stmt)
	rc := C.sqlite3_step(stmt)
	if rc == C.SQLITE_DONE {
		return "", nil
	}
	if rc != C.SQLITE_ROW {
		return "", fmt.Errorf("sqlite step rc=%d", int(rc))
	}
	p := C.sqlite3_column_text(stmt, 0)
	if p == nil {
		return "", nil
	}
	return C.GoString((*C.char)(unsafe.Pointer(p))), nil
}

func Quote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func (d *DB) BackupTo(path string) error {
	if d == nil || d.ptr == nil {
		return errors.New("database closed")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}
	cp := C.CString(path)
	defer C.free(unsafe.Pointer(cp))
	var dst *C.sqlite3
	if rc := C.sqlite3_open(cp, &dst); rc != C.SQLITE_OK {
		if dst != nil {
			C.sqlite3_close(dst)
		}
		return fmt.Errorf("sqlite backup open rc=%d", int(rc))
	}
	defer C.sqlite3_close(dst)
	main := C.CString("main")
	defer C.free(unsafe.Pointer(main))
	b := C.sqlite3_backup_init(dst, main, d.ptr, main)
	if b == nil {
		return errors.New("sqlite backup init failed")
	}
	rc := C.sqlite3_backup_step(b, -1)
	frc := C.sqlite3_backup_finish(b)
	if rc != C.SQLITE_DONE || frc != C.SQLITE_OK {
		return fmt.Errorf("sqlite backup failed step=%d finish=%d", int(rc), int(frc))
	}
	return nil
}
