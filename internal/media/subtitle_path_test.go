package media

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestBurnSubsRendersSpecialCharacterPaths(t *testing.T) {
	requireFFmpeg(t)
	names := []string{"Dữ liệu có dấu cách", "Dữ liệu O'Brien,[bản 1]"}
	if runtime.GOOS != "windows" {
		names = append(names, "Dữ liệu: dấu hai chấm", `Dữ liệu\dấu gạch ngược;[1]`)
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), name)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			src := filepath.Join(dir, "video nền.mp4")
			if out, err := exec.Command("ffmpeg", "-v", "error", "-y", "-f", "lavfi", "-i", "color=black:size=320x240:duration=1", "-c:v", "libx264", src).CombinedOutput(); err != nil {
				t.Fatalf("fixture: %v %s", err, out)
			}
			srt := filepath.Join(dir, "phụ đề O'Brien,[1].srt")
			if err := os.WriteFile(srt, []byte("1\n00:00:00,000 --> 00:00:01,000\nXin chào Việt Nam\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			dst := filepath.Join(dir, "kết quả.mp4")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := BurnSubs(ctx, src, srt, dst); err != nil {
				t.Fatal(err)
			}
			frame, err := exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-ss", "0.5", "-i", dst, "-frames:v", "1", "-f", "rawvideo", "-pix_fmt", "gray", "pipe:1").Output()
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
				t.Fatalf("subtitle text was not drawn: %d bright pixels", bright)
			}
		})
	}
}

func TestLUTRendersSpecialCharacterPath(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "red.mp4")
	if out, err := exec.Command("ffmpeg", "-v", "error", "-y", "-f", "lavfi", "-i", "color=red:size=64x64:duration=0.2", "-c:v", "libx264", src).CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v %s", err, out)
	}
	lut := filepath.Join(dir, "màu O'Brien,[1].cube")
	if err := os.WriteFile(lut, []byte("LUT_3D_SIZE 2\n0 0 0\n1 0 0\n0 1 0\n1 1 0\n0 0 1\n1 0 1\n0 1 1\n1 1 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "graded.mp4")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := ApplyLUT(ctx, src, lut, dst); err != nil {
		t.Fatal(err)
	}
	frame, err := exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-i", dst, "-frames:v", "1", "-f", "rawvideo", "-pix_fmt", "rgb24", "pipe:1").Output()
	if err != nil || len(frame) < 3 || frame[0] < 150 || frame[1] > 50 || frame[2] > 50 {
		t.Fatalf("identity LUT failed to preserve red pixels: bytes=%d err=%v", len(frame), err)
	}
}
