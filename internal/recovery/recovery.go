package recovery

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sidinet/sidinet-dashboard-docker/internal/database"
)

const MaxArchive = int64(256 << 20)
const MaxEntries = 2048
const MaxFile = int64(64 << 20)
const MaxExpanded = int64(512 << 20)

type Manifest struct {
	Format    int               `json:"format"`
	Product   string            `json:"product"`
	Version   string            `json:"version"`
	Schema    string            `json:"schema"`
	CreatedAt string            `json:"created_at"`
	Files     map[string]string `json:"files"`
}

func sha(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	_, e = io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), e
}

func Create(db *database.DB, dataDir, version, schema string) (string, error) {
	dir := filepath.Join(dataDir, "backups")
	if e := os.MkdirAll(dir, 0750); e != nil {
		return "", e
	}
	stamp := time.Now().UTC().Format("20060102T150405Z")
	tmp := filepath.Join(dir, ".backup-"+stamp+".sqlite")
	if e := db.BackupTo(tmp); e != nil {
		return "", e
	}
	defer os.Remove(tmp)
	out := filepath.Join(dir, "sidinet-backup-"+stamp+".zip")
	f, e := os.Create(out)
	if e != nil {
		return "", e
	}
	zw := zip.NewWriter(f)
	files := map[string]string{}
	add := func(src, name string) error {
		sum, e := sha(src)
		if e != nil {
			return e
		}
		files[name] = sum
		w, e := zw.Create(name)
		if e != nil {
			return e
		}
		r, e := os.Open(src)
		if e != nil {
			return e
		}
		defer r.Close()
		_, e = io.Copy(w, r)
		return e
	}
	if e = add(tmp, "database.sqlite"); e != nil {
		zw.Close()
		f.Close()
		return "", e
	}
	up := filepath.Join(dataDir, "uploads")
	_ = filepath.Walk(up, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || !info.Mode().IsRegular() {
			return nil
		}
		rel, _ := filepath.Rel(up, p)
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "../") {
			return nil
		}
		return add(p, "uploads/"+rel)
	})
	m := Manifest{Format: 1, Product: "SIDINET Dashboard", Version: version, Schema: schema, CreatedAt: time.Now().UTC().Format(time.RFC3339), Files: files}
	b, _ := json.MarshalIndent(m, "", "  ")
	w, _ := zw.Create("manifest.json")
	_, _ = w.Write(b)
	if e = zw.Close(); e != nil {
		f.Close()
		return "", e
	}
	if e = f.Close(); e != nil {
		return "", e
	}
	return out, nil
}

func Validate(path string) (Manifest, error) {
	st, e := os.Stat(path)
	if e != nil {
		return Manifest{}, e
	}
	if st.Size() > MaxArchive {
		return Manifest{}, errors.New("backup archive too large")
	}
	zr, e := zip.OpenReader(path)
	if e != nil {
		return Manifest{}, e
	}
	defer zr.Close()
	if len(zr.File) > MaxEntries {
		return Manifest{}, errors.New("too many archive entries")
	}
	var m Manifest
	var total int64
	for _, z := range zr.File {
		n := filepath.ToSlash(z.Name)
		if n == "" || strings.HasPrefix(n, "/") || strings.Contains(n, "../") || strings.Contains(n, "\\") {
			return m, errors.New("unsafe archive path")
		}
		if z.FileInfo().Mode()&os.ModeSymlink != 0 {
			return m, errors.New("symlinks not allowed")
		}
		if int64(z.UncompressedSize64) > MaxFile {
			return m, errors.New("archive member too large")
		}
		total += int64(z.UncompressedSize64)
		if total > MaxExpanded {
			return m, errors.New("archive expands too large")
		}
		if n == "manifest.json" {
			r, _ := z.Open()
			e = json.NewDecoder(io.LimitReader(r, 1<<20)).Decode(&m)
			r.Close()
			if e != nil {
				return m, e
			}
		}
	}
	if m.Format != 1 || m.Product != "SIDINET Dashboard" || m.Files == nil {
		return m, errors.New("invalid or missing manifest")
	}
	if _, ok := m.Files["database.sqlite"]; !ok {
		return m, errors.New("database missing from manifest")
	}
	for name, want := range m.Files {
		var zf *zip.File
		for _, z := range zr.File {
			if filepath.ToSlash(z.Name) == name {
				zf = z
				break
			}
		}
		if zf == nil {
			return m, fmt.Errorf("missing %s", name)
		}
		r, _ := zf.Open()
		h := sha256.New()
		_, e = io.Copy(h, io.LimitReader(r, MaxFile+1))
		r.Close()
		if e != nil {
			return m, e
		}
		if hex.EncodeToString(h.Sum(nil)) != want {
			return m, fmt.Errorf("checksum mismatch: %s", name)
		}
	}
	return m, nil
}

func Stage(upload io.Reader, dataDir string) (string, Manifest, error) {
	dir := filepath.Join(dataDir, "restore")
	if e := os.MkdirAll(dir, 0750); e != nil {
		return "", Manifest{}, e
	}
	p := filepath.Join(dir, "pending.zip")
	f, e := os.Create(p)
	if e != nil {
		return "", Manifest{}, e
	}
	n, e := io.Copy(f, io.LimitReader(upload, MaxArchive+1))
	f.Close()
	if e != nil || n > MaxArchive {
		os.Remove(p)
		return "", Manifest{}, errors.New("restore upload too large or invalid")
	}
	m, e := Validate(p)
	if e != nil {
		os.Remove(p)
		return "", m, e
	}
	return p, m, nil
}

func ApplyPending(dataDir string) error {
	p := filepath.Join(dataDir, "restore", "pending.zip")
	if _, e := os.Stat(p); os.IsNotExist(e) {
		return nil
	}
	_, e := Validate(p)
	if e != nil {
		return e
	}
	zr, e := zip.OpenReader(p)
	if e != nil {
		return e
	}
	defer zr.Close()
	stage := filepath.Join(dataDir, "restore", "staging")
	os.RemoveAll(stage)
	if e = os.MkdirAll(stage, 0750); e != nil {
		return e
	}
	for _, z := range zr.File {
		n := filepath.ToSlash(z.Name)
		if n == "manifest.json" {
			continue
		}
		dst := filepath.Join(stage, filepath.FromSlash(n))
		if !strings.HasPrefix(filepath.Clean(dst), filepath.Clean(stage)+string(os.PathSeparator)) {
			return errors.New("unsafe restore path")
		}
		if e = os.MkdirAll(filepath.Dir(dst), 0750); e != nil {
			return e
		}
		r, _ := z.Open()
		w, e := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if e != nil {
			r.Close()
			return e
		}
		_, e = io.Copy(w, r)
		w.Close()
		r.Close()
		if e != nil {
			return e
		}
	}
	// Validate staged SQLite before replacement.
	test, e := database.Open(filepath.Join(stage, "database.sqlite"))
	if e != nil {
		return e
	}
	e = test.Ping()
	test.Close()
	if e != nil {
		return e
	}
	db := filepath.Join(dataDir, "database.sqlite")
	rollback := filepath.Join(dataDir, "restore", "rollback.sqlite")
	_ = copyFile(db, rollback)
	for _, s := range []string{db + "-wal", db + "-shm"} {
		_ = os.Remove(s)
	}
	if e = copyFile(filepath.Join(stage, "database.sqlite"), db); e != nil {
		_ = copyFile(rollback, db)
		return e
	}
	if _, e = os.Stat(filepath.Join(stage, "uploads")); e == nil {
		old := filepath.Join(dataDir, "uploads")
		bak := filepath.Join(dataDir, "restore", "uploads.rollback")
		os.RemoveAll(bak)
		_ = os.Rename(old, bak)
		if e = os.Rename(filepath.Join(stage, "uploads"), old); e != nil {
			_ = os.Rename(bak, old)
			return e
		}
		os.RemoveAll(bak)
	}
	os.RemoveAll(stage)
	return os.Remove(p)
}
func copyFile(src, dst string) error {
	r, e := os.Open(src)
	if e != nil {
		return e
	}
	defer r.Close()
	if e = os.MkdirAll(filepath.Dir(dst), 0750); e != nil {
		return e
	}
	w, e := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	_, e = io.Copy(w, r)
	ce := w.Close()
	if e != nil {
		return e
	}
	return ce
}
