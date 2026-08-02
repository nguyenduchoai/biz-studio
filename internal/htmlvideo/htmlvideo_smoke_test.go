package htmlvideo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bizstudio/internal/media"
	"bizstudio/internal/store"
)

func TestResolveSize(t *testing.T) {
	cases := []struct {
		aspect string
		w, h   int
		ok     bool
	}{
		{"", 1080, 1920, true},
		{"9:16", 1080, 1920, true},
		{"16:9", 1920, 1080, true},
		{"1:1", 1080, 1080, true},
		{"4:3", 0, 0, false},
	}
	for _, c := range cases {
		w, h, err := resolveSize(c.aspect)
		if c.ok && (err != nil || w != c.w || h != c.h) {
			t.Errorf("resolveSize(%q) = %d×%d, %v — muốn %d×%d", c.aspect, w, h, err, c.w, c.h)
		}
		if !c.ok && err == nil {
			t.Errorf("resolveSize(%q) phải trả lỗi", c.aspect)
		}
	}
}

func TestConfigDefaults(t *testing.T) {
	var c Config
	if c.fps() != 24 {
		t.Errorf("fps mặc định = %d, muốn 24", c.fps())
	}
	if c.theme() != "vivid" {
		t.Errorf("theme mặc định = %q, muốn vivid", c.theme())
	}
	if c.bgmVolume() != 0.25 {
		t.Errorf("bgmVolume mặc định = %v, muốn 0.25", c.bgmVolume())
	}
}

func TestFrameCount(t *testing.T) {
	if n := frameCount(5, 24); n != 120 {
		t.Errorf("frameCount(5,24) = %d, muốn 120", n)
	}
	if n := frameCount(0, 24); n != 1 {
		t.Errorf("frameCount(0,24) = %d, muốn 1", n)
	}
}

func TestWriteSceneHTML(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "scene.html")
	sc := Scene{
		Template: "chart",
		Title:    "So sánh tốc độ",
		Chart:    []ChartItem{{Label: "Trước", Value: 12}, {Label: "Sau", Value: 47.5}},
		Accent:   "#22D3EE",
	}
	if err := writeSceneHTML(sc, Config{Theme: "dark"}, 1080, 1920, 4.5, "", dst); err != nil {
		t.Fatalf("writeSceneHTML lỗi: %v", err)
	}
	raw, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("đọc file HTML: %v", err)
	}
	html := string(raw)
	for _, want := range []string{"window.seek", `"template":"chart"`, `"theme":"dark"`, `"dur":4.5`, "So sánh tốc độ"} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML cảnh thiếu %q", want)
		}
	}
	if strings.Contains(html, "http://") || strings.Contains(html, "https://") {
		t.Error("template phải offline hoàn toàn — không được tham chiếu tài nguyên mạng")
	}
}

func TestNormalizeTemplate(t *testing.T) {
	if normalizeTemplate(" Hero ") != "hero" || normalizeTemplate("unknown") != "hero" || normalizeTemplate("outro") != "outro" {
		t.Error("normalizeTemplate không chuẩn hoá đúng")
	}
}

func TestWriteSRT(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "out.srt")
	jobs := []*sceneJob{
		{scene: Scene{VoiceText: "Xin chào các bạn."}, durS: 3},
		{scene: Scene{}, durS: 2},
		{scene: Scene{VoiceText: "Hẹn gặp lại!"}, durS: 3},
	}
	if err := writeSRT(jobs, dst); err != nil {
		t.Fatalf("writeSRT lỗi: %v", err)
	}
	raw, _ := os.ReadFile(dst)
	srt := string(raw)
	if !strings.Contains(srt, "00:00:00,000 --> 00:00:03,000") {
		t.Errorf("cue 1 sai timing:\n%s", srt)
	}
	if !strings.Contains(srt, "00:00:05,000 --> 00:00:08,000") {
		t.Errorf("cue 2 phải cộng dồn qua cảnh im lặng:\n%s", srt)
	}
}

// TestRenderSmoke — e2e nhỏ: 2 cảnh × ~1.2s @ 8fps, không thuyết minh.
// Tự skip nếu máy thiếu Chrome/Chromium hoặc ffmpeg.
func TestRenderSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("bỏ qua e2e ở chế độ -short")
	}
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("bỏ qua: thiếu %s trong PATH", bin)
		}
	}
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "data"))
	if err != nil {
		t.Fatalf("mở store: %v", err)
	}
	if _, err := FindChrome(st); err != nil {
		t.Skipf("bỏ qua: %v", err)
	}

	scenes := []Scene{
		{Template: "hero", Title: "Biz Studio", Subtitle: "HTML Video", Duration: 1.2},
		{Template: "chart", Title: "Số liệu", Chart: []ChartItem{{Label: "A", Value: 3}, {Label: "B", Value: 7}}, Duration: 1.2},
	}
	cfg := Config{Aspect: "1:1", FPS: 8, Theme: "dark"}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	out, err := Render(ctx, st, scenes, cfg, filepath.Join(dir, "work"), nil)
	if err != nil {
		t.Fatalf("Render lỗi: %v", err)
	}
	info, err := media.Probe(out)
	if err != nil {
		t.Fatalf("probe output: %v", err)
	}
	if info.Width != 1080 || info.Height != 1080 {
		t.Errorf("kích thước output = %d×%d, muốn 1080×1080", info.Width, info.Height)
	}
	if info.Duration < 2.0 || info.Duration > 3.5 {
		t.Errorf("thời lượng output = %.2fs, muốn ~2.4s", info.Duration)
	}
	if _, err := os.Stat(strings.TrimSuffix(out, ".mp4") + ".srt"); err != nil {
		t.Errorf("thiếu file SRT cạnh output: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "work", "tmp")); !os.IsNotExist(err) {
		t.Error("tmp/ phải được dọn sau khi render thành công")
	}
}
