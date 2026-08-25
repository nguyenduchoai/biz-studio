package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDatabaseIsOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	if _, err := Open(dir); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "db.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("db.json mode = %o, muốn 600", got)
	}
}
