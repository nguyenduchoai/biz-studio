package updater

import (
	"archive/zip"
	"errors"
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

func TestReplaceFilesRollsBackEarlierFilesOnFailure(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	for _, dir := range []string{src, dst} {
		if err := os.WriteFile(filepath.Join(dir, "a.exe"), []byte(dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(src, "z.exe"), []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dst, "z.exe"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := replaceFiles(src, dst); err == nil {
		t.Fatal("replacing an unexpected directory must fail")
	}
	got, err := os.ReadFile(filepath.Join(dst, "a.exe"))
	if err != nil || string(got) != dst {
		t.Fatalf("earlier binary was not restored: %q, %v", got, err)
	}
}

func TestApplyRestoresOldVersionWhenRelaunchFails(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	for _, dir := range []string{src, dst} {
		if err := os.WriteFile(filepath.Join(dir, "bizstudio"), []byte(dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	tx, err := replaceFilesTransaction(src, dst)
	if err != nil {
		t.Fatal(err)
	}
	launches := 0
	err = finishUpdate(tx, Stage{}, func(Stage) error {
		launches++
		if launches == 1 {
			return errors.New("new executable cannot launch")
		}
		return nil
	})
	if err == nil || launches != 2 {
		t.Fatalf("must report update failure and relaunch restored app: %v, launches=%d", err, launches)
	}
	got, readErr := os.ReadFile(filepath.Join(dst, "bizstudio"))
	if readErr != nil || string(got) != dst {
		t.Fatalf("old executable not restored: %q, %v", got, readErr)
	}
}

func TestMacBundleRollbackPreservesOldAppAndData(t *testing.T) {
	src := filepath.Join(t.TempDir(), "Biz Studio.app")
	dst := filepath.Join(t.TempDir(), "Biz Studio.app")
	for _, root := range []string{src, dst} {
		for _, name := range []string{"Contents/MacOS/bizstudio", "Contents/MacOS/launcher", "Contents/Info.plist"} {
			path := filepath.Join(root, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(root), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	tx, err := replaceAppTransaction(src, dst)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.rollback(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "Contents", "MacOS", "bizstudio"))
	if err != nil || string(got) != dst {
		t.Fatalf("old app not restored: %q, %v", got, err)
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
