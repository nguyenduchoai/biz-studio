package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"bizstudio/internal/util"
)

// run chạy ffmpeg, lỗi kèm phần cuối stderr (thông tin lỗi thật của ffmpeg nằm ở cuối).
// RunFFmpeg chạy ffmpeg với tham số cho trước — bản công khai của run, để các
// gói khác dùng chung một đường chạy ffmpeg (không qua shell).
func RunFFmpeg(ctx context.Context, args ...string) error { return run(ctx, args...) }

func run(ctx context.Context, args ...string) error {
	_, se, err := util.RunErr(ctx, "ffmpeg", args...)
	if err != nil {
		return fmt.Errorf("ffmpeg lỗi: %w — %s", err, tail(se, 400))
	}
	return nil
}

// tail lấy tối đa n ký tự cuối của chuỗi (đã trim).
func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return "…" + s[len(s)-n:]
	}
	return s
}

// ensureDir tạo thư mục cha của file đích.
func ensureDir(dst string) error {
	if dir := filepath.Dir(dst); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("không tạo được thư mục %s: %w", dir, err)
		}
	}
	return nil
}

// escapeFilterPath escape ':' và ”' cho path dùng trong filter ffmpeg (subtitles, lut3d).
func escapeFilterPath(p string) string {
	p = strings.ReplaceAll(p, `'`, `\'`)
	p = strings.ReplaceAll(p, `:`, `\:`)
	return p
}

// Thumbnail trích 1 frame tại giây t, scale về bề rộng w (giữ tỉ lệ, chiều cao chẵn).
func Thumbnail(src, dst string, t float64, w int) error {
	if w <= 0 {
		w = 640
	}
	if t < 0 {
		t = 0
	}
	if err := ensureDir(dst); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return run(ctx, "-y", "-ss", fmt.Sprintf("%.3f", t), "-i", src,
		"-frames:v", "1", "-vf", fmt.Sprintf("scale=%d:-2", w), dst)
}

// BurnSubs gắn cứng phụ đề SRT vào video.
func BurnSubs(ctx context.Context, src, srtPath, dst string) error {
	if _, err := os.Stat(srtPath); err != nil {
		return fmt.Errorf("không tìm thấy file phụ đề: %s", srtPath)
	}
	if err := ensureDir(dst); err != nil {
		return err
	}
	vf := fmt.Sprintf("subtitles='%s'", escapeFilterPath(srtPath))
	return run(ctx, "-y", "-i", src, "-vf", vf,
		"-c:v", "libx264", "-crf", "20", "-preset", "veryfast", "-c:a", "copy", dst)
}

// ExtractAudioWav16k tách audio thành WAV mono 16kHz (cho ASR).
func ExtractAudioWav16k(ctx context.Context, src, dst string) error {
	if err := ensureDir(dst); err != nil {
		return err
	}
	return run(ctx, "-y", "-i", src, "-vn", "-ac", "1", "-ar", "16000", "-c:a", "pcm_s16le", dst)
}

// ExtractFrames trích frame theo fps vào outDir/frame-%04d.jpg, trả danh sách path đã sort.
func ExtractFrames(ctx context.Context, src, outDir string, fps float64) ([]string, error) {
	if fps <= 0 {
		fps = 1
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("không tạo được thư mục %s: %w", outDir, err)
	}
	pattern := filepath.Join(outDir, "frame-%04d.jpg")
	if err := run(ctx, "-y", "-i", src, "-vf", fmt.Sprintf("fps=%g", fps), pattern); err != nil {
		return nil, err
	}
	files, err := filepath.Glob(filepath.Join(outDir, "frame-*.jpg"))
	if err != nil {
		return nil, fmt.Errorf("không liệt kê được frame: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("không trích xuất được frame nào từ %s", src)
	}
	sort.Strings(files)
	return files, nil
}

// Concat ghép nhiều video bằng concat demuxer; thử stream copy trước, lỗi thì re-encode.
func Concat(ctx context.Context, parts []string, dst string) error {
	if len(parts) == 0 {
		return fmt.Errorf("danh sách video cần ghép rỗng")
	}
	if err := ensureDir(dst); err != nil {
		return err
	}
	listPath, err := writeConcatList(parts, filepath.Dir(dst))
	if err != nil {
		return err
	}
	defer os.Remove(listPath)
	if err := run(ctx, "-y", "-f", "concat", "-safe", "0", "-i", listPath, "-c", "copy", dst); err == nil {
		return nil
	}
	return run(ctx, "-y", "-f", "concat", "-safe", "0", "-i", listPath,
		"-c:v", "libx264", "-crf", "20", "-preset", "veryfast", "-c:a", "aac", dst)
}

// writeConcatList ghi file danh sách cho concat demuxer (escape dấu nháy đơn).
func writeConcatList(parts []string, dir string) (string, error) {
	f, err := os.CreateTemp(dir, "concat-*.txt")
	if err != nil {
		return "", fmt.Errorf("không tạo được file danh sách ghép: %w", err)
	}
	for _, p := range parts {
		abs, absErr := filepath.Abs(p)
		if absErr != nil {
			abs = p
		}
		line := fmt.Sprintf("file '%s'\n", strings.ReplaceAll(abs, "'", `'\''`))
		if _, err := f.WriteString(line); err != nil {
			f.Close()
			os.Remove(f.Name())
			return "", fmt.Errorf("không ghi được file danh sách ghép: %w", err)
		}
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("không đóng được file danh sách ghép: %w", err)
	}
	return f.Name(), nil
}

// ApplyLUT áp LUT màu .cube lên video.
func ApplyLUT(ctx context.Context, src, lutPath, dst string) error {
	if _, err := os.Stat(lutPath); err != nil {
		return fmt.Errorf("không tìm thấy file LUT: %s", lutPath)
	}
	if err := ensureDir(dst); err != nil {
		return err
	}
	vf := fmt.Sprintf("lut3d='%s'", escapeFilterPath(lutPath))
	return run(ctx, "-y", "-i", src, "-vf", vf,
		"-c:v", "libx264", "-crf", "20", "-preset", "veryfast", "-c:a", "copy", dst)
}

// ReEncode scale+pad về đúng w×h, chất lượng cao (render final).
func ReEncode(ctx context.Context, src, dst string, w, h int) error {
	if w <= 0 || h <= 0 {
		return fmt.Errorf("kích thước không hợp lệ: %dx%d", w, h)
	}
	if err := ensureDir(dst); err != nil {
		return err
	}
	vf := fmt.Sprintf(
		"scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,setsar=1",
		w, h, w, h)
	return run(ctx, "-y", "-i", src, "-vf", vf,
		"-c:v", "libx264", "-crf", "18", "-preset", "medium",
		"-c:a", "aac", "-b:a", "192k", "-movflags", "+faststart", dst)
}
