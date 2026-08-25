package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenRecoversCorruptDatabaseFromBackup(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := &Project{Name: "Giữ lại"}
	s.SaveProject(p)
	// Lượt ghi thứ hai tạo db.json.bak chứa project vừa lưu.
	s.SaveProject(&Project{Name: "Mới hơn"})
	if err := os.WriteFile(filepath.Join(dir, "db.json"), []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	recovered, err := Open(dir)
	if err != nil {
		t.Fatalf("không recovery được: %v", err)
	}
	if len(recovered.Projects()) == 0 || recovered.Projects()[0].Name != "Giữ lại" {
		t.Fatalf("backup không giữ project: %#v", recovered.Projects())
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "db.json.corrupt-*"))
	if len(matches) != 1 {
		t.Fatalf("không giữ bản db hỏng: %v", matches)
	}
}

func TestWriteRollsBackRAMWhenPersistenceFails(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	before := len(s.Projects())
	s.path = filepath.Join(t.TempDir(), "missing", "db.json")
	s.SaveProject(&Project{Name: "không được ACK trong RAM"})
	if got := len(s.Projects()); got != before {
		t.Fatalf("RAM vẫn mutate sau lỗi persist: before=%d after=%d", before, got)
	}
	if s.PersistenceError() == "" {
		t.Fatal("lỗi persist không được giữ để API báo")
	}
}
