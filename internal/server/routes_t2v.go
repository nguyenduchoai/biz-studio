package server

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"bizstudio/internal/store"
	"bizstudio/internal/text2video"
	"bizstudio/internal/vtemplate"
)

const (
	t2vFetchTimeout  = 60 * time.Second
	t2vScriptTimeout = 20 * time.Minute
	t2vBuildTimeout  = 2 * time.Hour
	t2vShotTimeout   = 10 * time.Minute
	t2vUploadTimeout = 3 * time.Minute
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
	mux.HandleFunc("POST /api/t2v/sessions/{id}/storyboard", s.handleT2VStoryboard)
	mux.HandleFunc("POST /api/t2v/sessions/{id}/segments/{idx}/image", s.handleT2VShot)
	mux.HandleFunc("POST /api/t2v/sessions/{id}/segments/{idx}/image/upload", s.handleT2VShotUpload)
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
		Name          string `json:"name"`
		Width         int    `json:"width"`
		Height        int    `json:"height"`
		FPS           int    `json:"fps"`
		TemplateID    string `json:"templateId"`
		TargetSeconds int    `json:"targetSeconds"`
	}
	if r.ContentLength != 0 {
		if err := readJSON(r, &body); err != nil {
			httpErr(w, http.StatusBadRequest, "%s", err)
			return
		}
	}

	// Khuôn điền hộ các ô còn trống, KHÔNG đè lên thứ người dùng đã gửi: họ mở
	// khuôn dọc rồi tự sửa thành ngang trước khi bấm tạo thì phải giữ ý họ.
	var tplName string
	if tpl, ok := vtemplate.Find(body.TemplateID); ok {
		tplName = tpl.Name
		if body.TargetSeconds <= 0 {
			body.TargetSeconds = tpl.Seconds
		}
		if body.Width <= 0 && body.Height <= 0 {
			body.Width, body.Height = vtemplate.AspectSize(tpl.Aspect)
		}
	} else {
		// id bịa thì bỏ hẳn, đừng lưu một tham chiếu chết vào phiên
		body.TemplateID = ""
	}

	name := strings.TrimSpace(body.Name)
	if name == "" {
		if tplName != "" {
			name = tplName + " " + time.Now().Format("02/01 15:04")
		} else {
			name = "Phiên " + time.Now().Format("02/01 15:04")
		}
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
		Segments:   []store.T2VSegment{},
		TemplateID: body.TemplateID, TargetSeconds: body.TargetSeconds,
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
		TemplateID    *string             `json:"templateId"`
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
	// Đổi khuôn: chuỗi rỗng = gỡ khuôn (hợp lệ), id bịa thì từ chối chứ đừng
	// lưu tham chiếu chết rồi im lặng bỏ qua lúc viết kịch bản.
	if body.TemplateID != nil {
		id := strings.TrimSpace(*body.TemplateID)
		if id != "" {
			if _, ok := vtemplate.Find(id); !ok {
				httpErr(w, http.StatusBadRequest, "không có khuôn %q", id)
				return
			}
		}
		sess.TemplateID = id
	}
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
		segs, err := text2video.WriteScript(ctx, s.st, cur.SourceText, cur.ScriptEngine, cur.ScriptModel, cur.TargetSeconds, cur.TemplateID)
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

// ---------- Storyboard: ảnh từng cảnh ----------

// handleT2VStoryboard — job kind=t2v_storyboard: sinh ảnh cho MỌI cảnh chưa có
// ảnh (cảnh đã có ảnh, nhất là ảnh tự tải lên, không bị ghi đè).
func (s *Server) handleT2VStoryboard(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.t2vSession(w, r)
	if !ok {
		return
	}
	if len(sess.Segments) == 0 {
		httpErr(w, http.StatusBadRequest, "chưa có kịch bản — hãy viết kịch bản trước khi tạo storyboard")
		return
	}
	// Còn cảnh thiếu ảnh thì mới cần nguồn ảnh; đủ ảnh rồi (ví dụ tự tải lên
	// hết) thì cho chạy để job báo "không cần tạo lại".
	if t2vShotCount(sess) < len(sess.Segments) {
		if err := text2video.CheckImageSource(s.st); err != nil {
			httpErr(w, http.StatusBadRequest, "%s", err)
			return
		}
	}
	id := sess.ID
	j := s.Jobs.Submit("t2v_storyboard", "", "Storyboard: "+shortText(sess.Name, 40),
		func(upd func(float64, string)) (string, error) {
			ctx, cancel := context.WithTimeout(context.Background(), t2vBuildTimeout)
			defer cancel()
			cur, ok := s.st.T2VSession(id)
			if !ok {
				return "", fmt.Errorf("phiên %q đã bị xoá", id)
			}
			err := text2video.BuildStoryboard(ctx, s.st, &cur, text2video.SessionDir(s.DataDir, id), upd)
			s.st.SaveT2VSession(&cur) // giữ lại ảnh + mô tả cảnh đã làm được
			if err != nil {
				s.Log("error", "text2video", fmt.Sprintf("Storyboard phiên %s lỗi: %v", id, err))
				return "", err
			}
			return fmt.Sprintf("%d/%d cảnh đã có ảnh", t2vShotCount(cur), len(cur.Segments)), nil
		})
	writeJSON(w, http.StatusOK, j)
}

// handleT2VShot — job kind=t2v_shot: sinh lại ảnh cho ĐÚNG một cảnh.
func (s *Server) handleT2VShot(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.t2vSession(w, r)
	if !ok {
		return
	}
	idx, ok := s.t2vSegmentIndex(w, r, sess)
	if !ok {
		return
	}
	if err := text2video.CheckImageSource(s.st); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	var body struct {
		Prompt string `json:"prompt"`
	}
	if r.ContentLength != 0 {
		if err := readJSON(r, &body); err != nil {
			httpErr(w, http.StatusBadRequest, "%s", err)
			return
		}
	}
	id, prompt := sess.ID, strings.TrimSpace(body.Prompt)
	j := s.Jobs.Submit("t2v_shot", "", fmt.Sprintf("Ảnh cảnh %d: %s", idx+1, shortText(sess.Name, 32)),
		func(upd func(float64, string)) (string, error) {
			ctx, cancel := context.WithTimeout(context.Background(), t2vShotTimeout)
			defer cancel()
			cur, ok := s.st.T2VSession(id)
			if !ok {
				return "", fmt.Errorf("phiên %q đã bị xoá", id)
			}
			upd(15, fmt.Sprintf("Đang tạo ảnh cho cảnh %d…", idx+1))
			if err := text2video.BuildSegmentImage(ctx, s.st, &cur, idx, prompt, text2video.SessionDir(s.DataDir, id)); err != nil {
				s.Log("error", "text2video", fmt.Sprintf("Tạo ảnh cảnh %d phiên %s lỗi: %v", idx+1, id, err))
				return "", err
			}
			s.st.SaveT2VSession(&cur)
			upd(98, fmt.Sprintf("Đã tạo ảnh cảnh %d", idx+1))
			return cur.Segments[idx].ImagePath, nil
		})
	writeJSON(w, http.StatusOK, j)
}

// handleT2VShotUpload — multipart "files" (1 ảnh): thay ảnh của một cảnh bằng
// ảnh người dùng tự tải lên (đồng bộ, trả về phiên đã cập nhật).
func (s *Server) handleT2VShotUpload(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.t2vSession(w, r)
	if !ok {
		return
	}
	idx, ok := s.t2vSegmentIndex(w, r, sess)
	if !ok {
		return
	}
	if err := r.ParseMultipartForm(256 << 20); err != nil {
		httpErr(w, http.StatusBadRequest, "không đọc được form upload: %v", err)
		return
	}
	if r.MultipartForm == nil || len(r.MultipartForm.File["files"]) == 0 {
		httpErr(w, http.StatusBadRequest, "chưa chọn ảnh (field \"files\") — chọn 1 file ảnh cho cảnh này")
		return
	}
	tmp, err := s.saveT2VUpload(r.MultipartForm.File["files"][0])
	if err != nil {
		httpErr(w, http.StatusInternalServerError, "%s", err)
		return
	}
	defer os.Remove(tmp)

	ctx, cancel := context.WithTimeout(r.Context(), t2vUploadTimeout)
	defer cancel()
	if err := text2video.ImportSegmentImage(ctx, &sess, idx, tmp, text2video.SessionDir(s.DataDir, sess.ID)); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	s.st.SaveT2VSession(&sess)
	s.Log("info", "text2video", fmt.Sprintf("Đã thay ảnh cảnh %d của phiên %q bằng ảnh tải lên", idx+1, sess.Name))
	writeJSON(w, http.StatusOK, sess)
}

// t2vSegmentIndex đọc số thứ tự đoạn (0-based) từ URL, tự trả 400 nếu sai.
func (s *Server) t2vSegmentIndex(w http.ResponseWriter, r *http.Request, sess store.T2VSession) (int, bool) {
	raw := strings.TrimSpace(r.PathValue("idx"))
	idx, err := strconv.Atoi(raw)
	if err != nil {
		httpErr(w, http.StatusBadRequest, "số thứ tự đoạn không hợp lệ: %q", raw)
		return 0, false
	}
	if idx < 0 || idx >= len(sess.Segments) {
		httpErr(w, http.StatusBadRequest, "không có đoạn số %d — phiên đang có %d đoạn", idx+1, len(sess.Segments))
		return 0, false
	}
	return idx, true
}

// saveT2VUpload lưu ảnh upload vào data/tmp để xử lý; trả đường dẫn file tạm.
func (s *Server) saveT2VUpload(fh *multipart.FileHeader) (string, error) {
	dir := filepath.Join(s.DataDir, "tmp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("không tạo được thư mục tạm: %w", err)
	}
	tmp := filepath.Join(dir, s.st.NewID("shotsrc")+filepath.Ext(sanitizeFileName(fh.Filename)))
	if _, err := saveMultipartFile(fh, tmp); err != nil {
		return "", err
	}
	return tmp, nil
}

// t2vShotCount đếm số cảnh đã có ảnh.
func t2vShotCount(sess store.T2VSession) int {
	n := 0
	for _, seg := range sess.Segments {
		if strings.TrimSpace(seg.ImagePath) != "" {
			n++
		}
	}
	return n
}

// t2vFail đánh dấu phiên lỗi + ghi nhật ký (đọc lại bản mới nhất để không đè kết quả khác).
func (s *Server) t2vFail(id string, err error) {
	if cur, ok := s.st.T2VSession(id); ok {
		cur.Status = "error"
		s.st.SaveT2VSession(&cur)
	}
	s.Log("error", "text2video", fmt.Sprintf("Phiên %s lỗi: %v", id, err))
}
