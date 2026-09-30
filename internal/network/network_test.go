package network

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestParseDev(t *testing.T) {
	x, e := ParseDev("Inter-| Receive | Transmit\n eth0: 100 1 0 0 0 0 0 0 200 2 0 0 0 0 0 0\n")
	if e != nil || len(x) != 1 || x[0].RXBytes != 100 || x[0].TXBytes != 200 {
		t.Fatalf("bad parse: %#v %v", x, e)
	}
}

func TestIPsIncludesIPv6(t *testing.T) {
	d := t.TempDir()
	if err := os.MkdirAll(filepath.Join(d, "net"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "net", "fib_trie"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	line := "20010db8000000000000000000000001 02 40 00 80 eth0\n00000000000000000000000000000001 01 80 10 80 lo\n"
	if err := os.WriteFile(filepath.Join(d, "net", "if_inet6"), []byte(line), 0600); err != nil {
		t.Fatal(err)
	}
	v, err := NewIPs(d).Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := v.(map[string]any)["addresses"].([]string)
	if len(got) != 1 || got[0] != "2001:db8::1" {
		t.Fatalf("addresses=%v", got)
	}
}
