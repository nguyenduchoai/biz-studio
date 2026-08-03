package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"bizstudio/internal/store"
	"bizstudio/internal/text2video"
)

const (
	t2vFetchTimeout  = 60 * time.Second
	t2vScriptTimeout = 20 * time.Minute
	t2vBuildTimeout  = 2 * time.Hour
)

// routesT2V — Text → Video: phiên làm việc (nguồn → kịch bản → giọng đọc → dựng video).
func (s *Server) routesT2V(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/t2v/sessions", s.handleT2VList)
	mux.HandleFunc("POST /api/t2v/sessions", s.handleT2VCreate)
	mux.HandleFunc("GET /api/t2v/sessions/{id}", s.handleT2VGet)
	mux.HandleFunc("PUT /api/t2v/sessions/{id}", s.handleT2VUpdate)
	mux.HandleFunc("DELETE /api/t2v/sessions/{id}", s.handleT2VDelete)
	mux.HandleFunc("POST /api/t2v/sessions/{id}/fetch", s.handleT2VFetch)
	mux.HandleFunc("POST /api/t2v/sessions/{id}/script", s.handleT2VScript)
	mux.HandleFunc("POST /api/t2v/sessions/{id}/voice", s.handleT2VVoice)
	mux.HandleFunc("POST /api/t2v/sessions/{id}/build", s.handleT2VBuild)
}

// t2vSession lấy phiên theo path param, tự trả 404 tiếng Việt nếu không có.
func (s *Server) t2vSession(w http.ResponseWriter, r *http.Request) (store.T2VSession, bool) {
	id := r.PathValue("id")
	sess, ok := s.st.T2VSession(id)
	if !ok {
		httpErr(w, http.StatusNotFound, "không tìm thấy phiên Text → Video %q", id)
		return store.T2VSession{}, false
	}
	return sess, true
}

func (s *Server) handleT2VList(w http.ResponseWriter, r *http.Request) {
	list := s.st.T2VSessions()
	if list == nil {
		list = []store.T2VSession{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleT2VCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name   string `json:"name"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
		FPS    int    `json:"fps"`
	}
	if r.ContentLength != 0 {
		if err := readJSON(r, &body); err != nil {
			httpErr(w, http.StatusBadRequest, "%s", err)
			return
		}
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = "Phiên " + time.Now().Format("02/01 15:04")
	}
	if body.Width <= 0 {
		body.Width = 1080
	}
	if body.Height <= 0 {
		body.Height = 1920
	}
	if body.FPS <= 0 {
		body.FPS = 30
	}
	sess := store.T2VSession{
		Name: name, SourceKind: "text",
		Width: body.Width, Height: body.Height, FPS: body.FPS,
		Status: "draft", Step: 1, BuildMode: "html",
		Segments: []store.T2VSegment{},
	}
	s.st.SaveT2VSession(&sess)
	if err := os.MkdirAll(text2video.SessionDir(s.DataDir, sess.ID), 0o755); err != nil {
		httpErr(w, http.StatusInternalServerError, "không tạo được thư mục phiên: %v", err)
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

func (s *Server) handleT2VGet(w http.ResponseWriter, r *http.Request) {
	if sess, ok := s.t2vSession(w, r); ok {
		writeJSON(w, http.StatusOK, sess)
	}
}

// handleT2VUpdate — chỉ ghi các field editable; field không gửi thì giữ nguyên.
func (s *Server) handleT2VUpdate(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.t2vSession(w, r)
	if !ok {
		return
	}
	var body struct {
		Name          *string             `json:"name"`
		SourceKind    *string             `json:"sourceKind"`
		SourceURL     *string             `json:"sourceUrl"`
		SourceText    *string             `json:"sourceText"`
		ScriptEngine  *string             `json:"scriptEngine"`
		ScriptModel   *string             `json:"scriptModel"`
		TargetSeconds *int                `json:"targetSeconds"`
		Segments      *[]store.T2VSegment `json:"segments"`
		VoiceID       *string             `json:"voiceId"`
		VoiceEngine   *string             `json:"voiceEngine"`
		VoiceStyle    *string             `json:"voiceStyle"`
		Width         *int                `json:"width"`
		Height        *int                `json:"height"`
		FPS           *int                `json:"fps"`
		Step          *int                `json:"step"`
		BuildMode     *string             `json:"buildMode"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	setStr := func(dst *string, v *string) {
		if v != nil {
			*dst = strings.TrimSpace(*v)
		}
	}
	setInt := func(dst *int, v *int) {
		if v != nil {
			*dst = *v
		}
	}
	setStr(&sess.Name, body.Name)
	setStr(&sess.SourceKind, body.SourceKind)
	setStr(&sess.SourceURL, body.SourceURL)
	setStr(&sess.ScriptEngine, body.ScriptEngine)
	setStr(&sess.ScriptModel, body.ScriptModel)
	setStr(&sess.VoiceID, body.VoiceID)
	setStr(&sess.VoiceEngine, body.VoiceEngine)
	setStr(&sess.VoiceStyle, body.VoiceStyle)
	setStr(&sess.BuildMode, body.BuildMode)
	setInt(&sess.TargetSeconds, body.TargetSeconds)
	setInt(&sess.Width, body.Width)
	setInt(&sess.Height, body.Height)
	setInt(&sess.FPS, body.FPS)
	setInt(&sess.Step, body.Step)
	if body.SourceText != nil {
		sess.SourceText = strings.TrimSpace(*body.SourceText)
	}
	if body.Segments != nil {
		segs, changed := text2video.NormalizeSegments(sess.Segments, *body.Segments)
		sess.Segments = segs
		if changed {
			// Kịch bản đổi → giọng đọc cũ không còn khớp, buộc đọc lại.
			text2video.InvalidateVoice(&sess)
		}
	}
	s.st.SaveT2VSession(&sess)
	writeJSON(w, http.StatusOK, sess)
}

func (s *Server) handleT2VDelete(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.t2vSession(w, r)
	if !ok {
		return
	}
	s.st.DeleteT2VSession(sess.ID)
	if err := os.RemoveAll(text2video.SessionDir(s.DataDir, sess.ID)); err != nil {
		s.Log("warn", "text2video", fmt.Sprintf("Xoá thư mục phiên %s thất bại: %v", sess.ID, err))
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleT2VFetch — lấy nội dung bài viết từ link (đồng bộ, timeout 60s).
func (s *Server) handleT2VFetch(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.t2vSession(w, r)
	if !ok {
		return
	}
	var body struct {
		URL string `json:"url"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), t2vFetchTimeout)
	defer cancel()

	title, text, err := text2video.FetchArticle(ctx, body.URL)
	if err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	sess.SourceKind = "link"
	sess.SourceURL = strings.TrimSpace(body.URL)
	sess.SourceText = text
	if title != "" && strings.HasPrefix(sess.Name, "Phiên ") {
		sess.Name = shortText(title, 80)
	}
	if sess.Step < 1 {
		sess.Step = 1
	}
	s.st.SaveT2VSession(&sess)
	s.Log("info", "text2video", fmt.Sprintf("Đã lấy nội dung từ %s (%d ký tự)", sess.SourceURL, utf8.RuneCountInString(text)))
	writeJSON(w, http.StatusOK, sess)
}

// handleT2VScript — job kind=t2v_script: LLM viết kịch bản đọc, lưu segments.
func (s *Server) handleT2VScript(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.t2vSession(w, r)
	if !ok {
		return
	}
	if strings.TrimSpace(sess.SourceText) == "" {
		httpErr(w, http.StatusBadRequest, "chưa có nội dung nguồn — dán văn bản hoặc lấy nội dung từ link trước")
		return
	}
	id := sess.ID
	j := s.Jobs.Submit("t2v_script", "", "Viết kịch bản: "+shortText(sess.Name, 40), func(upd func(float64, string)) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), t2vScriptTimeout)
		defer cancel()
		cur, ok := s.st.T2VSession(id)
		if !ok {
			return "", fmt.Errorf("phiên %q đã bị xoá", id)
		}
		upd(10, "Đang nhờ AI viết kịch bản đọc…")
		segs, err := text2video.WriteScript(ctx, s.st, cur.SourceText, cur.ScriptEngine, cur.ScriptModel, cur.TargetSeconds)
		if err != nil {
			s.t2vFail(id, err)
			return "", err
		}
		cur.Segments = segs
		cur.Status = "script"
		if cur.Step < 2 {
			cur.Step = 2
		}
		s.st.SaveT2VSession(&cur)
		upd(98, fmt.Sprintf("Đã viết %d đoạn", len(segs)))
		return fmt.Sprintf("%d đoạn kịch bản", len(segs)), nil
	})
	writeJSON(w, http.StatusOK, j)
}

// handleT2VVoice — job kind=t2v_voice: đọc từng đoạn, đo thời lượng thật, ghép voice.wav.
func (s *Server) handleT2VVoice(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.t2vSession(w, r)
	if !ok {
		return
	}
	if len(sess.Segments) == 0 {
		httpErr(w, http.StatusBadRequest, "chưa có kịch bản — hãy viết kịch bản trước khi tạo giọng đọc")
		return
	}
	id := sess.ID
	j := s.Jobs.Submit("t2v_voice", "", "Giọng đọc: "+shortText(sess.Name, 40), func(upd func(float64, string)) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), t2vBuildTimeout)
		defer cancel()
		cur, ok := s.st.T2VSession(id)
		if !ok {
			return "", fmt.Errorf("phiên %q đã bị xoá", id)
		}
		if err := text2video.BuildVoice(ctx, s.st, &cur, text2video.SessionDir(s.DataDir, id), upd); err != nil {
			s.t2vFail(id, err)
			return "", err
		}
		cur.Status = "voice"
		if cur.Step < 3 {
			cur.Step = 3
		}
		s.st.SaveT2VSession(&cur)
		return cur.VoicePath, nil
	})
	writeJSON(w, http.StatusOK, j)
}

// handleT2VBuild — job kind=t2v_build: mode "html" render bằng HTML Video,
// mode "ai" tạo dự án + khởi động phiên AI (output = projectID).
func (s *Server) handleT2VBuild(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.t2vSession(w, r)
	if !ok {
		return
	}
	var body struct {
		Mode string `json:"mode"`
	}
	if r.ContentLength != 0 {
		if err := readJSON(r, &body); err != nil {
			httpErr(w, http.StatusBadRequest, "%s", err)
			return
		}
	}
	mode := strings.ToLower(strings.TrimSpace(body.Mode))
	if mode == "" {
		mode = strings.ToLower(strings.TrimSpace(sess.BuildMode))
	}
	if mode == "" {
		mode = "html"
	}
	if mode != "html" && mode != "ai" {
		httpErr(w, http.StatusBadRequest, "chế độ dựng video không hợp lệ: %q (chỉ hỗ trợ \"html\" hoặc \"ai\")", body.Mode)
		return
	}
	if strings.TrimSpace(sess.VoicePath) == "" {
		httpErr(w, http.StatusBadRequest, "chưa có giọng đọc — hãy chạy bước Giọng đọc trước khi dựng video")
		return
	}

	id := sess.ID
	label := map[string]string{"html": "HTML Video", "ai": "AI dựng"}[mode]
	j := s.Jobs.Submit("t2v_build", "", fmt.Sprintf("Dựng video (%s): %s", label, shortText(sess.Name, 40)),
		func(upd func(float64, string)) (string, error) {
			ctx, cancel := context.WithTimeout(context.Background(), t2vBuildTimeout)
			defer cancel()
			cur, ok := s.st.T2VSession(id)
			if !ok {
				return "", fmt.Errorf("phiên %q đã bị xoá", id)
			}
			cur.Status = "building"
			cur.BuildMode = mode
			s.st.SaveT2VSession(&cur)

			out, err := s.t2vRunBuild(ctx, &cur, mode, upd)
			if err != nil {
				s.t2vFail(id, err)
				return "", err
			}
			s.st.SaveT2VSession(&cur)
			return out, nil
		})
	writeJSON(w, http.StatusOK, j)
}

// t2vRunBuild chạy đúng chế độ dựng và cập nhật các field kết quả của phiên.
func (s *Server) t2vRunBuild(ctx context.Context, cur *store.T2VSession, mode string, upd func(float64, string)) (string, error) {
	if mode == "html" {
		mp4, err := text2video.BuildVideoHTML(ctx, s.st, cur, text2video.SessionDir(s.DataDir, cur.ID), upd)
		if err != nil {
			return "", err
		}
		cur.OutputPath = s.toolRelPath(mp4)
		cur.Status = "done"
		cur.Step = 5
		return cur.OutputPath, nil
	}

	upd(10, "Tạo dự án và chuẩn bị nguyên liệu…")
	pid, err := text2video.BuildProject(ctx, s.st, cur, filepath.Join(s.DataDir, "projects"))
	if err != nil {
		return "", err
	}
	s.ProjectDir(pid) // đảm bảo đủ thư mục con của dự án
	cur.ProjectID = pid
	cur.Step = 5
	s.st.SaveT2VSession(cur)

	upd(60, "Khởi động phiên AI dựng video…")
	if _, err := s.runner().Start(pid, ""); err != nil {
		return "", fmt.Errorf("đã tạo dự án %s nhưng không khởi động được phiên AI: %w", pid, err)
	}
	cur.Status = "building" // AI đang dựng, dự án theo dõi tiếp ở trang Dự án
	upd(95, "Phiên AI đang chạy — mở dự án để theo dõi")
	return pid, nil
}

// t2vFail đánh dấu phiên lỗi + ghi nhật ký (đọc lại bản mới nhất để không đè kết quả khác).
func (s *Server) t2vFail(id string, err error) {
	if cur, ok := s.st.T2VSession(id); ok {
		cur.Status = "error"
		s.st.SaveT2VSession(&cur)
	}
	s.Log("error", "text2video", fmt.Sprintf("Phiên %s lỗi: %v", id, err))
}
