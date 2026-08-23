package qc

import (
	"context"
	"image"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func mkVideo(t *testing.T, dir, name, lavfi string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	out, err := exec.Command("ffmpeg", "-v", "error", "-y",
		"-f", "lavfi", "-i", lavfi,
		"-c:v", "libx264", "-pix_fmt", "yuv420p", p).CombinedOutput()
	if err != nil {
		t.Fatalf("dựng %s: %v — %s", name, err, out)
	}
	return p
}

// Ảnh lưới phải là một PNG thật, đúng số cột/hàng. Sinh ra file 0 byte hay một
// ô duy nhất thì tính năng coi như không có, mà lỗi lại im lặng.
func TestContactSheetProducesGrid(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("máy không có ffmpeg")
	}
	dir := t.TempDir()
	src := mkVideo(t, dir, "in.mp4", "testsrc=size=640x360:rate=25:duration=30")
	dst := filepath.Join(dir, "sheet", "contact.png")

	if err := ContactSheet(context.Background(), src, dst); err != nil {
		t.Fatalf("ContactSheet: %v", err)
	}
	f, err := os.Open(dst)
	if err != nil {
		t.Fatalf("mở ảnh lưới: %v", err)
	}
	defer f.Close()
	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatalf("ảnh lưới không giải mã được: %v", err)
	}
	if format != "png" {
		t.Errorf("định dạng %q, muốn png", format)
	}
	wantW := sheetCols * sheetTileW
	if cfg.Width != wantW {
		t.Errorf("rộng %d, muốn %d (%d cột × %d)", cfg.Width, wantW, sheetCols, sheetTileW)
	}
	// Ô cao bao nhiêu tuỳ tỉ lệ video; chỉ cần đúng số hàng nhân lên.
	if cfg.Height < sheetRows*100 {
		t.Errorf("cao %d — nghi là thiếu hàng (muốn %d hàng)", cfg.Height, sheetRows)
	}
	t.Logf("ảnh lưới %dx%d (%d ô)", cfg.Width, cfg.Height, sheetCols*sheetRows)
}

// Video ngắn hơn số ô vẫn phải ra ảnh, không được lỗi: người dùng dựng clip
// 5 giây là chuyện thường.
func TestContactSheetOnShortVideo(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("máy không có ffmpeg")
	}
	dir := t.TempDir()
	src := mkVideo(t, dir, "short.mp4", "testsrc=size=320x240:rate=25:duration=2")
	dst := filepath.Join(dir, "short.png")
	if err := ContactSheet(context.Background(), src, dst); err != nil {
		t.Fatalf("video 2 giây mà dựng lưới thất bại: %v", err)
	}
	if fi, err := os.Stat(dst); err != nil || fi.Size() == 0 {
		t.Fatalf("không sinh ra ảnh: %v", err)
	}
}

func TestContactSheetErrorsOnMissingFile(t *testing.T) {
	dir := t.TempDir()
	err := ContactSheet(context.Background(), filepath.Join(dir, "khong-co.mp4"), filepath.Join(dir, "x.png"))
	if err == nil {
		t.Fatal("file không tồn tại mà vẫn báo thành công")
	}
}
