package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bizstudio/internal/consent"
	"bizstudio/internal/media"
	"bizstudio/internal/store"
	"bizstudio/internal/tts"
	"bizstudio/internal/util"
)

const (
	cloneMinDuration  = 2.0  // giây
	cloneMaxDuration  = 20.0 // giây
	cloneSampleHint   = "clip mẫu nên dài 3–8 giây, giọng rõ, không nhạc nền"
	cloneSetupHint    = "chưa cài VieNeu-TTS — chạy ./scripts/setup-vieneu.sh rồi thử lại (giọng nhân bản cần VieNeu)"
	clonePreviewText  = "Xin chào, đây là giọng nhân bản của bạn từ Biz Studio."
	cloneConvertLimit = 5 * time.Minute
)

func (s *Server) routesClone(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/tools/clone-voices", s.handleCloneList)
	mux.HandleFunc("POST /api/tools/clone-voices", s.handleCloneCreate)
	mux.HandleFunc("DELETE /api/tools/clone-voices/{id}", s.handleCloneDelete)
	mux.HandleFunc("POST /api/tools/clone-voices/{id}/preview", s.handleClonePreview)
}

// handleCloneList — danh sách giọng nhân bản (mới nhất trước).
func (s *Server) handleCloneList(w http.ResponseWriter, r *http.Request) {
	list := s.st.CloneVoices()
	if list == nil {
		list = []store.CloneVoice{}
	}
	writeJSON(w, http.StatusOK, list)
}

// handleCloneCreate — multipart: files (1 clip mẫu audio/video), name (bắt buộc), gender, note.
// Clip được chuyển sang wav mono 44100 lưu tại data/vieneu/clones/<id>.wav.
func (s *Server) handleCloneCreate(w http.ResponseWriter, r *http.Request) {
	if !tts.VieNeuAvailable(s.st) {
		httpErr(w, http.StatusBadRequest, "%s", cloneSetupHint)
		return
	}
	if err := r.ParseMultipartForm(2 << 30); err != nil {
		httpErr(w, http.StatusBadRequest, "không đọc được form upload: %v", err)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		httpErr(w, http.StatusBadRequest, "cần đặt tên cho giọng nhân bản")
		return
	}

	// Cổng xác nhận quyền — chặn TRƯỚC khi đụng tới file upload.
	//
	// Chặn ở đây chứ không chỉ ở giao diện: gọi API bằng curl hay script cũng
	// phải đi qua cùng một cổng. Một ô tích ở giao diện chỉ chặn người dùng
	// giao diện.
	grant := consent.Grant{
		Kind:      consent.KindVoice,
		Rights:    r.FormValue("rights") == "true",
		Adult:     r.FormValue("adult") == "true",
		Permitted: r.FormValue("permitted") == "true",
		Subject:   strings.TrimSpace(r.FormValue("subject")),
		At:        time.Now(),
	}
	if err := consent.Check(grant, consent.KindVoice); err != nil {
		httpErr(w, http.StatusForbidden, "%s", err)
		return
	}
	s.Log("info", "clone", consent.Line(grant, consent.KindVoice)+" · giọng: "+name)
	if r.MultipartForm == nil || len(r.MultipartForm.File["files"]) == 0 {
		httpErr(w, http.StatusBadRequest, "chưa chọn clip mẫu (field \"files\") — %s", cloneSampleHint)
		return
	}

	tmp, err := s.saveCloneUpload(r.MultipartForm.File["files"][0])
	if err != nil {
		httpErr(w, http.StatusInternalServerError, "%s", err)
		return
	}
	defer os.Remove(tmp)

	// Chặn sớm clip quá dài/quá ngắn trước khi tốn công chuyển đổi.
	if info, perr := media.Probe(tmp); perr == nil && info.Duration > 0 && !cloneDurationOK(info.Duration) {
		httpErr(w, http.StatusBadRequest, "%s", cloneDurationMsg(info.Duration))
		return
	}

	id := s.st.NewID("clv")
	wav, dur, err := s.convertCloneSample(tmp, id)
	if err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	if !cloneDurationOK(dur) {
		_ = os.Remove(wav)
		httpErr(w, http.StatusBadRequest, "%s", cloneDurationMsg(dur))
		return
	}

	c := store.CloneVoice{
		ID:        id,
		Name:      name,
		Path:      tts.CloneRelPath(id),
		Gender:    strings.TrimSpace(r.FormValue("gender")),
		Note:      strings.TrimSpace(r.FormValue("note")),
		Duration:  dur,
		CreatedAt: time.Now(),
	}
	s.st.SaveCloneVoice(&c)
	s.Log("info", "clone", fmt.Sprintf("Đã tạo giọng nhân bản %q (%.1f giây)", c.Name, dur))
	writeJSON(w, http.StatusOK, c)
}

// handleCloneDelete — xoá record + file clip mẫu.
func (s *Server) handleCloneDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, ok := s.st.CloneVoice(id)
	if !ok {
		httpErr(w, http.StatusNotFound, "không tìm thấy giọng nhân bản %q", id)
		return
	}
	if strings.TrimSpace(c.Path) != "" {
		p := filepath.Join(s.DataDir, filepath.FromSlash(c.Path))
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			httpErr(w, http.StatusInternalServerError, "không xoá được clip mẫu: %v", err)
			return
		}
	}
	s.st.DeleteCloneVoice(id)
	s.Log("info", "clone", fmt.Sprintf("Đã xoá giọng nhân bản %q", c.Name))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleClonePreview — nghe thử: Job kind=tts đọc text bằng giọng clone:<id>.
func (s *Server) handleClonePreview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, ok := s.st.CloneVoice(id)
	if !ok {
		httpErr(w, http.StatusNotFound, "không tìm thấy giọng nhân bản %q", id)
		return
	}
	if !tts.VieNeuAvailable(s.st) {
		httpErr(w, http.StatusBadRequest, "%s", cloneSetupHint)
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := cloneDecodeBody(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	text := strings.TrimSpace(body.Text)
	if text == "" {
		text = clonePreviewText
	}
	dst := filepath.Join(s.DataDir, "tmp", s.st.NewID("tts")+".wav")

	j := s.Jobs.Submit("tts", "", "Nghe thử giọng nhân bản: "+shortText(c.Name, 40), func(upd func(float64, string)) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		upd(10, "Đang tổng hợp giọng nhân bản…")
		if err := tts.Speak(ctx, s.st, text, tts.CloneVoiceID(c.ID), 0, "vieneu", dst); err != nil {
			return "", err
		}
		return s.toolRelPath(dst), nil
	})
	writeJSON(w, http.StatusOK, j)
}

// ---------- helpers ----------

// saveCloneUpload lưu clip mẫu vào data/tmp để ffmpeg xử lý; trả đường dẫn file tạm.
func (s *Server) saveCloneUpload(fh *multipart.FileHeader) (string, error) {
	dir := filepath.Join(s.DataDir, "tmp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("không tạo được thư mục tạm: %w", err)
	}
	tmp := filepath.Join(dir, s.st.NewID("clonesrc")+filepath.Ext(sanitizeFileName(fh.Filename)))
	if _, err := saveMultipartFile(fh, tmp); err != nil {
		return "", err
	}
	return tmp, nil
}

// convertCloneSample chuyển clip mẫu sang wav mono 44100 tại vieneu/clones/<id>.wav,
// trả đường dẫn wav + thời lượng (giây) đo bằng ffprobe.
func (s *Server) convertCloneSample(src, id string) (string, float64, error) {
	dir, err := tts.CloneDir(s.st)
	if err != nil {
		return "", 0, err
	}
	dst := filepath.Join(dir, id+".wav")

	ctx, cancel := context.WithTimeout(context.Background(), cloneConvertLimit)
	defer cancel()
	if _, se, err := util.RunErr(ctx, "ffmpeg", "-y", "-i", src,
		"-vn", "-ac", "1", "-ar", "44100", dst); err != nil {
		_ = os.Remove(dst)
		return "", 0, fmt.Errorf("không xử lý được clip mẫu (ffmpeg): %v — %s (clip phải có tiếng nói; %s)",
			err, cloneStderrTail(se), cloneSampleHint)
	}
	info, err := media.Probe(dst)
	if err != nil {
		_ = os.Remove(dst)
		return "", 0, fmt.Errorf("không đọc được thời lượng clip mẫu: %v", err)
	}
	return dst, info.Duration, nil
}

// cloneDurationOK — thời lượng clip mẫu nằm trong khoảng cho phép.
func cloneDurationOK(d float64) bool { return d >= cloneMinDuration && d <= cloneMaxDuration }

// cloneDurationMsg — thông báo lỗi thời lượng kèm hướng dẫn.
func cloneDurationMsg(d float64) string {
	return fmt.Sprintf("clip mẫu dài %.1f giây — chỉ nhận từ %.0f đến %.0f giây (%s)",
		d, cloneMinDuration, cloneMaxDuration, cloneSampleHint)
}

// cloneStderrTail lấy vài dòng cuối stderr của ffmpeg (phần chứa lý do lỗi).
func cloneStderrTail(se string) string {
	lines := strings.Split(strings.TrimSpace(se), "\n")
	if len(lines) > 3 {
		lines = lines[len(lines)-3:]
	}
	return shortText(strings.TrimSpace(strings.Join(lines, " | ")), 240)
}

// cloneDecodeBody đọc body JSON tuỳ chọn (body rỗng → giữ nguyên giá trị mặc định).
func cloneDecodeBody(r *http.Request, v any) error {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(v); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("body JSON không hợp lệ: %w", err)
	}
	return nil
}
