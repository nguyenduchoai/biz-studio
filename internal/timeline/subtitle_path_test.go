package timeline

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestTimelineSubtitlesRenderSpecialCharacterPaths(t *testing.T) {
	requireFFmpeg(t)
	name := "Dữ liệu O'Brien,[bản 1]"
	if runtime.GOOS != "windows" {
		name += ": dấu hai chấm"
	}
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(dir, "video nền.mp4")
	if out, err := exec.Command("ffmpeg", "-v", "error", "-y", "-f", "lavfi", "-i", "color=black:size=320x240:duration=1", "-f", "lavfi", "-i", "sine=duration=1", "-c:v", "libx264", "-c:a", "aac", "-shortest", base).CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v %s", err, out)
	}
	doc := &Doc{Video: base, VideoDur: 1, Subs: []Cue{{Start: 0, End: 1, Text: "Xin chào Việt Nam"}}}
	doc.Normalize()
	srt := filepath.Join(dir, "phụ đề O'Brien,[1].srt")
	if err := WriteSRT(doc, srt); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(doc, base, true, srt)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "kết quả.mp4")
	run(t, plan, dst)
	frame, err := exec.Command("ffmpeg", "-v", "error", "-ss", "0.5", "-i", dst, "-frames:v", "1", "-f", "rawvideo", "-pix_fmt", "gray", "pipe:1").Output()
	if err != nil {
		t.Fatal(err)
	}
	bright := 0
	for _, pixel := range frame {
		if pixel > 100 {
			bright++
		}
	}
	if bright < 30 {
		t.Fatalf("timeline subtitles were not drawn: %d bright pixels", bright)
	}
}
