package server

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"bizstudio/internal/avatar"
	"bizstudio/internal/consent"
	"bizstudio/internal/media"
	"bizstudio/internal/store"
	"bizstudio/internal/tts"
)

// Routes cho "Avatar nói" — ảnh + giọng → video người nói (LongCat-Video-Avatar).

const avatarJobTimeout = 45 * time.Minute

func (s *Server) routesAvatar(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/tools/avatar", s.handleAvatarStatus)
	mux.HandleFunc("POST /api/tools/avatar", s.handleAvatarGenerate)
	mux.HandleFunc("POST /api/tools/avatar/voice", s.handleAvatarVoice)
}

// handleAvatarStatus — GET /api/tools/avatar.
func (s *Server) handleAvatarStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	writeJSON(w, http.StatusOK, avatar.Check(ctx, s.st))
}

// handleAvatarVoice — POST /api/tools/avatar/voice: đọc văn bản thành file
// giọng bằng engine sẵn có (VieNeu / giọng nhân bản), để làm đầu vào cho avatar.
//
// Đây là chỗ nối hai module lại thành một dây chuyền: gõ chữ → giọng Việt trên
// máy mình → video người nói. Không phải upload file giọng từ đâu khác.
func (s *Server) handleAvatarVoice(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text   string  `json:"text"`
		Voice  string  `json:"voice"`
		Engine string  `json:"engine"`
		Rate   float64 `json:"rate"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	text := strings.TrimSpace(body.Text)
	if text == "" {
		httpErr(w, http.StatusBadRequest, "chưa nhập lời cần đọc")
		return
	}
	dst := filepath.Join(s.avatarDir(""), uniqueFileName(s.avatarDir(""),
		sanitizeFileName(shortText(text, 30))+".wav"))
	j := s.Jobs.Submit("avatar_voice", "", "Đọc lời cho avatar: "+shortText(text, 40),
		func(upd func(float64, string)) (string, error) {
			ctx, cancel := context.WithTimeout(context.Background(), toolJobTimeout)
			defer cancel()
			upd(20, "Đang đọc bằng engine giọng của máy…")
			if err := tts.Speak(ctx, s.st, text, body.Voice, body.Rate, body.Engine, dst); err != nil {
				return "", err
			}
			if info, err := media.Probe(dst); err == nil && info.Duration > 0 {
				upd(98, fmt.Sprintf("Xong — %.1f giây", info.Duration))
			}
			return s.toolRelPath(dst), nil
		})
	writeJSON(w, http.StatusOK, j)
}

// handleAvatarGenerate — POST /api/tools/avatar: job dựng video người nói.
func (s *Server) handleAvatarGenerate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ImagePath string `json:"imagePath"`
		AudioPath string `json:"audioPath"`
		Prompt    string `json:"prompt"`
		ProjectID string `json:"projectId"`

		// Xác nhận quyền dùng khuôn mặt trong ảnh.
		Rights    bool   `json:"rights"`
		Adult     bool   `json:"adult"`
		Permitted bool   `json:"permitted"`
		Subject   string `json:"subject"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}

	// Chặn TRƯỚC mọi việc khác: làm một tấm ảnh người thật nói theo lời đọc chỉ
	// hợp lệ khi người đó đồng ý, mà chỉ người bấm nút mới biết điều đó.
	grant := consent.Grant{
		Kind: consent.KindFace, Rights: body.Rights, Adult: body.Adult,
		Permitted: body.Permitted, Subject: strings.TrimSpace(body.Subject), At: time.Now(),
	}
	if err := consent.Check(grant, consent.KindFace); err != nil {
		httpErr(w, http.StatusForbidden, "%s", err)
		return
	}
	s.Log("info", "avatar", consent.Line(grant, consent.KindFace))
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	st := avatar.Check(ctx, s.st)
	cancel()
	if !st.Ready {
		httpErr(w, http.StatusBadRequest, "%s", st.Detail)
		return
	}
	img, ok := s.toolSrcPath(w, body.ImagePath)
	if !ok {
		return
	}
	aud, ok := s.toolSrcPath(w, body.AudioPath)
	if !ok {
		return
	}
	// Báo trước độ dài: LongCat dựng theo thời lượng giọng, giọng dài thì lâu.
	note := ""
	if info, err := media.Probe(aud); err == nil && info.Duration > 0 {
		note = fmt.Sprintf(" (%.1f giây giọng)", info.Duration)
	}

	dir := s.avatarDir(body.ProjectID)
	dst := filepath.Join(dir, uniqueFileName(dir,
		sanitizeFileName(strings.TrimSuffix(filepath.Base(img), filepath.Ext(img)))+"-noi.mp4"))
	opts := avatar.Opts{ImagePath: img, AudioPath: aud, Prompt: body.Prompt}

	j := s.Jobs.Submit("avatar", body.ProjectID,
		"Avatar nói: "+filepath.Base(img)+note,
		func(upd func(float64, string)) (string, error) {
			jctx, jcancel := context.WithTimeout(context.Background(), avatarJobTimeout)
			defer jcancel()
			if err := avatar.Generate(jctx, s.st, opts, dst, upd); err != nil {
				return "", err
			}
			s.Log("info", "avatar", "Đã dựng video người nói: "+filepath.Base(dst))
			if body.ProjectID != "" {
				s.attachAvatarAsset(body.ProjectID, dst)
			}
			return s.toolRelPath(dst), nil
		})
	writeJSON(w, http.StatusOK, j)
}

// attachAvatarAsset đưa video vừa dựng vào thư viện media của dự án.
func (s *Server) attachAvatarAsset(projectID, path string) {
	a := store.Asset{
		ProjectID: projectID, Kind: "video",
		Name: filepath.Base(path), Path: s.toolRelPath(path),
	}
	if info, err := media.Probe(path); err == nil {
		a.Duration = info.Duration
	}
	s.st.SaveAsset(&a)
}

// avatarDir — nơi lưu: trong dự án nếu có, không thì data/avatar.
func (s *Server) avatarDir(projectID string) string {
	if strings.TrimSpace(projectID) != "" {
		return filepath.Join(s.DataDir, "projects", projectID, "avatar")
	}
	return filepath.Join(s.DataDir, "avatar")
}
