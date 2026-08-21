package server

import (
	"context"
	"encoding/json"
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
	"bizstudio/internal/tts"
	"bizstudio/internal/util"
	"bizstudio/internal/whisper"
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

	// PUT /api/settings — GỘP vào cấu hình đang có, không thay cả khối.
	//
	// Trước đây đây là phép thay toàn bộ: gửi lên `{"veoModel":"x"}` là mọi
	// trường khác bị xoá sạch, kể cả API key. Giao diện luôn gửi đủ mọi trường
	// nên không lộ ra, nhưng bất cứ ai gọi API bằng curl/script — hoặc gọi
	// nhầm — đều mất trắng cấu hình mà không có cách khôi phục (store ghi đè
	// thẳng, không giữ bản sao lưu). Nay chỉ trường nào CÓ MẶT trong JSON gửi
	// lên mới bị đổi.
	mux.HandleFunc("PUT /api/settings", func(w http.ResponseWriter, r *http.Request) {
		cfg, err := mergeSettings(s.st.Settings(), r)
		if err != nil {
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
		"ffmpeg":  checkBinVersion(ctx, "ffmpeg", "-version"),
		"claude":  checkBinVersion(ctx, binOrDefault(cfg.ClaudeBin, "claude"), "--version"),
		"ytdlp":   checkYtdlp(ctx, binOrDefault(cfg.YtdlpBin, "yt-dlp")),
		"gemini":  checkGeminiKey(cfg),
		"openai":  checkOpenAI(cfg),
		"pexels":  checkPexels(cfg),
		"chrome":  checkChrome(ctx, cfg),
		"vieneu":  s.checkVieNeu(ctx),
		"whisper": s.checkWhisper(ctx),
	}
	writeJSON(w, http.StatusOK, res)
}

// checkOpenAI gọi GET {base}/models với Bearer key (API Trực Tiếp).
func checkOpenAI(cfg store.Settings) toolCheck {
	key := strings.TrimSpace(cfg.OpenAIKey)
	if key == "" {
		return toolCheck{OK: false, Detail: "chưa nhập API key (tab API Trực Tiếp)"}
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.OpenAIBase), "/")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	if !strings.HasSuffix(base, "/v1") && !strings.Contains(base, "/v1/") {
		base += "/v1"
	}
	req, _ := http.NewRequest(http.MethodGet, base+"/models", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return toolCheck{OK: false, Detail: "gọi API Trực Tiếp thất bại: " + truncateDetail(err.Error(), 200)}
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return toolCheck{OK: true, Detail: "kết nối API Trực Tiếp thành công (" + base + ")"}
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return toolCheck{OK: false, Detail: fmt.Sprintf("API Trực Tiếp lỗi (HTTP %d): %s", resp.StatusCode, truncateDetail(string(body), 200))}
}

// checkPexels gọi search 1 ảnh để xác thực key (Media Xu hướng).
func checkPexels(cfg store.Settings) toolCheck {
	key := strings.TrimSpace(cfg.PexelsKey)
	if key == "" {
		return toolCheck{OK: false, Detail: "chưa nhập Pexels API key (tab Media Xu hướng)"}
	}
	req, _ := http.NewRequest(http.MethodGet, "https://api.pexels.com/v1/search?query=nature&per_page=1", nil)
	req.Header.Set("Authorization", key)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return toolCheck{OK: false, Detail: "gọi Pexels thất bại: " + truncateDetail(err.Error(), 200)}
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return toolCheck{OK: true, Detail: "kết nối Pexels thành công"}
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return toolCheck{OK: false, Detail: fmt.Sprintf("Pexels lỗi (HTTP %d): %s", resp.StatusCode, truncateDetail(string(body), 200))}
}

// checkVieNeu kiểm tra venv VieNeu-TTS (import SDK, không nạp model).
func (s *Server) checkVieNeu(ctx context.Context) toolCheck {
	py := tts.VieNeuPythonPath(s.st)
	if py == "" {
		return toolCheck{OK: false, Detail: "chưa cài — bấm \"Cài\" ở mục Công cụ trên máy"}
	}
	out, err := tts.VieNeuRun(ctx, s.st, "-c", "import vieneu, importlib.metadata as m; print(m.version('vieneu'))")
	if err != nil {
		return toolCheck{OK: false, Detail: "venv có nhưng import lỗi: " + truncateDetail(err.Error(), 200)}
	}
	return toolCheck{OK: true, Detail: "VieNeu-TTS v" + strings.TrimSpace(out) + " (on-device, 48 kHz)"}
}

// checkWhisper kiểm tra venv faster-whisper (import SDK, không nạp model).
func (s *Server) checkWhisper(ctx context.Context) toolCheck {
	if !whisper.Available(s.st) {
		return toolCheck{OK: false, Detail: "chưa cài — bấm \"Cài\" ở mục Công cụ trên máy"}
	}
	out, err := whisper.Run(ctx, s.st,
		"-c", "import faster_whisper, importlib.metadata as m; print(m.version('faster-whisper'))")
	if err != nil {
		return toolCheck{OK: false, Detail: "venv có nhưng import lỗi: " + truncateDetail(err.Error(), 200)}
	}
	cfg := s.st.Settings()
	// Chỉ số liệu, không chữ mô tả: chuỗi này ghép động ở server nên lớp dịch —
	// vốn tra theo nguyên văn — không khớp được, chữ Việt sẽ lọt vào bản tiếng
	// Anh. Phần mô tả đã có ở dòng Desc của công cụ và dịch được bình thường.
	return toolCheck{OK: true, Detail: fmt.Sprintf("faster-whisper v%s · model %s · compute %s",
		strings.TrimSpace(out), cfg.WhisperModel, cfg.WhisperCompute)}
}

// ytdlpStaleDays — quá số ngày này thì nhắc cập nhật.
//
// yt-dlp ra bản mới sau mỗi một đến ba tuần vì YouTube liên tục đổi cách chống
// tải. Bản để lâu biểu hiện ra thành "HTTP Error 403: Forbidden" ngay giữa lúc
// tải — thông báo không hề nhắc đến chuyện phiên bản, nên người dùng tưởng mình
// bị chặn. Nói thẳng tuổi của bản đang dùng ở đây rẻ hơn nhiều so với việc họ
// ngồi đoán.
const ytdlpStaleDays = 30

// checkYtdlp kiểm tra yt-dlp và cảnh báo nếu bản đang cài đã cũ.
func checkYtdlp(ctx context.Context, bin string) toolCheck {
	res := checkBinVersion(ctx, bin, "--version")
	if !res.OK {
		return res
	}
	if d, ok := ytdlpAgeDays(res.Detail, time.Now()); ok && d >= ytdlpStaleDays {
		res.Detail = fmt.Sprintf("%s — bản này đã %d ngày tuổi. Lỗi 403 khi tải "+
			"thường là do bản cũ; bấm \"Cập nhật\" ở mục Công cụ.", res.Detail, d)
	}
	return res
}

// ytdlpAgeDays đọc số phiên bản dạng ngày (2026.08.19) và trả tuổi tính theo
// ngày. Dạng khác (bản tự dựng, nightly có hậu tố) trả ok=false — thà không nói
// gì còn hơn nói sai.
func ytdlpAgeDays(version string, now time.Time) (int, bool) {
	f := strings.SplitN(strings.TrimSpace(version), ".", 4)
	if len(f) < 3 {
		return 0, false
	}
	t, err := time.Parse("2006.01.02", strings.Join(f[:3], "."))
	if err != nil {
		return 0, false
	}
	d := int(now.Sub(t).Hours() / 24)
	if d < 0 {
		return 0, false
	}
	return d, true
}

// checkChrome dò trình duyệt render HTML Video.
func checkChrome(ctx context.Context, cfg store.Settings) toolCheck {
	bin := findChromeBin(cfg)
	if bin == "" {
		return toolCheck{OK: false, Detail: "không tìm thấy Google Chrome/Chromium — cài Chrome hoặc nhập đường dẫn ở Cấu hình"}
	}
	return checkBinVersion(ctx, bin, "--version")
}

// findChromeBin: ChromeBin cấu hình → đường dẫn macOS quen thuộc → PATH.
func findChromeBin(cfg store.Settings) string {
	candidates := []string{
		strings.TrimSpace(cfg.ChromeBin),
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"google-chrome", "chromium",
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if filepath.IsAbs(c) {
			if _, err := os.Stat(c); err == nil {
				return c
			}
			continue
		}
		if util.Exists(c) {
			return c
		}
	}
	return ""
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

// mergeSettings đọc JSON gửi lên và chỉ ghi đè những trường CÓ MẶT trong đó.
//
// Cách làm: mã hoá cấu hình hiện tại thành JSON, trộn với JSON gửi lên ở mức
// khoá, rồi giải mã lại. Nhờ vậy không phải liệt kê tay hàng chục trường —
// thêm trường mới vào Settings là tự động hoạt động, không sợ bỏ sót.
func mergeSettings(cur store.Settings, r *http.Request) (store.Settings, error) {
	var patch map[string]json.RawMessage
	if err := readJSON(r, &patch); err != nil {
		return cur, err
	}
	base, err := json.Marshal(cur)
	if err != nil {
		return cur, fmt.Errorf("đọc cấu hình hiện tại: %w", err)
	}
	var merged map[string]json.RawMessage
	if err := json.Unmarshal(base, &merged); err != nil {
		return cur, fmt.Errorf("đọc cấu hình hiện tại: %w", err)
	}
	for k, v := range patch {
		merged[k] = v
	}
	raw, err := json.Marshal(merged)
	if err != nil {
		return cur, fmt.Errorf("gộp cấu hình: %w", err)
	}
	var out store.Settings
	if err := json.Unmarshal(raw, &out); err != nil {
		return cur, fmt.Errorf("cấu hình gửi lên không hợp lệ: %w", err)
	}
	return out, nil
}
