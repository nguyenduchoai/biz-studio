package publishpkg

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func publishSource(dataDir, rel string) (string, error) {
	if !filepath.IsLocal(rel) {
		return "", fmt.Errorf("video output phải nằm trong thư mục dữ liệu")
	}
	base, err := filepath.EvalSymlinks(dataDir)
	if err != nil {
		return "", err
	}
	base, err = filepath.Abs(base)
	if err != nil {
		return "", err
	}
	src, err := filepath.EvalSymlinks(filepath.Join(base, rel))
	if err != nil {
		return "", fmt.Errorf("không tìm thấy video output: %w", err)
	}
	inside, err := filepath.Rel(base, src)
	if err != nil || !filepath.IsLocal(inside) {
		return "", fmt.Errorf("video output trỏ ra ngoài thư mục dữ liệu; nhập lại video vào dự án")
	}
	return src, nil
}

// Build everything alongside the live package. Replacement never merges stale
// subtitles/thumbnails, and a failed replacement restores the previous package.
func installPackage(staged, dst string) (string, error) {
	backup := ""
	if _, err := os.Lstat(dst); err == nil {
		backup, err = os.MkdirTemp(filepath.Dir(dst), ".publish-previous-")
		if err != nil {
			return "", err
		}
		if err := os.Remove(backup); err != nil {
			return "", err
		}
		if err := os.Rename(dst, backup); err != nil {
			return "", fmt.Errorf("không thể thay gói cũ (đóng file đang mở rồi thử lại): %w", err)
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.Rename(staged, dst); err != nil {
		if backup != "" {
			if restoreErr := os.Rename(backup, dst); restoreErr != nil {
				return "", fmt.Errorf("không cài được gói mới: %v; gói cũ được giữ tại %s (khôi phục thất bại: %v)", err, backup, restoreErr)
			}
		}
		return "", fmt.Errorf("không cài được gói mới; đã giữ nguyên gói trước: %w", err)
	}
	return backup, nil
}

type contextReader struct {
	ctx context.Context
	in  io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.in.Read(p)
}
