package updater

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractZipRejectsTraversal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("../outside.exe")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("bad"))
	_ = zw.Close()
	_ = f.Close()
	if err := extractZip(path, t.TempDir()); err == nil {
		t.Fatal("archive traversal phải bị từ chối")
	}
}

func TestReplaceFilesPreservesUnrelatedData(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "bizstudio"), []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dst, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "data", "db.json"), []byte("user data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := replaceFiles(src, dst); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "data", "db.json"))
	if err != nil || string(got) != "user data" {
		t.Fatalf("dữ liệu người dùng bị ảnh hưởng: %q, %v", got, err)
	}
}
