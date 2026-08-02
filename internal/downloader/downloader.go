// Package downloader — tải video qua yt-dlp, báo tiến độ realtime.
package downloader

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"bizstudio/internal/store"
	"bizstudio/internal/util"
)

var (
	reProgress = regexp.MustCompile(`\[download\]\s+([\d.]+)%`)
	reDest     = regexp.MustCompile(`Destination:\s+(.+)$`)
	reMerger   = regexp.MustCompile(`\[Merger\]\s+Merging formats into\s+"(.+)"`)
	reAlready  = regexp.MustCompile(`\[download\]\s+(.+?)\s+has already been downloaded`)
)

// Download tải 1 link qua yt-dlp vào Settings.DownloadDir.
// quality: "best" (mặc định) | "1080" | "720" | "audio".
// upd(progress 0..100, detail) được gọi theo từng dòng tiến độ.
// Trả đường dẫn file kết quả.
func Download(ctx context.Context, st *store.Store, link, quality string, upd func(float64, string)) (string, error) {
	link = strings.TrimSpace(link)
	if link == "" {
		return "", fmt.Errorf("thiếu link video cần tải")
	}
	cfg := st.Settings()
	bin := cfg.YtdlpBin
	if bin == "" {
		bin = "yt-dlp"
	}
	if !util.Exists(bin) {
		return "", fmt.Errorf("chưa cài yt-dlp — cài bằng: brew install yt-dlp")
	}
	dir := cfg.DownloadDir
	if dir == "" {
		dir = filepath.Join(st.DataDir, "downloads")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("không tạo được thư mục tải về %s: %w", dir, err)
	}

	args := buildArgs(cfg, dir, quality, link)
	started := time.Now()
	upd(0, "Bắt đầu tải: "+link)

	cmd := exec.CommandContext(ctx, bin, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("không mở được stdout của yt-dlp: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("không chạy được yt-dlp: %w", err)
	}

	lastPath := scanOutput(stdout, upd)

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("tải video bị huỷ: %w", ctx.Err())
		}
		return "", fmt.Errorf("yt-dlp lỗi: %s", truncate(stderr.String(), 500))
	}

	if lastPath != "" && fileExists(lastPath) {
		upd(100, "Đã tải xong: "+filepath.Base(lastPath))
		return lastPath, nil
	}
	// Fallback: file mới nhất xuất hiện trong DownloadDir sau khi chạy.
	if p := newestFile(dir, started); p != "" {
		upd(100, "Đã tải xong: "+filepath.Base(p))
		return p, nil
	}
	return "", fmt.Errorf("yt-dlp chạy xong nhưng không tìm thấy file tải về trong %s", dir)
}

// buildArgs dựng tham số yt-dlp theo chất lượng + cookies.
func buildArgs(cfg store.Settings, dir, quality, link string) []string {
	args := []string{
		"--newline", "--no-playlist",
		"-o", filepath.Join(dir, "%(title).80s-%(id)s.%(ext)s"),
	}
	switch quality {
	case "1080":
		args = append(args, "-f", "bv*[height<=1080]+ba/b[height<=1080]")
	case "720":
		args = append(args, "-f", "bv*[height<=720]+ba/b[height<=720]")
	case "audio":
		args = append(args, "-f", "bestaudio", "-x", "--audio-format", "m4a")
	}
	if cfg.CookiesFile != "" {
		args = append(args, "--cookies", cfg.CookiesFile)
	}
	return append(args, link)
}

// scanOutput đọc stdout từng dòng: bắt % tiến độ + đường dẫn file đích cuối cùng.
func scanOutput(r io.Reader, upd func(float64, string)) string {
	lastPath := ""
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if m := reProgress.FindStringSubmatch(line); m != nil {
			if p, err := strconv.ParseFloat(m[1], 64); err == nil {
				upd(p, line)
			}
		}
		if m := reDest.FindStringSubmatch(line); m != nil {
			lastPath = strings.TrimSpace(m[1])
		}
		if m := reMerger.FindStringSubmatch(line); m != nil {
			lastPath = strings.TrimSpace(m[1])
		}
		if m := reAlready.FindStringSubmatch(line); m != nil {
			lastPath = strings.TrimSpace(m[1])
		}
	}
	return lastPath
}

// newestFile trả file mới nhất trong dir có mtime sau mốc started (bỏ file tạm).
func newestFile(dir string, started time.Time) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	best, bestTime := "", time.Time{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".part" || ext == ".ytdl" || strings.HasPrefix(name, ".") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(started.Add(-2*time.Second)) && info.ModTime().After(bestTime) {
			best, bestTime = filepath.Join(dir, name), info.ModTime()
		}
	}
	return best
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "(không có thông báo lỗi)"
	}
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
