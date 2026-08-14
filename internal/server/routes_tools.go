package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bizstudio/internal/downloader"
	"bizstudio/internal/media"
	"bizstudio/internal/store"
	"bizstudio/internal/translate"
	"bizstudio/internal/tts"
	"bizstudio/internal/vox"
	"bizstudio/internal/whisper"
)

const toolJobTimeout = 2 * time.Hour

func (s *Server) routesTools(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/tools/download", s.handleToolDownload)
	mux.HandleFunc("POST /api/tools/asr", s.handleToolASR)
	mux.HandleFunc("POST /api/tools/ocr", s.handleToolOCR)
	mux.HandleFunc("POST /api/tools/translate", s.handleToolTranslate)
	mux.HandleFunc("GET /api/tools/voices", s.handleToolVoices)
	mux.HandleFunc("POST /api/tools/tts", s.handleToolTTS)
	mux.HandleFunc("POST /api/tools/scenes", s.handleToolScenes)
	mux.HandleFunc("POST /api/tools/vox", s.handleToolVox)
	mux.HandleFunc("POST /api/tools/autocut", s.handleToolAutocut)
	mux.HandleFunc("POST /api/tools/upload", s.handleToolUpload)
}

// handleToolUpload — nhận file (multipart "files") cho OCR/ASR/Dịch thuật,
// lưu vào data/uploads/, trả [{name, path, size}] với path tương đối DataDir.
func (s *Server) handleToolUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(2 << 30); err != nil {
		httpErr(w, http.StatusBadRequest, "không đọc được form upload: %v", err)
		return
	}
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		httpErr(w, http.StatusBadRequest, "chưa chọn file nào (field \"files\")")
		return
	}
	upDir := filepath.Join(s.DataDir, "uploads")
	if err := os.MkdirAll(upDir, 0o755); err != nil {
		httpErr(w, http.StatusInternalServerError, "không tạo được thư mục uploads: %v", err)
		return
	}
	type up struct {
		Name string `json:"name"`
		Path string `json:"path"`
		Size int64  `json:"size"`
	}
	out := make([]up, 0, len(files))
	for _, fh := range files {
		name := uniqueFileName(upDir, sanitizeFileName(fh.Filename))
		size, err := saveMultipartFile(fh, filepath.Join(upDir, name))
		if err != nil {
			httpErr(w, http.StatusInternalServerError, "lưu %s thất bại: %v", name, err)
			return
		}
		out = append(out, up{Name: name, Path: "uploads/" + name, Size: size})
	}
	s.Log("info", "upload", fmt.Sprintf("Đã nhận %d file vào uploads/", len(out)))
	writeJSON(w, http.StatusOK, out)
}

// ---------- path helpers ----------

// toolAbsPath: path tương đối → ghép vào DataDir; tuyệt đối → giữ nguyên.
func (s *Server) toolAbsPath(p string) string {
	p = strings.TrimSpace(p)
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(s.DataDir, p)
}

// toolRelPath: file nằm trong DataDir → đường dẫn tương đối (FE link /data/<path>);
// ngoài DataDir → đường dẫn tuyệt đối.
func (s *Server) toolRelPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	rel, err := filepath.Rel(s.DataDir, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return abs
	}
	return rel
}

// toolSrcPath resolve path + kiểm tra file nguồn tồn tại; đã ghi lỗi HTTP nếu fail.
func (s *Server) toolSrcPath(w http.ResponseWriter, raw string) (string, bool) {
	if strings.TrimSpace(raw) == "" {
		httpErr(w, http.StatusBadRequest, "thiếu đường dẫn file nguồn")
		return "", false
	}
	p := s.toolAbsPath(raw)
	fi, err := os.Stat(p)
	if err != nil {
		httpErr(w, http.StatusBadRequest, "không tìm thấy file: %s", p)
		return "", false
	}
	if fi.IsDir() {
		httpErr(w, http.StatusBadRequest, "đường dẫn là thư mục, cần chọn file: %s", p)
		return "", false
	}
	return p, true
}

// ---------- handlers ----------

// handleToolDownload — mỗi link 1 job kind=download, trả []Job.
func (s *Server) handleToolDownload(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Links   []string `json:"links"`
		Quality string   `json:"quality"`
		Threads int      `json:"threads"` // nhận để tương thích FE, yt-dlp tự quản lý
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	var links []string
	for _, l := range body.Links {
		if t := strings.TrimSpace(l); t != "" {
			links = append(links, t)
		}
	}
	if len(links) == 0 {
		httpErr(w, http.StatusBadRequest, "chưa nhập link nào để tải")
		return
	}

	out := make([]store.Job, 0, len(links))
	for _, link := range links {
		link := link
		j := s.Jobs.Submit("download", "", "Tải video: "+link, func(upd func(float64, string)) (string, error) {
			ctx, cancel := context.WithTimeout(context.Background(), toolJobTimeout)
			defer cancel()
			path, err := downloader.Download(ctx, s.st, link, body.Quality, upd)
			if err != nil {
				return "", err
			}
			return s.toolRelPath(path), nil
		})
		out = append(out, *j)
	}
	writeJSON(w, http.StatusOK, out)
}

// handleToolTranslate — text ngắn (<2000 ký tự) dịch đồng bộ; còn lại chạy job trên file.
func (s *Server) handleToolTranslate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path       string `json:"path"`
		Text       string `json:"text"`
		Mode       string `json:"mode"`
		Engine     string `json:"engine"`
		TargetLang string `json:"targetLang"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	text := strings.TrimSpace(body.Text)

	if text != "" && len(text) < 2000 {
		ctx, cancel := context.WithTimeout(r.Context(), 300*time.Second)
		defer cancel()
		result, err := translate.Text(ctx, s.st, text, body.Mode, body.Engine, body.TargetLang)
		if err != nil {
			httpErr(w, http.StatusInternalServerError, "dịch thất bại: %v", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"text": result})
		return
	}

	var src string
	if text != "" {
		// Văn bản dài → ghi ra file tạm rồi dịch như file.
		tmpDir := filepath.Join(s.DataDir, "tmp")
		if err := os.MkdirAll(tmpDir, 0o755); err != nil {
			httpErr(w, http.StatusInternalServerError, "không tạo được thư mục tạm: %v", err)
			return
		}
		src = filepath.Join(tmpDir, s.st.NewID("translate")+".txt")
		if err := os.WriteFile(src, []byte(text), 0o644); err != nil {
			httpErr(w, http.StatusInternalServerError, "không ghi được file tạm: %v", err)
			return
		}
	} else {
		var ok bool
		if src, ok = s.toolSrcPath(w, body.Path); !ok {
			return
		}
	}

	j := s.Jobs.Submit("translate", "", "Dịch: "+filepath.Base(src), func(upd func(float64, string)) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), toolJobTimeout)
		defer cancel()
		out, err := translate.File(ctx, s.st, src, body.Mode, body.Engine, body.TargetLang, upd)
		if err != nil {
			return "", err
		}
		return s.toolRelPath(out), nil
	})
	writeJSON(w, http.StatusOK, j)
}

// handleToolVoices — danh sách giọng đọc (macOS say + Gemini).
func (s *Server) handleToolVoices(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, tts.VoicesFor(s.st))
}

// handleToolTTS — job kind=tts, output=data/tmp/tts_<id>.wav.
func (s *Server) handleToolTTS(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text   string  `json:"text"`
		Voice  string  `json:"voice"`
		Rate   float64 `json:"rate"`
		Engine string  `json:"engine"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	if strings.TrimSpace(body.Text) == "" {
		httpErr(w, http.StatusBadRequest, "nội dung TTS trống")
		return
	}
	dst := filepath.Join(s.DataDir, "tmp", s.st.NewID("tts")+".wav")

	j := s.Jobs.Submit("tts", "", "Tạo giọng đọc: "+shortText(body.Text, 60), func(upd func(float64, string)) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		upd(10, "Đang tổng hợp giọng nói…")
		if err := tts.Speak(ctx, s.st, body.Text, body.Voice, body.Rate, body.Engine, dst); err != nil {
			return "", err
		}
		return s.toolRelPath(dst), nil
	})
	writeJSON(w, http.StatusOK, j)
}

// handleToolVox — job kind=vox render video từ danh sách cảnh.
func (s *Server) handleToolVox(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ProjectID string      `json:"projectId"`
		Scenes    []vox.Scene `json:"scenes"`
		Config    vox.Config  `json:"config"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	if len(body.Scenes) == 0 {
		httpErr(w, http.StatusBadRequest, "không có cảnh nào để render")
		return
	}

	pid := strings.TrimSpace(body.ProjectID)
	var workDir string
	if pid != "" {
		if _, ok := s.st.Project(pid); !ok {
			httpErr(w, http.StatusNotFound, "không tìm thấy dự án: %s", pid)
			return
		}
		workDir = filepath.Join(s.ProjectDir(pid), "outputs")
	} else {
		workDir = filepath.Join(s.DataDir, "tmp", s.st.NewID("vox"))
	}

	j := s.Jobs.Submit("vox", pid, "Render Vox: "+shortText(body.Scenes[0].Title, 50), func(upd func(float64, string)) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), toolJobTimeout)
		defer cancel()
		mp4, err := vox.Render(ctx, s.st, body.Scenes, body.Config, workDir, upd)
		if err != nil {
			return "", err
		}
		out := s.toolRelPath(mp4)
		if pid != "" {
			if p, ok := s.st.Project(pid); ok {
				p.OutputFile = out
				p.Status = "done"
				s.st.SaveProject(&p)
			}
		}
		return out, nil
	})
	writeJSON(w, http.StatusOK, j)
}

// handleToolAutocut — job kind=autocut, output=<src>.cut.mp4 cùng thư mục.
// guard (mặc định true) → dùng AutoCutGuarded: ngưỡng TỰ ĐO theo file và không
// cắt vào khoảng lặng nào chạm vào từ trong transcriptPath (file .words.json
// do bóc băng whisper sinh ra). guard=false → giữ nguyên cách cắt cũ.
func (s *Server) handleToolAutocut(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path           string  `json:"path"`
		SilenceDb      float64 `json:"silenceDb"`
		MinSilence     float64 `json:"minSilence"`
		TranscriptPath string  `json:"transcriptPath"`
		Guard          *bool   `json:"guard"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	src, ok := s.toolSrcPath(w, body.Path)
	if !ok {
		return
	}
	dst := strings.TrimSuffix(src, filepath.Ext(src)) + ".cut.mp4"

	guard := body.Guard == nil || *body.Guard
	if !guard {
		j := s.Jobs.Submit("autocut", "", "Cắt khoảng lặng: "+filepath.Base(src), func(upd func(float64, string)) (string, error) {
			ctx, cancel := context.WithTimeout(context.Background(), toolJobTimeout)
			defer cancel()
			if err := media.AutoCut(ctx, src, dst, body.SilenceDb, body.MinSilence, upd); err != nil {
				return "", err
			}
			return s.toolRelPath(dst), nil
		})
		writeJSON(w, http.StatusOK, j)
		return
	}

	// Transcript bảo vệ: đọc ngay để báo lỗi sớm nếu đường dẫn sai.
	var tr *whisper.Transcript
	if p := strings.TrimSpace(body.TranscriptPath); p != "" {
		abs, ok := s.toolSrcPath(w, p)
		if !ok {
			return
		}
		loaded, err := whisper.LoadJSON(abs)
		if err != nil {
			httpErr(w, http.StatusBadRequest, "%v", err)
			return
		}
		tr = loaded
	}

	opt := media.AutoCutOpt{SilenceDb: body.SilenceDb, MinSilence: body.MinSilence}
	j := s.Jobs.Submit("autocut", "", "Cắt khoảng lặng an toàn: "+filepath.Base(src), func(upd func(float64, string)) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), toolJobTimeout)
		defer cancel()
		rep, err := media.AutoCutGuarded(ctx, src, dst, tr, opt, upd)
		if err != nil {
			return "", err
		}
		upd(99, rep.Summary())
		s.Log("info", "autocut", filepath.Base(src)+" — "+rep.Summary())
		return s.toolRelPath(dst), nil
	})
	writeJSON(w, http.StatusOK, j)
}

// shortText rút gọn chuỗi hiển thị (theo rune, an toàn tiếng Việt).
func shortText(v string, n int) string {
	v = strings.TrimSpace(v)
	runes := []rune(v)
	if len(runes) > n {
		return string(runes[:n]) + "…"
	}
	return v
}

// toolSrcDir — bản dành cho THƯ MỤC của toolSrcPath. Tách riêng chứ không nới
// lỏng hàm kia: mọi công cụ khác đều nhận đúng một file, và cái chặn "đường dẫn
// là thư mục" ở đó bắt được lỗi gõ nhầm rất hay gặp.
func (s *Server) toolSrcDir(w http.ResponseWriter, raw string) (string, bool) {
	if strings.TrimSpace(raw) == "" {
		httpErr(w, http.StatusBadRequest, "thiếu đường dẫn thư mục")
		return "", false
	}
	p := s.toolAbsPath(raw)
	fi, err := os.Stat(p)
	if err != nil {
		httpErr(w, http.StatusBadRequest, "không tìm thấy thư mục: %s", p)
		return "", false
	}
	if !fi.IsDir() {
		httpErr(w, http.StatusBadRequest, "đường dẫn là file, cần chọn thư mục: %s", p)
		return "", false
	}
	return p, true
}
