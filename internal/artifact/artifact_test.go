package artifact

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

type payload struct {
	Text string `json:"text"`
	N    int    `json:"n"`
}

func TestSaveLoadRoundTrip(t *testing.T) {
	s := New(t.TempDir())
	key := Key("a", "b")
	var got payload
	if s.Load("test", key, &got) {
		t.Fatal("chưa lưu gì mà đã đọc được")
	}
	s.Save("test", key, payload{Text: "xin chào", N: 7})
	if !s.Load("test", key, &got) {
		t.Fatal("lưu rồi mà không đọc lại được")
	}
	if got.Text != "xin chào" || got.N != 7 {
		t.Errorf("dữ liệu sai: %+v", got)
	}
}

// Đổi bất kỳ phần nào của khoá phải ra khoá khác, nếu không kết quả của lần
// chạy này bị trả về cho lần chạy khác.
func TestKeyChangesWithEveryPart(t *testing.T) {
	base := Key("video.mp4", "vi", "small")
	for _, other := range [][]string{
		{"video2.mp4", "vi", "small"},
		{"video.mp4", "en", "small"},
		{"video.mp4", "vi", "large-v3"},
		{"video.mp4", "vi"},
	} {
		if Key(other...) == base {
			t.Errorf("khoá trùng với %v — kết quả sẽ bị dùng nhầm", other)
		}
	}
	if Key("video.mp4", "vi", "small") != base {
		t.Error("cùng đầu vào mà ra khoá khác — cache không bao giờ trúng")
	}
}

// Người dùng thay file rồi giữ nguyên tên là chuyện thường. Khoá chỉ theo tên
// thì trả về bản bóc băng của video CŨ — hỏng cực khó lần ra.
func TestFileKeyChangesWhenFileChanges(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "same-name.txt")

	if err := os.WriteFile(p, []byte("nội dung một"), 0o644); err != nil {
		t.Fatal(err)
	}
	k1 := FileKey(p)

	// Lùi mốc sửa đổi để chắc chắn khác, không phụ thuộc độ phân giải đồng hồ.
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(p, []byte("nội dung hai — dài hơn hẳn"), 0o644); err != nil {
		t.Fatal(err)
	}
	k2 := FileKey(p)

	if k1 == k2 {
		t.Errorf("file đổi mà khoá không đổi: %s", k1)
	}
	if FileKey(filepath.Join(dir, "khong-co.txt")) == k2 {
		t.Error("file không tồn tại lại trùng khoá với file có thật")
	}
}

// Cache hỏng (ghi dở vì mất điện) phải bị bỏ qua và xoá, không được làm job chết.
func TestLoadIgnoresCorruptFile(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	key := Key("x")
	s.Save("test", key, payload{Text: "ok"})

	if err := os.WriteFile(s.path("test", key), []byte("{ hỏng"), 0o644); err != nil {
		t.Fatal(err)
	}
	var got payload
	if s.Load("test", key, &got) {
		t.Fatal("đọc được từ file hỏng")
	}
	if _, err := os.Stat(s.path("test", key)); !os.IsNotExist(err) {
		t.Error("file hỏng chưa bị xoá — lần sau vẫn vấp đúng chỗ đó")
	}
}

func TestPruneRemovesOldOnly(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	s.Save("test", Key("moi"), payload{Text: "mới"})
	s.Save("test", Key("cu"), payload{Text: "cũ"})

	old := s.path("test", Key("cu"))
	past := time.Now().Add(-maxAge - time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}

	if n := s.Prune(); n != 1 {
		t.Errorf("xoá %d file, muốn đúng 1", n)
	}
	var got payload
	if !s.Load("test", Key("moi"), &got) {
		t.Error("Prune xoá nhầm cache còn hạn")
	}
	if s.Load("test", Key("cu"), &got) {
		t.Error("cache quá hạn vẫn còn")
	}
}

// Cache là thứ tăng tốc, không phải thứ bắt buộc: Store nil hay thư mục không
// ghi được đều không được làm job chết.
func TestNilStoreIsHarmless(t *testing.T) {
	var s *Store
	var got payload
	if s.Load("test", "k", &got) {
		t.Error("Store nil mà đọc được gì đó")
	}
	s.Save("test", "k", payload{}) // không được panic
	if n := s.Prune(); n != 0 {
		t.Errorf("Store nil mà Prune xoá %d file", n)
	}
}
