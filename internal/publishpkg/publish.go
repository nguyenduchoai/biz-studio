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

	"bizstudio/internal/qc"
	"bizstudio/internal/store"
)

// Build tạo gói xuất bản trong <projectDir>/publish và trả đường dẫn file zip.
func Build(ctx context.Context, st *store.Store, p *store.Project, projectDir string, upd func(float64, string)) (string, error) {
	if p.OutputFile == "" {
		return "", fmt.Errorf("dự án chưa có video output")
	}
	src, err := publishSource(st.DataDir, p.OutputFile)
	if err != nil {
		return "", err
	}
	if p.ID == "" || filepath.Base(p.ID) != p.ID {
		return "", fmt.Errorf("mã dự án không hợp lệ")
	}
	if upd == nil {
		upd = func(float64, string) {}
	}
	upd(2, "Kiểm tra toàn bộ video và QC trước khi xuất bản…")
	// Never reuse project/qc.json: the same output path can now contain a new
	// render. No publish file or provider request is made before this gate.
	report, err := qc.Run(ctx, src)
	if err != nil {
		return "", fmt.Errorf("chưa thể xuất bản: QC video thất bại; render lại hoặc sửa file rồi thử lại: %w", err)
	}
	for _, warning := range report.Warnings {
		st.AddLog("warn", "publish", "QC: "+warning)
	}

	pubDir := filepath.Join(projectDir, "publish")
	staged, err := os.MkdirTemp(projectDir, ".publish-")
	if err != nil {
		return "", fmt.Errorf("không tạo được thư mục publish: %w", err)
	}
	defer os.RemoveAll(staged)
	upd(5, fmt.Sprintf("QC hợp lệ (%d cảnh báo); chuẩn bị gói mới", len(report.Warnings)))

	finalPath := filepath.Join(staged, "final.mp4")
	if err := copyFile(ctx, src, finalPath); err != nil {
		return "", fmt.Errorf("không copy được video: %w", err)
	}
	fingerprint, err := qc.Fingerprint(ctx, finalPath)
	if err != nil {
		return "", err
	}
	if fingerprint != report.SourceSHA256 {
		return "", fmt.Errorf("video đã thay đổi sau QC; chờ render xong rồi xuất bản lại")
	}
	if err := writeJSONFile(filepath.Join(staged, "qc.json"), report); err != nil {
		return "", fmt.Errorf("không ghi được báo cáo QC: %w", err)
	}
	upd(30, "Đã copy video → final.mp4")

	if srt := newestSrt(projectDir); srt != "" {
		if err := copyFile(ctx, srt, filepath.Join(staged, "subs.srt")); err != nil {
			return "", fmt.Errorf("không copy được phụ đề: %w", err)
		}
		if err := srtToVtt(filepath.Join(staged, "subs.srt"), filepath.Join(staged, "subs.vtt")); err != nil {
			return "", fmt.Errorf("không chuyển được phụ đề sang VTT: %w", err)
		}
		upd(45, "Đã đóng gói phụ đề (subs.srt + subs.vtt)")
	} else {
		upd(45, "Không tìm thấy phụ đề .srt — bỏ qua")
	}

	upd(50, "Đang sinh metadata (tiêu đề, mô tả, hashtags)…")
	meta := generateMeta(ctx, st, p)
	meta["qcWarnings"], meta["videoSha256"] = report.Warnings, report.SourceSHA256
	addProbeInfo(ctx, st, meta, finalPath)
	if err := writeJSONFile(filepath.Join(staged, "meta.json"), meta); err != nil {
		return "", fmt.Errorf("không ghi được meta.json: %w", err)
	}
	upd(70, "Đã ghi meta.json")

	if p.ThumbFile != "" {
		tsrc := filepath.Join(st.DataDir, p.ThumbFile)
		if fileExists(tsrc) {
			dst := filepath.Join(staged, "thumbnail"+strings.ToLower(filepath.Ext(tsrc)))
			if err := copyFile(ctx, tsrc, dst); err != nil {
				return "", fmt.Errorf("không copy được thumbnail: %w", err)
			}
			upd(80, "Đã copy thumbnail")
		} else {
			st.AddLog("warn", "publish", "Không tìm thấy thumbnail: "+tsrc)
			upd(80, "Không tìm thấy thumbnail — bỏ qua")
		}
	}

	upd(85, "Đang nén gói xuất bản…")
	zipName := p.ID + "-package.zip"
	if err := zipDir(ctx, staged, filepath.Join(staged, zipName)); err != nil {
		return "", fmt.Errorf("không nén được gói xuất bản: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	backup, err := installPackage(staged, pubDir)
	if err != nil {
		return "", err
	}
	if backup != "" {
		if err := os.RemoveAll(backup); err != nil {
			st.AddLog("warn", "publish", "Gói mới đã sẵn sàng; không xoá được bản lưu cũ: "+backup)
		}
	}
	upd(100, fmt.Sprintf("Hoàn thành gói xuất bản (%d cảnh báo QC — xem qc.json)", len(report.Warnings)))
	return filepath.Join(pubDir, zipName), nil
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
func zipDir(ctx context.Context, dir, zipPath string) error {
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
		if err := addToZip(ctx, zw, dir, path); err != nil {
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

func addToZip(ctx context.Context, zw *zip.Writer, baseDir, path string) error {
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
	_, err = io.Copy(w, contextReader{ctx, src})
	return err
}

func copyFile(ctx context.Context, src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, contextReader{ctx, in}); err != nil {
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
