package qc

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestParseLUFS(t *testing.T) {
	stderr := `[Parsed_ebur128_0 @ 0x1] t: 1.0  TARGET:-23 LUFS  M: -20.0 S: -20.0  I: -25.4 LUFS  LRA: 0.0 LU
[Parsed_ebur128_0 @ 0x1] Summary:

  Integrated loudness:
    I:         -15.5 LUFS
    Threshold: -25.6 LUFS

  Loudness range:
    LRA:         6.0 LU
    Threshold:  -35.5 LUFS
    LRA low:    -20.0 LUFS
    LRA high:   -14.0 LUFS`
	v, ok := parseLUFS(stderr)
	if !ok || v != -15.5 {
		t.Fatalf("parseLUFS = %v/%v, muốn -15.5/true (phải lấy dòng Summary cuối)", v, ok)
	}
}

func TestParseBlackAndFreeze(t *testing.T) {
	stderr := `[blackdetect @ 0x1] black_start:0 black_end:1.001 black_duration:1.001
[freezedetect @ 0x1] lavfi.freezedetect.freeze_start: 12.4
[freezedetect @ 0x1] lavfi.freezedetect.freeze_duration: 3.1
[freezedetect @ 0x1] lavfi.freezedetect.freeze_end: 15.5`
	blacks := parseBlack(stderr)
	if len(blacks) != 1 || blacks[0].Start != 0 || blacks[0].End != 1.001 {
		t.Fatalf("parseBlack = %+v", blacks)
	}
	freezes := parseFreeze(stderr, 20)
	if len(freezes) != 1 || freezes[0].Start != 12.4 || freezes[0].End != 15.5 {
		t.Fatalf("parseFreeze = %+v", freezes)
	}
}

// TestRunOnBlackSilentVideo — video đen tĩnh + im lặng phải ra đủ cảnh báo.
func TestRunOnBlackSilentVideo(t *testing.T) {
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("bỏ qua: thiếu %s trong PATH", bin)
		}
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "black.mp4")
	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "color=c=black:size=320x240:rate=30:duration=5",
		"-f", "lavfi", "-i", "anullsrc=r=44100:cl=mono",
		"-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac", "-t", "5", src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("không tạo được video mẫu: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	rep, err := Run(ctx, src)
	if err != nil {
		t.Fatalf("qc.Run lỗi: %v", err)
	}
	if rep.DurationS < 4.5 || rep.DurationS > 5.5 {
		t.Errorf("DurationS = %.2f, muốn ~5", rep.DurationS)
	}
	if rep.Width != 320 || rep.Height != 240 {
		t.Errorf("kích thước = %dx%d, muốn 320x240", rep.Width, rep.Height)
	}
	if len(rep.BlackSpans) == 0 {
		t.Error("muốn có BlackSpans với video đen")
	}
	if len(rep.FreezeSpans) == 0 {
		t.Error("muốn có FreezeSpans với video tĩnh")
	}
	if len(rep.SilenceSpans) == 0 {
		t.Error("muốn có SilenceSpans với audio im lặng")
	}
	if len(rep.Warnings) == 0 {
		t.Error("muốn có cảnh báo tiếng Việt")
	}
}
