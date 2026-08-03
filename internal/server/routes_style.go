package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bizstudio/internal/gemini"
	"bizstudio/internal/store"
	"bizstudio/internal/stylekit"
)

const (
	// Chủ thể mặc định của ảnh xem thử — trung tính để thấy rõ chất của bộ style.
	stylePreviewSubject = "a person working at a desk near a window"
	stylePreviewTimeout = 5 * time.Minute
	stylePaletteMax     = 12

	styleNoKeyHint = "chưa cấu hình Gemini API key — mở trang \"Cấu hình & API\", dán khoá Gemini rồi bấm Lưu để xem thử ảnh của bộ style"
)

// routesStyle — Style Kit: bộ style hình ảnh dùng chung để mọi cảnh trong một
// video trông như cùng một bộ phim.
func (s *Server) routesStyle(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/styles", s.handleStyleList)
	mux.HandleFunc("POST /api/styles", s.handleStyleCreate)
	mux.HandleFunc("PUT /api/styles/{id}", s.handleStyleUpdate)
	mux.HandleFunc("DELETE /api/styles/{id}", s.handleStyleDelete)
	mux.HandleFunc("POST /api/styles/{id}/default", s.handleStyleDefault)
	mux.HandleFunc("POST /api/styles/{id}/preview", s.handleStylePreview)
}

// styleBody — payload tạo / sửa bộ style.
type styleBody struct {
	Name        string   `json:"name"`
	StylePrompt string   `json:"stylePrompt"`
	Negative    string   `json:"negative"`
	Palette     []string `json:"palette"`
	Theme       string   `json:"theme"`
	IsDefault   bool     `json:"isDefault"`
}

// handleStyleList — GET /api/styles (bộ đang dùng đứng đầu, sau đó mới nhất trước).
func (s *Server) handleStyleList(w http.ResponseWriter, r *http.Request) {
	list := s.st.StyleKits()
	if list == nil {
		list = []store.StyleKit{}
	}
	writeJSON(w, http.StatusOK, list)
}

// handleStyleCreate — POST /api/styles.
func (s *Server) handleStyleCreate(w http.ResponseWriter, r *http.Request) {
	body, ok := readStyleBody(w, r)
	if !ok {
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		httpErr(w, http.StatusBadRequest, "thiếu tên bộ style")
		return
	}
	k := store.StyleKit{
		Name:        name,
		StylePrompt: strings.TrimSpace(body.StylePrompt),
		Negative:    strings.TrimSpace(body.Negative),
		Palette:     normalizePalette(body.Palette),
		Theme:       normalizeStyleTheme(body.Theme),
		// Bộ đầu tiên luôn là bộ đang dùng — hệ thống không được thiếu bộ mặc định.
		IsDefault: body.IsDefault || len(s.st.StyleKits()) == 0,
	}
	s.st.SaveStyleKit(&k)
	s.Log("info", "style", fmt.Sprintf("Đã tạo bộ style %q", k.Name))
	writeJSON(w, http.StatusOK, k)
}

// handleStyleUpdate — PUT /api/styles/{id}.
func (s *Server) handleStyleUpdate(w http.ResponseWriter, r *http.Request) {
	cur, ok := s.styleKit(w, r)
	if !ok {
		return
	}
	body, ok := readStyleBody(w, r)
	if !ok {
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		httpErr(w, http.StatusBadRequest, "thiếu tên bộ style")
		return
	}
	cur.Name = name
	cur.StylePrompt = strings.TrimSpace(body.StylePrompt)
	cur.Negative = strings.TrimSpace(body.Negative)
	cur.Palette = normalizePalette(body.Palette)
	cur.Theme = normalizeStyleTheme(body.Theme)
	// Bộ đang mặc định không tự bỏ mặc định khi sửa — muốn đổi thì chọn bộ khác
	// làm mặc định, tránh trạng thái không bộ nào được dùng.
	cur.IsDefault = cur.IsDefault || body.IsDefault

	s.st.SaveStyleKit(&cur)
	s.Log("info", "style", fmt.Sprintf("Đã cập nhật bộ style %q", cur.Name))
	writeJSON(w, http.StatusOK, cur)
}

// handleStyleDelete — DELETE /api/styles/{id}; xoá bộ đang mặc định thì tự đặt
// bộ còn lại mới nhất làm mặc định.
func (s *Server) handleStyleDelete(w http.ResponseWriter, r *http.Request) {
	k, ok := s.styleKit(w, r)
	if !ok {
		return
	}
	s.st.DeleteStyleKit(k.ID)
	if err := os.Remove(s.stylePreviewPath(k.ID)); err != nil && !os.IsNotExist(err) {
		s.Log("warn", "style", fmt.Sprintf("Không xoá được ảnh xem thử của bộ style %q: %v", k.Name, err))
	}
	s.Log("info", "style", fmt.Sprintf("Đã xoá bộ style %q", k.Name))
	if k.IsDefault {
		s.promoteNewestStyle()
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleStyleDefault — POST /api/styles/{id}/default.
func (s *Server) handleStyleDefault(w http.ResponseWriter, r *http.Request) {
	k, ok := s.styleKit(w, r)
	if !ok {
		return
	}
	k.IsDefault = true
	s.st.SaveStyleKit(&k)
	s.Log("info", "style", fmt.Sprintf("Đang dùng bộ style %q cho mọi ảnh sinh ra", k.Name))
	writeJSON(w, http.StatusOK, k)
}

// handleStylePreview — POST /api/styles/{id}/preview: sinh ảnh mẫu của bộ style.
func (s *Server) handleStylePreview(w http.ResponseWriter, r *http.Request) {
	k, ok := s.styleKit(w, r)
	if !ok {
		return
	}
	if strings.TrimSpace(s.st.Settings().GeminiAPIKey) == "" {
		httpErr(w, http.StatusBadRequest, "%s", styleNoKeyHint)
		return
	}
	var body struct {
		Subject string `json:"subject"`
	}
	if r.ContentLength != 0 {
		if err := readJSON(r, &body); err != nil {
			httpErr(w, http.StatusBadRequest, "%s", err)
			return
		}
	}
	subject := strings.TrimSpace(body.Subject)
	if subject == "" {
		subject = stylePreviewSubject
	}
	prompt := stylekit.ApplyKit(k, subject)
	dst := s.stylePreviewPath(k.ID)

	j := s.Jobs.Submit("style_preview", "", "Xem thử bộ style: "+shortText(k.Name, 40),
		func(upd func(float64, string)) (string, error) {
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return "", fmt.Errorf("không tạo được thư mục ảnh xem thử: %w", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), stylePreviewTimeout)
			defer cancel()
			upd(20, "Đang sinh ảnh mẫu bằng Gemini…")
			if err := gemini.NewFromSettings(s.st).GenerateImage(ctx, prompt, dst); err != nil {
				return "", err
			}
			return s.toolRelPath(dst), nil
		})
	writeJSON(w, http.StatusOK, j)
}

// ---------- helpers ----------

// styleKit lấy bộ style theo path param, tự trả 404 tiếng Việt nếu không có.
func (s *Server) styleKit(w http.ResponseWriter, r *http.Request) (store.StyleKit, bool) {
	id := r.PathValue("id")
	k, ok := s.st.StyleKit(id)
	if !ok {
		httpErr(w, http.StatusNotFound, "không tìm thấy bộ style %q", id)
		return store.StyleKit{}, false
	}
	return k, true
}

// stylePreviewPath — data/styles/<id>-preview.png.
func (s *Server) stylePreviewPath(id string) string {
	return filepath.Join(s.DataDir, "styles", id+"-preview.png")
}

// readStyleBody đọc payload tạo/sửa; đã ghi lỗi HTTP nếu body hỏng.
func readStyleBody(w http.ResponseWriter, r *http.Request) (styleBody, bool) {
	var body styleBody
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return styleBody{}, false
	}
	return body, true
}

// promoteNewestStyle đặt bộ còn lại mới nhất làm mặc định — luôn phải có một bộ
// đang dùng, nếu không mọi ảnh sinh ra sẽ mất tính đồng nhất.
func (s *Server) promoteNewestStyle() {
	rest := s.st.StyleKits()
	if len(rest) == 0 {
		return
	}
	next := rest[0]
	for _, k := range rest {
		if k.UpdatedAt.After(next.UpdatedAt) {
			next = k
		}
	}
	next.IsDefault = true
	s.st.SaveStyleKit(&next)
	s.Log("info", "style", fmt.Sprintf("Chuyển sang dùng bộ style %q", next.Name))
}

// normalizeStyleTheme ép theme về giá trị hợp lệ (vivid | dark | light).
func normalizeStyleTheme(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "dark":
		return "dark"
	case "light":
		return "light"
	default:
		return "vivid"
	}
}

// normalizePalette bỏ mã màu rỗng và giới hạn số lượng màu.
func normalizePalette(list []string) []string {
	out := make([]string, 0, len(list))
	for _, c := range list {
		if c = strings.TrimSpace(c); c == "" {
			continue
		}
		out = append(out, c)
		if len(out) >= stylePaletteMax {
			break
		}
	}
	return out
}
