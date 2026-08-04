package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bizstudio/internal/media"
	"bizstudio/internal/store"
	"bizstudio/internal/veo"
)

// Routes cho Veo — sinh video bằng AI.
//
// Đây là module DUY NHẤT trong Biz Studio tiêu tiền thật của người dùng theo
// từng lần bấm, nên: (1) mọi endpoint đều trả kèm ước tính chi phí, (2) không
// có khoá thì từ chối ngay chứ không thử rồi mới báo lỗi.

const veoJobTimeout = 20 * time.Minute

func (s *Server) routesVeo(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/tools/veo", s.handleVeoInfo)
	mux.HandleFunc("POST /api/tools/veo/estimate", s.handleVeoEstimate)
	mux.HandleFunc("POST /api/tools/veo", s.handleVeoGenerate)
}

// handleVeoInfo — GET /api/tools/veo: model + bảng giá + đã có khoá chưa.
func (s *Server) handleVeoInfo(w http.ResponseWriter, r *http.Request) {
	c := veo.NewFromSettings(s.st)
	cfg := s.st.Settings()
	writeJSON(w, http.StatusOK, map[string]any{
		"ready":      c.Ready(),
		"model":      c.Model,
		"models":     veo.Models(),
		"durations":  veo.AllowedDurations,
		"resolution": firstNonEmpty(cfg.VeoResolution, veo.DefaultResolution),
		"seconds":    firstPositiveInt(cfg.VeoSeconds, veo.DefaultDuration),
		"usingGemini": strings.TrimSpace(cfg.VeoAPIKey) == "" &&
			strings.TrimSpace(cfg.GeminiAPIKey) != "",
		"note": "Veo tính phí theo giây video trên khoá Google của bạn và KHÔNG có bậc miễn phí — " +
			"dự án Google phải bật thanh toán. Chi phí hiện trước mỗi lần tạo là ước tính; hoá đơn thật do Google tính.",
	})
}

// handleVeoEstimate — POST /api/tools/veo/estimate: ước tính chi phí, không gọi
// Veo và không tốn đồng nào. Giao diện gọi mỗi khi người dùng đổi tuỳ chọn.
func (s *Server) handleVeoEstimate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Model      string `json:"model"`
		Resolution string `json:"resolution"`
		Seconds    int    `json:"seconds"`
		Count      int    `json:"count"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	model := strings.TrimSpace(body.Model)
	if model == "" {
		model = veo.NewFromSettings(s.st).Model
	}
	usd, err := veo.EstimateUSD(model, body.Resolution, body.Seconds, body.Count)
	if err != nil {
		// Không ước tính được thì nói thẳng, tuyệt đối không trả 0 —
		// giao diện sẽ hiện "miễn phí" trong khi thực tế vẫn bị tính tiền.
		httpErr(w, http.StatusBadRequest, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"usd":   usd,
		"model": model,
		"text":  fmt.Sprintf("khoảng $%.2f", usd),
	})
}

// handleVeoGenerate — POST /api/tools/veo: job tạo video.
// projectId có thì lưu thành asset của dự án, không thì để trong data/veo.
func (s *Server) handleVeoGenerate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Prompt     string `json:"prompt"`
		Negative   string `json:"negative"`
		Model      string `json:"model"`
		Aspect     string `json:"aspect"`
		Resolution string `json:"resolution"`
		Seconds    int    `json:"seconds"`
		ImagePath  string `json:"imagePath"`
		AllowAdult bool   `json:"allowAdult"`
		ProjectID  string `json:"projectId"`
		// Confirmed phải là true: người dùng đã nhìn thấy chi phí và đồng ý.
		// Thiếu cờ này thì từ chối — không ai bị trừ tiền vì gọi API nhầm.
		Confirmed bool `json:"confirmed"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	c := veo.NewFromSettings(s.st)
	if m := strings.TrimSpace(body.Model); m != "" {
		if _, ok := veo.FindModel(m); !ok {
			httpErr(w, http.StatusBadRequest, "không có model Veo %q", m)
			return
		}
		c.Model = m
	}
	if !c.Ready() {
		httpErr(w, http.StatusBadRequest, "%s", veo.ErrNoKey)
		return
	}
	if strings.TrimSpace(body.Prompt) == "" {
		httpErr(w, http.StatusBadRequest, "chưa nhập mô tả cảnh cần tạo")
		return
	}
	if !body.Confirmed {
		httpErr(w, http.StatusBadRequest,
			"cần xác nhận chi phí trước khi tạo video (Veo tính phí theo giây trên khoá của bạn)")
		return
	}
	usd, err := veo.EstimateUSD(c.Model, body.Resolution, body.Seconds, 1)
	if err != nil {
		httpErr(w, http.StatusBadRequest, "%v", err)
		return
	}

	img := ""
	if p := strings.TrimSpace(body.ImagePath); p != "" {
		abs, ok := s.toolSrcPath(w, p)
		if !ok {
			return
		}
		img = abs
	}
	dst := s.veoOutPath(body.ProjectID, body.Prompt)
	opts := veo.Opts{
		Prompt: body.Prompt, Negative: body.Negative,
		Aspect: body.Aspect, Resolution: body.Resolution, Seconds: body.Seconds,
		ImagePath: img, AllowAdult: body.AllowAdult,
	}

	title := fmt.Sprintf("Veo — %s (ước tính $%.2f)", shortText(body.Prompt, 40), usd)
	j := s.Jobs.Submit("veo", body.ProjectID, title, func(upd func(float64, string)) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), veoJobTimeout)
		defer cancel()
		spent, err := c.Generate(ctx, opts, dst, upd)
		if err != nil {
			return "", err
		}
		s.Log("info", "veo", fmt.Sprintf("Đã tạo video Veo (%s) — chi phí ước tính $%.2f: %s",
			c.Model, spent, filepath.Base(dst)))
		if body.ProjectID != "" {
			s.attachVeoAsset(body.ProjectID, dst)
		}
		return s.toolRelPath(dst), nil
	})
	writeJSON(w, http.StatusOK, j)
}

// attachVeoAsset đưa clip vừa tạo vào thư viện media của dự án, để dùng ngay ở
// Studio Editor / ghép cảnh mà không phải tìm file thủ công.
func (s *Server) attachVeoAsset(projectID, path string) {
	info, err := os.Stat(path)
	if err != nil {
		s.Log("warn", "veo", "Không đọc được file video vừa tạo: "+err.Error())
		return
	}
	a := store.Asset{
		ProjectID: projectID,
		Kind:      "video",
		Name:      filepath.Base(path),
		Path:      s.toolRelPath(path),
		Size:      info.Size(),
	}
	if probed, perr := media.Probe(path); perr == nil {
		a.Duration = probed.Duration
	}
	s.st.SaveAsset(&a)
}

// veoOutPath — nơi lưu clip: trong dự án nếu có, không thì data/veo.
func (s *Server) veoOutPath(projectID, prompt string) string {
	name := uniqueFileName(s.veoDir(projectID), sanitizeFileName(shortText(prompt, 40))+".mp4")
	return filepath.Join(s.veoDir(projectID), name)
}

func (s *Server) veoDir(projectID string) string {
	if strings.TrimSpace(projectID) != "" {
		return filepath.Join(s.DataDir, "projects", projectID, "veo")
	}
	return filepath.Join(s.DataDir, "veo")
}

func firstNonEmpty(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func firstPositiveInt(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}
