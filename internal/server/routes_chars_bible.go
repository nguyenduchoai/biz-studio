package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bizstudio/internal/charbible"
	"bizstudio/internal/gemini"
)

// Routes cho hồ sơ nhân vật đầy đủ: AI dựng hồ sơ + vẽ bản ba góc nhìn.

const sheetTimeout = 5 * time.Minute

func (s *Server) routesCharsBible(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/characters/{id}/bible", s.handleCharBible)
	mux.HandleFunc("POST /api/characters/{id}/sheet", s.handleCharSheet)
}

// handleCharBible — POST /api/characters/{id}/bible: AI dựng hồ sơ đầy đủ từ
// mô tả ngắn đang có, rồi ghép giọng đọc trong máy.
func (s *Server) handleCharBible(w http.ResponseWriter, r *http.Request) {
	c, ok := s.charByID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	if err := charbible.Fill(ctx, s.st, &c); err != nil {
		httpErr(w, http.StatusBadRequest, "%v", err)
		return
	}
	s.st.SaveCharacter(&c)
	note := "Đã dựng hồ sơ cho nhân vật " + shortText(c.Name, 40)
	if c.VoiceSpec != nil && c.VoiceSpec.VoiceID != "" {
		note += " · ghép giọng " + c.VoiceSpec.VoiceID
	}
	s.Log("info", "characters", note)
	writeJSON(w, http.StatusOK, c)
}

// handleCharSheet — POST /api/characters/{id}/sheet: vẽ bản ba góc nhìn, dùng
// làm tờ tham chiếu giữ ngoại hình nhất quán cho cả video.
func (s *Server) handleCharSheet(w http.ResponseWriter, r *http.Request) {
	c, ok := s.charByID(w, r)
	if !ok {
		return
	}
	if strings.TrimSpace(s.st.Settings().GeminiAPIKey) == "" {
		httpErr(w, http.StatusBadRequest, "%s", charNoKeyHint)
		return
	}
	prompt := charbible.SheetPrompt(c, strings.TrimSpace(s.styleLine()))
	if prompt == "" {
		httpErr(w, http.StatusBadRequest, charNoLookHint, c.Name)
		return
	}
	// Bản vẽ dùng negative RIÊNG của nó, không lấy negative của bộ Style Kit:
	// bộ style hay cấm "photorealistic", mà bản vẽ nhân vật thì cần chân thực.
	dst := filepath.Join(s.DataDir, "characters", c.ID+"-sheet.png")

	var body struct {
		SetAsRef bool `json:"setAsRef"`
	}
	if r.ContentLength != 0 {
		_ = readJSON(r, &body)
	}

	j := s.Jobs.Submit("char_sheet", "", "Vẽ bản ba góc nhìn: "+shortText(c.Name, 40),
		func(upd func(float64, string)) (string, error) {
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return "", fmt.Errorf("không tạo được thư mục ảnh nhân vật: %w", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), sheetTimeout)
			defer cancel()
			upd(20, "Đang vẽ bản ba góc nhìn (chân dung + 3 hình chiếu + chi tiết)…")
			if err := gemini.NewFromSettings(s.st).GenerateImage(ctx, prompt, dst); err != nil {
				return "", err
			}
			// Đọc lại bản mới nhất: người dùng có thể đã sửa nhân vật trong lúc vẽ.
			fresh, ok := s.st.Character(c.ID)
			if !ok {
				return "", fmt.Errorf("nhân vật đã bị xoá trong lúc vẽ")
			}
			fresh.SheetImage = s.toolRelPath(dst)
			if body.SetAsRef {
				fresh.RefImage = fresh.SheetImage
			}
			s.st.SaveCharacter(&fresh)
			upd(98, "Xong")
			return fresh.SheetImage, nil
		})
	writeJSON(w, http.StatusOK, j)
}

// styleLine lấy câu mô tả phong cách của bộ Style Kit đang dùng, để bản vẽ
// cùng chất với các cảnh trong video.
func (s *Server) styleLine() string {
	if k, ok := s.st.ActiveStyleKit(); ok {
		return k.StylePrompt
	}
	return ""
}
