package server

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"bizstudio/internal/media"
	"bizstudio/internal/store"
)

func (s *Server) routesAssets(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/projects/{id}/assets", s.handleAssetUpload)
	mux.HandleFunc("PUT /api/assets/{id}", s.handleAssetUpdate)
	mux.HandleFunc("DELETE /api/assets/{id}", s.handleAssetDelete)
}

func (s *Server) handleAssetUpload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.st.Project(id); !ok {
		httpErr(w, http.StatusNotFound, "không tìm thấy dự án %q", id)
		return
	}
	assets, err := saveUploadedAssets(s, id, r)
	if err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	writeJSON(w, http.StatusOK, assets)
}

// saveUploadedAssets đọc multipart field "files", lưu vào projects/<id>/assets/
// và tạo Asset records. Dùng chung cho /api/projects/{id}/assets và /m/{id}/upload.
func saveUploadedAssets(s *Server, projectID string, r *http.Request) ([]store.Asset, error) {
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		return nil, fmt.Errorf("không đọc được dữ liệu upload: %w", err)
	}
	if r.MultipartForm == nil || len(r.MultipartForm.File["files"]) == 0 {
		return nil, fmt.Errorf("không có file nào trong trường 'files'")
	}
	defer r.MultipartForm.RemoveAll()
	files := r.MultipartForm.File["files"]
	if len(files) > mobileUploadMaxFile {
		return nil, fmt.Errorf("tối đa %d file mỗi lượt", mobileUploadMaxFile)
	}
	assetsDir := filepath.Join(s.ProjectDir(projectID), "assets")
	baseOrder := len(s.st.AssetsByProject(projectID))

	out := make([]store.Asset, 0, len(files))
	for i, fh := range files {
		name := uniqueFileName(assetsDir, sanitizeFileName(fh.Filename))
		dst := filepath.Join(assetsDir, name)
		size, err := saveMultipartFile(fh, dst)
		if err != nil {
			return out, err
		}
		a := store.Asset{
			ProjectID: projectID,
			Kind:      assetKindByExt(name),
			Name:      name,
			Path:      "projects/" + projectID + "/assets/" + name,
			Size:      size,
			Order:     baseOrder + i,
		}
		if a.Kind != "other" {
			if info, perr := media.Probe(dst); perr == nil {
				a.Duration = info.Duration
			} else if a.Kind != "image" { // ảnh: bỏ qua lỗi probe
				s.Log("warn", "assets", "Không probe được "+name+": "+perr.Error())
			}
		}
		s.st.SaveAsset(&a)
		out = append(out, a)
	}
	return out, nil
}

// saveMultipartFile ghi 1 file upload xuống dst, trả số byte đã ghi.
func saveMultipartFile(fh *multipart.FileHeader, dst string) (int64, error) {
	src, err := fh.Open()
	if err != nil {
		return 0, fmt.Errorf("không mở được file upload %q: %w", fh.Filename, err)
	}
	defer src.Close()
	out, err := os.Create(dst)
	if err != nil {
		return 0, fmt.Errorf("không tạo được file %s: %w", dst, err)
	}
	n, err := io.Copy(out, src)
	if err != nil {
		out.Close()
		_ = os.Remove(dst)
		return 0, fmt.Errorf("không ghi được file %s: %w", dst, err)
	}
	if err := out.Close(); err != nil {
		return 0, fmt.Errorf("không đóng được file %s: %w", dst, err)
	}
	return n, nil
}

// sanitizeFileName bỏ path, thay ký tự lạ; giữ chữ (kể cả unicode), số, ".", "-", "_".
func sanitizeFileName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	var b strings.Builder
	for _, r := range name {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '-' || r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), ".-_")
	if out == "" {
		out = "file"
	}
	return out
}

// uniqueFileName chống trùng tên trong dir bằng hậu tố -1, -2, …
func uniqueFileName(dir, name string) string {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	candidate := name
	for i := 1; ; i++ {
		if _, err := os.Stat(filepath.Join(dir, candidate)); os.IsNotExist(err) {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d%s", stem, i, ext)
	}
}

// assetKindByExt phân loại asset theo phần mở rộng file.
func assetKindByExt(name string) string {
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(name), ".")) {
	case "mp4", "mov", "mkv", "webm":
		return "video"
	case "jpg", "jpeg", "png", "webp", "gif":
		return "image"
	case "mp3", "wav", "m4a", "aac", "flac":
		return "audio"
	default:
		return "other"
	}
}

func (s *Server) handleAssetUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a, ok := s.st.Asset(id)
	if !ok {
		httpErr(w, http.StatusNotFound, "không tìm thấy asset %q", id)
		return
	}
	var body struct {
		Desc  *string `json:"desc"`
		Order *int    `json:"order"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	if body.Desc != nil {
		a.Desc = *body.Desc
	}
	if body.Order != nil {
		a.Order = *body.Order
	}
	s.st.SaveAsset(&a)
	writeJSON(w, http.StatusOK, a)
}

func (s *Server) handleAssetDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a, ok := s.st.Asset(id)
	if !ok {
		httpErr(w, http.StatusNotFound, "không tìm thấy asset %q", id)
		return
	}
	if a.Path != "" {
		if err := os.Remove(filepath.Join(s.DataDir, a.Path)); err != nil && !os.IsNotExist(err) {
			httpErr(w, http.StatusInternalServerError, "không xoá được file asset: %s", err)
			return
		}
	}
	s.st.DeleteAsset(id)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
