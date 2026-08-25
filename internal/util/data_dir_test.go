package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsDefaultDataDirUsesLocalAppData(t *testing.T) {
	base := filepath.Join(t.TempDir(), "Local")
	got := DefaultDataDirFor("windows", base, "", filepath.Join(t.TempDir(), "Biz Studio.exe"))
	want := filepath.Join(base, "BizStudio")
	if got != want {
		t.Fatalf("default data dir = %q, muốn %q", got, want)
	}
}

func TestWindowsDefaultDataDirPreservesPortableData(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "data")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "db.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := DefaultDataDirFor("windows", filepath.Join(dir, "Local"), "", filepath.Join(dir, "Biz Studio.exe"))
	if got != legacy {
		t.Fatalf("default data dir = %q, muốn giữ %q", got, legacy)
	}
}

func TestDataDirIDIsStableAndDistinct(t *testing.T) {
	a := DataDirID(filepath.Join(t.TempDir(), "a"))
	b := DataDirID(filepath.Join(t.TempDir(), "b"))
	if a == "" || b == "" || a == b {
		t.Fatalf("data IDs không hợp lệ: %q %q", a, b)
	}
}
