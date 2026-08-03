package dubbing

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"bizstudio/internal/util"
)

// runFFmpeg chạy ffmpeg, lỗi kèm phần cuối stderr (thông tin lỗi thật của
// ffmpeg nằm ở cuối output).
func runFFmpeg(ctx context.Context, args ...string) error {
	if _, se, err := util.RunErr(ctx, "ffmpeg", args...); err != nil {
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

// shortText rút gọn chuỗi hiển thị theo rune (an toàn tiếng Việt).
func shortText(v string, n int) string {
	v = strings.TrimSpace(v)
	runes := []rune(v)
	if len(runes) > n {
		return string(runes[:n]) + "…"
	}
	return v
}

// copyFile sao chép file src → dst (tạo thư mục cha nếu cần).
func copyFile(src, dst string) error {
	if dir := filepath.Dir(dst); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("tạo thư mục %s: %w", dir, err)
		}
	}
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("mở file %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("tạo file %s: %w", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("ghi file %s: %w", dst, err)
	}
	return out.Close()
}
