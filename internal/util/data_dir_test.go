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

func TestMacDefaultDataDirIsSharedByCLIAndApp(t *testing.T) {
	t.Chdir(t.TempDir())
	config := filepath.Join(t.TempDir(), "Application Support")
	if got := DefaultDataDirFor("darwin", "", config, "/Applications/Biz Studio.app/Contents/MacOS/bizstudio"); got != filepath.Join(config, "BizStudio") {
		t.Fatalf("Mac default data dir = %q; app and CLI must share Application Support", got)
	}
}

func TestMacDefaultDataDirPreservesExistingWorkingDirectoryData(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("data", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("data", "db.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := DefaultDataDirFor("darwin", "", t.TempDir(), ""); got != "data" {
		t.Fatalf("lost existing CLI data: %q", got)
	}
}
