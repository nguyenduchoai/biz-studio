package server

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"math"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bizstudio/internal/store"
	"bizstudio/internal/util"
)

// toolCheck — kết quả kiểm tra 1 công cụ trong POST /api/settings/test.
type toolCheck struct {
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

func (s *Server) routesSettings(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/settings", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, s.st.Settings())
	})

	mux.HandleFunc("PUT /api/settings", func(w http.ResponseWriter, r *http.Request) {
		var cfg store.Settings
		if err := readJSON(r, &cfg); err != nil {
			httpErr(w, http.StatusBadRequest, "%s", err)
			return
		}
		s.st.SaveSettings(cfg)
		// Trả bản đã áp defaults (SaveSettings tự áp trong store).
		writeJSON(w, http.StatusOK, s.st.Settings())
	})

	mux.HandleFunc("POST /api/settings/test", s.handleSettingsTest)
	mux.HandleFunc("POST /api/settings/cleanup", s.handleSettingsCleanup)
}

// handleSettingsTest kiểm tra ffmpeg / claude / yt-dlp / Gemini API.
func (s *Server) handleSettingsTest(w http.ResponseWriter, r *http.Request) {
	cfg := s.st.Settings()
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	res := map[string]toolCheck{
		"ffmpeg": checkBinVersion(ctx, "ffmpeg", "-version"),
		"claude": checkBinVersion(ctx, binOrDefault(cfg.ClaudeBin, "claude"), "--version"),
		"ytdlp":  checkBinVersion(ctx, binOrDefault(cfg.YtdlpBin, "yt-dlp"), "--version"),
		"gemini": checkGeminiKey(cfg),
	}
	writeJSON(w, http.StatusOK, res)
}

func binOrDefault(bin, def string) string {
	if strings.TrimSpace(bin) == "" {
		return def
	}
	return strings.TrimSpace(bin)
}

// checkBinVersion chạy "<bin> <args>" và lấy dòng đầu output làm detail.
func checkBinVersion(ctx context.Context, bin string, args ...string) toolCheck {
	out, err := util.Run(ctx, bin, args...)
	if err != nil {
		return toolCheck{OK: false, Detail: truncateDetail(err.Error(), 300)}
	}
	line := strings.TrimSpace(strings.SplitN(out, "\n", 2)[0])
	if line == "" {
		line = "hoạt động bình thường"
	}
	return toolCheck{OK: true, Detail: truncateDetail(line, 200)}
}

// checkGeminiKey gọi GET {base}/v1beta/models?key=...&pageSize=1 (timeout 15s).
func checkGeminiKey(cfg store.Settings) toolCheck {
	key := strings.TrimSpace(cfg.GeminiAPIKey)
	if key == "" {
		return toolCheck{OK: false, Detail: "chưa nhập API key"}
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.GeminiBase), "/")
	if base == "" {
		base = "https://generativelanguage.googleapis.com"
	}
	endpoint := fmt.Sprintf("%s/v1beta/models?key=%s&pageSize=1", base, neturl.QueryEscape(key))

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		return toolCheck{OK: false, Detail: "gọi Gemini API thất bại: " + truncateDetail(err.Error(), 200)}
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return toolCheck{OK: true, Detail: "kết nối Gemini API thành công"}
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return toolCheck{OK: false, Detail: fmt.Sprintf("Gemini API lỗi (HTTP %d): %s", resp.StatusCode, truncateDetail(string(body), 200))}
}

// handleSettingsCleanup xoá nội dung data/tmp + projects/*/tmp, trả {freedMB}.
func (s *Server) handleSettingsCleanup(w http.ResponseWriter, r *http.Request) {
	dirs := []string{filepath.Join(s.DataDir, "tmp")}
	projRoot := filepath.Join(s.DataDir, "projects")
	if entries, err := os.ReadDir(projRoot); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				dirs = append(dirs, filepath.Join(projRoot, e.Name(), "tmp"))
			}
		}
	}

	var freed int64
	for _, d := range dirs {
		n, err := cleanupDirContents(d)
		freed += n
		if err != nil {
			httpErr(w, http.StatusInternalServerError, "dọn dẹp %s thất bại: %v", d, err)
			return
		}
	}
	mb := math.Round(float64(freed)/(1024*1024)*10) / 10
	s.Log("info", "settings", fmt.Sprintf("Đã dọn dẹp file tạm, giải phóng %.1f MB", mb))
	writeJSON(w, http.StatusOK, map[string]any{"freedMB": mb})
}

// cleanupDirContents xoá mọi mục con trong dir (giữ lại dir), trả tổng byte đã giải phóng.
func cleanupDirContents(dir string) (int64, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	var freed int64
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		freed += pathSize(p)
		if err := os.RemoveAll(p); err != nil {
			return freed, err
		}
	}
	return freed, nil
}

// pathSize tính tổng dung lượng file của một path (file hoặc thư mục đệ quy).
func pathSize(p string) int64 {
	var total int64
	_ = filepath.WalkDir(p, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, e := d.Info(); e == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

func truncateDetail(v string, n int) string {
	v = strings.TrimSpace(v)
	if len(v) > n {
		return v[:n] + "…"
	}
	return v
}
