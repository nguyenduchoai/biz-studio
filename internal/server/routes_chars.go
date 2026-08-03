package server

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"bizstudio/internal/gemini"
	"bizstudio/internal/store"
	"bizstudio/internal/stylekit"
	"bizstudio/internal/text2video"
	"bizstudio/internal/util"
)

const (
	// charDirName — thư mục ảnh nhân vật trong data/.
	charDirName = "characters"
	// charPreviewScene — bối cảnh mặc định của ảnh xem thử: trung tính để nhìn
	// rõ nhân vật chứ không bị bối cảnh lấn át.
	charPreviewScene = "standing in a simple room, medium shot"

	charPreviewTimeout = 5 * time.Minute
	charConvertTimeout = 2 * time.Minute
	charUploadMaxMem   = 256 << 20

	charNoKeyHint  = "chưa cấu hình Gemini API key — mở trang \"Cấu hình & API\", dán khoá Gemini rồi bấm Lưu để xem thử nhân vật"
	charNoLookHint = "nhân vật %q chưa có mô tả ngoại hình — nhập phần \"Ngoại hình\" (tiếng Anh) rồi mới xem thử được"
	charNoFileHint = "chưa chọn ảnh (field \"files\") — chọn 1 file ảnh chân dung làm ảnh tham chiếu"
)

// routesChars — Nhân vật nhất quán: hồ sơ ngoại hình cố định của nhân vật, được
// chèn vào prompt sinh ảnh của mọi cảnh có nhân vật đó (xem text2video.CharacterClause).
func (s *Server) routesChars(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/characters", s.handleCharList)
	mux.HandleFunc("POST /api/characters", s.handleCharCreate)
	mux.HandleFunc("PUT /api/characters/{id}", s.handleCharUpdate)
	mux.HandleFunc("DELETE /api/characters/{id}", s.handleCharDelete)
	mux.HandleFunc("POST /api/characters/{id}/ref", s.handleCharRef)
	mux.HandleFunc("POST /api/characters/{id}/preview", s.handleCharPreview)
}

// charBody — payload tạo / sửa nhân vật.
type charBody struct {
	Name string `json:"name"`
	Look string `json:"look"`
	Role string `json:"role"`
}

// handleCharList — GET /api/characters.
func (s *Server) handleCharList(w http.ResponseWriter, r *http.Request) {
	list := s.st.Characters()
	if list == nil {
		list = []store.Character{}
	}
	writeJSON(w, http.StatusOK, list)
}

// handleCharCreate — POST /api/characters.
func (s *Server) handleCharCreate(w http.ResponseWriter, r *http.Request) {
	body, ok := readCharBody(w, r)
	if !ok {
		return
	}
	name := charLine(body.Name)
	if name == "" {
		httpErr(w, http.StatusBadRequest, "thiếu tên nhân vật")
		return
	}
	c := store.Character{Name: name, Look: charLine(body.Look), Role: charLine(body.Role)}
	s.st.SaveCharacter(&c)
	s.Log("info", "characters", fmt.Sprintf("Đã tạo nhân vật %q", c.Name))
	writeJSON(w, http.StatusOK, c)
}

// handleCharUpdate — PUT /api/characters/{id} (ảnh tham chiếu đổi qua /ref).
func (s *Server) handleCharUpdate(w http.ResponseWriter, r *http.Request) {
	c, ok := s.charByID(w, r)
	if !ok {
		return
	}
	body, ok := readCharBody(w, r)
	if !ok {
		return
	}
	name := charLine(body.Name)
	if name == "" {
		httpErr(w, http.StatusBadRequest, "thiếu tên nhân vật")
		return
	}
	c.Name, c.Look, c.Role = name, charLine(body.Look), charLine(body.Role)
	s.st.SaveCharacter(&c)
	s.Log("info", "characters", fmt.Sprintf("Đã cập nhật nhân vật %q", c.Name))
	writeJSON(w, http.StatusOK, c)
}

// handleCharDelete — DELETE /api/characters/{id}: xoá hồ sơ, gỡ nhân vật khỏi
// mọi cảnh đang gán và xoá ảnh kèm theo.
func (s *Server) handleCharDelete(w http.ResponseWriter, r *http.Request) {
	c, ok := s.charByID(w, r)
	if !ok {
		return
	}
	s.st.DeleteCharacter(c.ID)
	n := s.detachCharacter(c.ID)
	s.removeCharFiles(c)

	msg := fmt.Sprintf("Đã xoá nhân vật %q", c.Name)
	if n > 0 {
		msg += fmt.Sprintf(" và gỡ khỏi %d phiên Text → Video", n)
	}
	s.Log("info", "characters", msg)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "sessionsUpdated": n})
}

// handleCharRef — POST /api/characters/{id}/ref, multipart "files" (1 ảnh).
//
// LƯU Ý: ảnh tham chiếu hiện chỉ để NGƯỜI DÙNG nhìn và đối chiếu — Gemini API
// sinh ảnh từ mô tả chữ (Look), KHÔNG nhận ảnh mẫu, nên ảnh này không ảnh hưởng
// kết quả sinh ảnh. Tính nhất quán vẫn do phần "Ngoại hình" quyết định.
func (s *Server) handleCharRef(w http.ResponseWriter, r *http.Request) {
	c, ok := s.charByID(w, r)
	if !ok {
		return
	}
	if err := r.ParseMultipartForm(charUploadMaxMem); err != nil {
		httpErr(w, http.StatusBadRequest, "không đọc được form upload: %v", err)
		return
	}
	if r.MultipartForm == nil || len(r.MultipartForm.File["files"]) == 0 {
		httpErr(w, http.StatusBadRequest, "%s", charNoFileHint)
		return
	}
	tmp, err := s.saveCharUpload(r.MultipartForm.File["files"][0])
	if err != nil {
		httpErr(w, http.StatusInternalServerError, "%s", err)
		return
	}
	defer os.Remove(tmp)

	ctx, cancel := context.WithTimeout(r.Context(), charConvertTimeout)
	defer cancel()
	if err := s.storeCharImage(ctx, tmp, s.charRefPath(c.ID)); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	c.RefImage = charRelPath(c.ID)
	s.st.SaveCharacter(&c)
	s.Log("info", "characters", fmt.Sprintf("Đã cập nhật ảnh tham chiếu của nhân vật %q", c.Name))
	writeJSON(w, http.StatusOK, c)
}

// handleCharPreview — POST /api/characters/{id}/preview: sinh ảnh thử để xem
// mô tả ngoại hình cho ra nhân vật thế nào (cùng thứ tự ghép prompt như khi
// dựng storyboard: cảnh → nhân vật → Style Kit).
func (s *Server) handleCharPreview(w http.ResponseWriter, r *http.Request) {
	c, ok := s.charByID(w, r)
	if !ok {
		return
	}
	if strings.TrimSpace(s.st.Settings().GeminiAPIKey) == "" {
		httpErr(w, http.StatusBadRequest, "%s", charNoKeyHint)
		return
	}
	if strings.TrimSpace(c.Look) == "" {
		httpErr(w, http.StatusBadRequest, charNoLookHint, c.Name)
		return
	}
	var body struct {
		Scene string `json:"scene"`
	}
	if r.ContentLength != 0 {
		if err := readJSON(r, &body); err != nil {
			httpErr(w, http.StatusBadRequest, "%s", err)
			return
		}
	}
	scene := charLine(body.Scene)
	if scene == "" {
		scene = charPreviewScene
	}
	prompt := stylekit.Apply(s.st, scene+". "+text2video.CharacterClause(s.st, []string{c.ID}))
	dst := s.charPreviewPath(c.ID)

	j := s.Jobs.Submit("char_preview", "", "Xem thử nhân vật: "+shortText(c.Name, 40),
		func(upd func(float64, string)) (string, error) {
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return "", fmt.Errorf("không tạo được thư mục ảnh nhân vật: %w", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), charPreviewTimeout)
			defer cancel()
			upd(20, "Đang sinh ảnh thử bằng Gemini…")
			if err := gemini.NewFromSettings(s.st).GenerateImage(ctx, prompt, dst); err != nil {
				return "", err
			}
			return s.toolRelPath(dst), nil
		})
	writeJSON(w, http.StatusOK, j)
}

// ---------- helpers ----------

// charByID lấy nhân vật theo path param, tự trả 404 tiếng Việt nếu không có.
func (s *Server) charByID(w http.ResponseWriter, r *http.Request) (store.Character, bool) {
	id := r.PathValue("id")
	c, ok := s.st.Character(id)
	if !ok {
		httpErr(w, http.StatusNotFound, "không tìm thấy nhân vật %q", id)
		return store.Character{}, false
	}
	return c, true
}

// readCharBody đọc payload tạo/sửa; đã ghi lỗi HTTP nếu body hỏng.
func readCharBody(w http.ResponseWriter, r *http.Request) (charBody, bool) {
	var body charBody
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return charBody{}, false
	}
	return body, true
}

// charLine gộp mọi khoảng trắng (kể cả xuống dòng) thành một dấu cách — mô tả
// nhân vật được ghép thẳng vào câu prompt nên không được có xuống dòng.
func charLine(v string) string { return strings.Join(strings.Fields(v), " ") }

// charRelPath — ảnh tham chiếu, đường dẫn tương đối DataDir (FE mở qua /data/…).
func charRelPath(id string) string { return path.Join(charDirName, id+".png") }

// charRefPath — data/characters/<id>.png.
func (s *Server) charRefPath(id string) string {
	return filepath.Join(s.DataDir, charDirName, id+".png")
}

// charPreviewPath — data/characters/<id>-preview.png.
func (s *Server) charPreviewPath(id string) string {
	return filepath.Join(s.DataDir, charDirName, id+"-preview.png")
}

// saveCharUpload lưu ảnh upload vào data/tmp để xử lý; trả đường dẫn file tạm.
func (s *Server) saveCharUpload(fh *multipart.FileHeader) (string, error) {
	dir := filepath.Join(s.DataDir, "tmp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("không tạo được thư mục tạm: %w", err)
	}
	tmp := filepath.Join(dir, s.st.NewID("charsrc")+filepath.Ext(sanitizeFileName(fh.Filename)))
	if _, err := saveMultipartFile(fh, tmp); err != nil {
		return "", err
	}
	return tmp, nil
}

// storeCharImage chuyển ảnh tải lên sang PNG tại dst bằng ffmpeg (cũng là bước
// kiểm tra file có thật là ảnh). Ghi ra file tạm rồi mới thay file đích để ảnh
// cũ không mất khi lỗi.
func (s *Server) storeCharImage(ctx context.Context, src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("không tạo được thư mục ảnh nhân vật: %w", err)
	}
	tmp := dst + ".part.png"
	defer os.Remove(tmp)

	if _, se, err := util.RunErr(ctx, "ffmpeg", "-y", "-i", src, "-frames:v", "1", tmp); err != nil {
		return fmt.Errorf("không đọc được ảnh tải lên (chỉ nhận file ảnh: png, jpg, webp, heic…): %v — %s",
			err, cloneStderrTail(se))
	}
	if fi, err := os.Stat(tmp); err != nil || fi.Size() == 0 {
		return fmt.Errorf("ảnh tải lên rỗng hoặc không đọc được")
	}
	if err := os.Rename(tmp, dst); err != nil {
		return fmt.Errorf("không ghi được ảnh nhân vật: %w", err)
	}
	return nil
}

// detachCharacter gỡ id nhân vật khỏi MỌI cảnh của MỌI phiên Text → Video — bỏ
// bước này thì cảnh vẫn trỏ tới nhân vật đã xoá. Trả số phiên đã sửa.
func (s *Server) detachCharacter(id string) int {
	n := 0
	for _, sess := range s.st.T2VSessions() {
		// Bản sao riêng: slice segments lấy từ store dùng chung bộ nhớ với dữ
		// liệu đang lưu, sửa thẳng lên đó là sửa trộm sau lưng store.
		segs := make([]store.T2VSegment, len(sess.Segments))
		copy(segs, sess.Segments)

		changed := false
		for i := range segs {
			ids, dropped := dropCharID(segs[i].CharacterIDs, id)
			if dropped {
				segs[i].CharacterIDs = ids
				changed = true
			}
		}
		if !changed {
			continue
		}
		sess.Segments = segs
		s.st.SaveT2VSession(&sess)
		n++
	}
	return n
}

// dropCharID trả danh sách MỚI đã bỏ id (không đụng vào slice cũ).
func dropCharID(ids []string, id string) ([]string, bool) {
	out := make([]string, 0, len(ids))
	for _, v := range ids {
		if strings.TrimSpace(v) == id {
			continue
		}
		out = append(out, v)
	}
	if len(out) == len(ids) {
		return ids, false
	}
	return out, true
}

// removeCharFiles xoá ảnh tham chiếu + ảnh xem thử của nhân vật (chỉ file nằm
// trong data dir; thiếu file thì bỏ qua).
func (s *Server) removeCharFiles(c store.Character) {
	paths := []string{s.charRefPath(c.ID), s.charPreviewPath(c.ID)}
	if p := strings.TrimSpace(c.RefImage); p != "" && !filepath.IsAbs(p) {
		paths = append(paths, filepath.Join(s.DataDir, filepath.FromSlash(p)))
	}
	for _, p := range paths {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			s.Log("warn", "characters",
				fmt.Sprintf("Không xoá được ảnh %s của nhân vật %q: %v", filepath.Base(p), c.Name, err))
		}
	}
}
