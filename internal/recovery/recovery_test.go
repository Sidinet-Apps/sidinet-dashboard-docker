package recovery

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestRejectZipSlip(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "x.zip")
	f, _ := os.Create(p)
	z := zip.NewWriter(f)
	w, _ := z.Create("../evil")
	w.Write([]byte("x"))
	z.Close()
	f.Close()
	if _, e := Validate(p); e == nil {
		t.Fatal("expected unsafe path rejection")
	}
}
