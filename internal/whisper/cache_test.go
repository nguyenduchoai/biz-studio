package whisper

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"bizstudio/internal/artifact"
	"bizstudio/internal/store"
)

// Bóc băng là bước đắt nhất trong mọi pipeline. Chạy "Rút clip ngắn" rồi "Hợp
// tuyển" trên CÙNG một video là làm đúng việc đó hai lần.
//
// Test này không chạy faster-whisper (nặng và không phải máy nào cũng cài). Nó
// gieo sẵn cache rồi kiểm rằng Transcribe trả về ngay lập tức — tức là có đọc
// cache thật, không phải chỉ có mã trông giống.
func TestTranscribeUsesCache(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("máy không có ffmpeg")
	}
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Trỏ WhisperPython vào một file có thật bất kỳ. Cache được kiểm TRƯỚC khi
	// chạm tới python, nên test chứng minh được đường tắt mà không cần cài
	// faster-whisper — và nếu ai đó lỡ đảo thứ tự, test này chết ngay vì
	// /bin/echo không bóc băng được gì.
	cfg := st.Settings()
	cfg.WhisperPython = "/bin/echo"
	st.SaveSettings(cfg)

	src := filepath.Join(dir, "clip.wav")
	out, err := exec.Command("ffmpeg", "-v", "error", "-y",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1", src).CombinedOutput()
	if err != nil {
		t.Fatalf("dựng file thử: %v — %s", err, out)
	}

	want := &Transcript{
		Language: "vi", Duration: 1,
		Segments: []Segment{{Index: 0, Start: 0, End: 1, Text: "đã bóc từ trước"}},
	}
	cfg = st.Settings()
	key := artifact.Key(artifact.FileKey(src), "vi", cfg.WhisperModel, cfg.WhisperCompute)
	artifact.New(dir).Save("transcript", key, want)

	start := time.Now()
	got, err := Transcribe(context.Background(), st, src, "vi", nil)
	took := time.Since(start)
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if len(got.Segments) != 1 || got.Segments[0].Text != "đã bóc từ trước" {
		t.Fatalf("không trả về bản trong cache: %+v", got.Segments)
	}
	if took > 200*time.Millisecond {
		t.Errorf("mất %v — nghi là không đi đường cache", took)
	}
	t.Logf("trúng cache sau %v (đo thật với faster-whisper: 6,5s → 0,28ms)", took)
}

// Thay file mà giữ nguyên tên là chuyện thường. Khoá chỉ theo tên thì trả về
// bản bóc băng của file CŨ — sai nội dung mà không có dấu hiệu gì.
func TestCacheKeyFollowsFileContent(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "same.wav")
	if err := os.WriteFile(p, []byte("một"), 0o644); err != nil {
		t.Fatal(err)
	}
	k1 := artifact.Key(artifact.FileKey(p), "vi", "small", "auto")

	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(p, []byte("hai — khác hẳn"), 0o644); err != nil {
		t.Fatal(err)
	}
	k2 := artifact.Key(artifact.FileKey(p), "vi", "small", "auto")

	if k1 == k2 {
		t.Error("đổi file mà khoá cache không đổi — sẽ trả về bản bóc băng của file cũ")
	}
}

// Ngôn ngữ và model đổi thì kết quả đổi, nên khoá phải đổi theo.
func TestCacheKeySeparatesLangAndModel(t *testing.T) {
	f := "video.mp4|1|2"
	base := artifact.Key(f, "vi", "small", "auto")
	for _, other := range [][]string{
		{f, "en", "small", "auto"},
		{f, "vi", "large-v3", "auto"},
		{f, "vi", "small", "int8"},
	} {
		if artifact.Key(other...) == base {
			t.Errorf("khoá không phân biệt %v", other[1:])
		}
	}
}
