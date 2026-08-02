package media

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// makeSample tạo video test 6s (320x240@30): tiếng bíp 0-2s, im lặng 2-4s, bíp 4-6s.
func makeSample(t *testing.T, dir string) string {
	t.Helper()
	src := filepath.Join(dir, "sample.mp4")
	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=30:duration=6",
		"-f", "lavfi", "-i", "aevalsrc=if(between(t\\,2\\,4)\\,0\\,0.5*sin(440*2*PI*t)):s=44100:d=6",
		"-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac", "-shortest", src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("không tạo được video mẫu: %v\n%s", err, out)
	}
	return src
}

func requireFFmpeg(t *testing.T) {
	t.Helper()
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("bỏ qua: thiếu %s trong PATH", bin)
		}
	}
}

func TestProbeAndAutoCut(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()
	src := makeSample(t, dir)

	info, err := Probe(src)
	if err != nil {
		t.Fatalf("Probe lỗi: %v", err)
	}
	if info.Duration < 5.5 || info.Duration > 6.5 {
		t.Errorf("duration = %.2f, muốn ~6", info.Duration)
	}
	if info.Width != 320 || info.Height != 240 {
		t.Errorf("kích thước = %dx%d, muốn 320x240", info.Width, info.Height)
	}
	if info.FPS < 29 || info.FPS > 31 {
		t.Errorf("fps = %.2f, muốn ~30", info.FPS)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	dst := filepath.Join(dir, "cut.mp4")
	if err := AutoCut(ctx, src, dst, 0, 0, nil); err != nil {
		t.Fatalf("AutoCut lỗi: %v", err)
	}
	cutInfo, err := Probe(dst)
	if err != nil {
		t.Fatalf("Probe output lỗi: %v", err)
	}
	if cutInfo.Duration >= info.Duration-0.5 {
		t.Errorf("sau AutoCut duration = %.2f, muốn ngắn hơn %.2f rõ rệt", cutInfo.Duration, info.Duration)
	}
}

func TestThumbnailAndConcat(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()
	src := makeSample(t, dir)

	thumb := filepath.Join(dir, "thumb.jpg")
	if err := Thumbnail(src, thumb, 1.0, 320); err != nil {
		t.Fatalf("Thumbnail lỗi: %v", err)
	}
	if st, err := os.Stat(thumb); err != nil || st.Size() == 0 {
		t.Fatalf("thumbnail rỗng hoặc thiếu: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	joined := filepath.Join(dir, "joined.mp4")
	if err := Concat(ctx, []string{src, src}, joined); err != nil {
		t.Fatalf("Concat lỗi: %v", err)
	}
	info, err := Probe(joined)
	if err != nil {
		t.Fatalf("Probe joined lỗi: %v", err)
	}
	if info.Duration < 11 || info.Duration > 13.5 {
		t.Errorf("duration ghép = %.2f, muốn ~12", info.Duration)
	}
}

func TestParseSilences(t *testing.T) {
	stderr := `[silencedetect @ 0x1] silence_start: 2.01
[silencedetect @ 0x1] silence_end: 3.98 | silence_duration: 1.97
[silencedetect @ 0x1] silence_start: 5.5`
	spans := ParseSilences(stderr, 6)
	if len(spans) != 2 {
		t.Fatalf("số khoảng lặng = %d, muốn 2", len(spans))
	}
	if spans[0].Start != 2.01 || spans[0].End != 3.98 {
		t.Errorf("span[0] = %+v", spans[0])
	}
	if spans[1].Start != 5.5 || spans[1].End != 6 {
		t.Errorf("span[1] chưa chốt tại duration: %+v", spans[1])
	}
}

func TestComputeKeeps(t *testing.T) {
	keeps := computeKeeps([]TimeSpan{{Start: 2, End: 4}}, 6)
	if len(keeps) != 2 {
		t.Fatalf("số đoạn giữ = %d, muốn 2", len(keeps))
	}
	if keeps[0].start != 0 || keeps[0].end != 2.15 {
		t.Errorf("keep[0] = %+v, muốn [0, 2.15]", keeps[0])
	}
	if keeps[1].start != 3.85 || keeps[1].end != 6 {
		t.Errorf("keep[1] = %+v, muốn [3.85, 6]", keeps[1])
	}
	// Video toàn im lặng → không còn đoạn giữ.
	if got := computeKeeps([]TimeSpan{{Start: 0, End: 6}}, 6); len(got) != 0 {
		t.Errorf("video toàn im lặng nhưng vẫn giữ %d đoạn", len(got))
	}
}
