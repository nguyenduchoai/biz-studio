// Package publishpkg — đóng gói xuất bản: video final, phụ đề, metadata, thumbnail, zip.
package publishpkg

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bizstudio/internal/store"
)

// Build tạo gói xuất bản trong <projectDir>/publish và trả đường dẫn file zip.
func Build(ctx context.Context, st *store.Store, p *store.Project, projectDir string, upd func(float64, string)) (string, error) {
	if p.OutputFile == "" {
		return "", fmt.Errorf("dự án chưa có video output")
	}
	src := filepath.Join(st.DataDir, p.OutputFile)
	if !fileExists(src) {
		return "", fmt.Errorf("không tìm thấy video output: %s", src)
	}

	pubDir := filepath.Join(projectDir, "publish")
	if err := os.MkdirAll(pubDir, 0o755); err != nil {
		return "", fmt.Errorf("không tạo được thư mục publish: %w", err)
	}
	upd(5, "Chuẩn bị thư mục publish")

	finalPath := filepath.Join(pubDir, "final.mp4")
	if err := copyFile(src, finalPath); err != nil {
		return "", fmt.Errorf("không copy được video: %w", err)
	}
	upd(30, "Đã copy video → final.mp4")

	if srt := newestSrt(projectDir); srt != "" {
		if err := copyFile(srt, filepath.Join(pubDir, "subs.srt")); err != nil {
			return "", fmt.Errorf("không copy được phụ đề: %w", err)
		}
		if err := srtToVtt(filepath.Join(pubDir, "subs.srt"), filepath.Join(pubDir, "subs.vtt")); err != nil {
			return "", fmt.Errorf("không chuyển được phụ đề sang VTT: %w", err)
		}
		upd(45, "Đã đóng gói phụ đề (subs.srt + subs.vtt)")
	} else {
		upd(45, "Không tìm thấy phụ đề .srt — bỏ qua")
	}

	upd(50, "Đang sinh metadata (tiêu đề, mô tả, hashtags)…")
	meta := generateMeta(ctx, st, p)
	addProbeInfo(ctx, st, meta, finalPath)
	if err := writeJSONFile(filepath.Join(pubDir, "meta.json"), meta); err != nil {
		return "", fmt.Errorf("không ghi được meta.json: %w", err)
	}
	upd(70, "Đã ghi meta.json")

	if p.ThumbFile != "" {
		tsrc := filepath.Join(st.DataDir, p.ThumbFile)
		if fileExists(tsrc) {
			dst := filepath.Join(pubDir, "thumbnail"+strings.ToLower(filepath.Ext(tsrc)))
			if err := copyFile(tsrc, dst); err != nil {
				return "", fmt.Errorf("không copy được thumbnail: %w", err)
			}
			upd(80, "Đã copy thumbnail")
		} else {
			st.AddLog("warn", "publish", "Không tìm thấy thumbnail: "+tsrc)
			upd(80, "Không tìm thấy thumbnail — bỏ qua")
		}
	}

	upd(85, "Đang nén gói xuất bản…")
	zipPath := filepath.Join(pubDir, p.ID+"-package.zip")
	if err := zipDir(pubDir, zipPath); err != nil {
		return "", fmt.Errorf("không nén được gói xuất bản: %w", err)
	}
	upd(100, "Hoàn thành gói xuất bản")
	return zipPath, nil
}

// newestSrt tìm file .srt mới nhất trong projectDir (root + outputs/).
func newestSrt(projectDir string) string {
	best, bestTime := "", time.Time{}
	for _, dir := range []string{projectDir, filepath.Join(projectDir, "outputs")} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".srt") {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			if info.ModTime().After(bestTime) {
				best, bestTime = filepath.Join(dir, e.Name()), info.ModTime()
			}
		}
	}
	return best
}

// srtToVtt chuyển SRT sang WebVTT: thêm header + đổi dấu phẩy ms thành dấu chấm ở dòng timing.
func srtToVtt(srtPath, vttPath string) error {
	b, err := os.ReadFile(srtPath)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	for i, line := range lines {
		if strings.Contains(line, "-->") {
			lines[i] = strings.ReplaceAll(line, ",", ".")
		}
	}
	out := "WEBVTT\n\n" + strings.Join(lines, "\n")
	return os.WriteFile(vttPath, []byte(out), 0o644)
}

// zipDir nén toàn bộ dir vào zipPath (bỏ qua chính file zip).
func zipDir(dir, zipPath string) error {
	// Gom danh sách file trước để không nén nhầm file zip đang ghi dở.
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || path == zipPath {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return err
	}

	f, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	for _, path := range files {
		if err := addToZip(zw, dir, path); err != nil {
			zw.Close()
			f.Close()
			return err
		}
	}
	if err := zw.Close(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func addToZip(zw *zip.Writer, baseDir, path string) error {
	rel, err := filepath.Rel(baseDir, path)
	if err != nil {
		return err
	}
	w, err := zw.Create(filepath.ToSlash(rel))
	if err != nil {
		return err
	}
	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()
	_, err = io.Copy(w, src)
	return err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func writeJSONFile(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}
