package agent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Fingerprint content, not mtime: touching or renaming an old video must not
// make a failed run appear successful. The bounded context also covers reads.
type outputSnapshot map[[sha256.Size]byte]bool

func (r *Runner) snapshotOutputs(ctx context.Context, projectID string) (outputSnapshot, error) {
	root := filepath.Join(r.dataDir, "projects", projectID)
	if err := ensureProjectDirs(root); err != nil {
		return nil, err
	}
	before := outputSnapshot{}
	err := filepath.WalkDir(filepath.Join(root, "outputs"), func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 || d.IsDir() || !strings.EqualFold(filepath.Ext(name), ".mp4") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("output cũ không phải file thường: %s", d.Name())
		}
		sum, err := digestFile(ctx, name)
		if err != nil {
			return err
		}
		before[sum] = true
		return nil
	})
	return before, err
}

func ensureProjectDirs(root string) error {
	for _, dir := range []string{filepath.Dir(root), root, filepath.Join(root, "assets"), filepath.Join(root, "outputs"), filepath.Join(root, "tmp")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		info, err := os.Lstat(dir)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("thư mục dự án không được là liên kết: %s", filepath.Base(dir))
		}
	}
	return nil
}

func (r *Runner) verifiedOutput(ctx context.Context, projectID string, before outputSnapshot) (string, error) {
	root := filepath.Join(r.dataDir, "projects", projectID)
	metaPath, err := projectRegularFile(root, "meta.json")
	if err != nil {
		return "", fmt.Errorf("meta.json không hợp lệ: %w", err)
	}
	f, err := os.Open(metaPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	var meta struct {
		Status string `json:"status"`
		Output string `json:"output"`
	}
	raw, err := io.ReadAll(io.LimitReader(f, 64*1024+1))
	if err != nil || len(raw) > 64*1024 || json.Unmarshal(raw, &meta) != nil {
		return "", fmt.Errorf("meta.json hỏng hoặc quá lớn")
	}
	if meta.Status != "done" {
		return "", fmt.Errorf("meta.json chưa xác nhận status=done")
	}
	rel := strings.ReplaceAll(meta.Output, "\\", "/")
	if !strings.HasPrefix(rel, "outputs/") || !strings.EqualFold(path.Ext(rel), ".mp4") {
		return "", fmt.Errorf("video kết quả phải nằm trong outputs/ và có đuôi .mp4")
	}
	abs, err := projectRegularFile(root, rel)
	if err != nil {
		return "", err
	}
	sum, err := digestFile(ctx, abs)
	if err != nil {
		return "", err
	}
	if before[sum] {
		return "", fmt.Errorf("chỉ tìm thấy video cũ chưa thay đổi từ trước lượt chạy")
	}
	if err := r.validateVideo(ctx, abs); err != nil {
		return "", err
	}
	// Recheck containment/content after decoding: do not publish a swapped or
	// still-growing artifact written by a detached child process.
	if _, err := projectRegularFile(root, rel); err != nil {
		return "", err
	}
	after, err := digestFile(ctx, abs)
	if err != nil {
		return "", err
	}
	if after != sum {
		return "", fmt.Errorf("video còn thay đổi trong khi kiểm tra; hãy chờ dựng xong")
	}
	return path.Join("projects", projectID, rel), nil
}

func projectRegularFile(root, rel string) (string, error) {
	if rel == "" || strings.ContainsAny(rel, ":\\\x00") || path.IsAbs(rel) || path.Clean(rel) != rel {
		return "", fmt.Errorf("đường dẫn kết quả không hợp lệ")
	}
	parts := strings.Split(rel, "/")
	name := root
	for i, part := range append([]string{""}, parts...) {
		if part == ".." {
			return "", fmt.Errorf("đường dẫn kết quả vượt thư mục dự án")
		}
		name = filepath.Join(name, part)
		info, err := os.Lstat(name)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("kết quả không được là liên kết")
		}
		if i < len(parts) && !info.IsDir() {
			return "", fmt.Errorf("thư mục kết quả không hợp lệ")
		}
		if i == len(parts) && (!info.Mode().IsRegular() || info.Size() == 0) {
			return "", fmt.Errorf("kết quả không phải file thường có nội dung")
		}
	}
	return name, nil
}

func digestFile(ctx context.Context, name string) ([sha256.Size]byte, error) {
	var result [sha256.Size]byte
	f, err := os.Open(name)
	if err != nil {
		return result, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return result, err
	}
	if !info.Mode().IsRegular() {
		return result, fmt.Errorf("kết quả không phải file thường")
	}
	h := sha256.New()
	buf := make([]byte, 256*1024)
	for {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		n, err := f.Read(buf)
		_, _ = h.Write(buf[:n])
		if err == io.EOF {
			break
		}
		if err != nil {
			return result, err
		}
	}
	copy(result[:], h.Sum(nil))
	return result, nil
}
