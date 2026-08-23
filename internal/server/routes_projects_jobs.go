package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bizstudio/internal/gemini"
	"bizstudio/internal/media"
	"bizstudio/internal/publishpkg"
	"bizstudio/internal/qc"
	"bizstudio/internal/store"
)

// handleProjectQC — POST /api/projects/{id}/qc: chạy QC video output → qc.json.
func (s *Server) handleProjectQC(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, ok := s.st.Project(id)
	if !ok {
		httpErr(w, http.StatusNotFound, "không tìm thấy dự án %q", id)
		return
	}
	if strings.TrimSpace(p.OutputFile) == "" {
		httpErr(w, http.StatusBadRequest, "dự án chưa có video output")
		return
	}
	dir := s.ProjectDir(id)
	job := s.Jobs.Submit("qc", id, "Chạy QC video output", func(upd func(float64, string)) (string, error) {
		cur, ok := s.st.Project(id)
		if !ok {
			return "", fmt.Errorf("dự án %q đã bị xoá", id)
		}
		if strings.TrimSpace(cur.OutputFile) == "" {
			return "", errors.New("dự án chưa có video output")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		upd(10, "Đang phân tích video…")
		rep, err := qc.Run(ctx, filepath.Join(s.DataDir, cur.OutputFile))
		if err != nil {
			return "", err
		}
		b, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			return "", fmt.Errorf("không mã hoá được báo cáo QC: %w", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "qc.json"), b, 0o644); err != nil {
			return "", fmt.Errorf("không ghi được qc.json: %w", err)
		}

		// Ảnh lưới đi kèm báo cáo số. Kiểm bằng số bắt được khung đen, khung
		// đứng, tiếng nhỏ — không bắt được cảnh lặp, ghép nhầm thứ tự, chữ đè
		// lên mặt. Những thứ đó chỉ mắt thấy, mà không ai tua lại cả video sau
		// mỗi lần render.
		//
		// Lỗi dựng ảnh KHÔNG làm hỏng lượt QC: báo cáo số vẫn còn nguyên giá trị.
		upd(80, "Dựng ảnh lưới kiểm nhanh…")
		sheet := filepath.Join(dir, "qc-contact-sheet.png")
		if err := qc.ContactSheet(ctx, filepath.Join(s.DataDir, cur.OutputFile), sheet); err != nil {
			s.Log("warn", "qc", "không dựng được ảnh lưới: "+err.Error())
		} else {
			s.Log("info", "qc", "Ảnh lưới kiểm nhanh: projects/"+id+"/qc-contact-sheet.png")
		}
		return "projects/" + id + "/qc.json", nil
	})
	writeJSON(w, http.StatusOK, job)
}

// handleProjectThumbnail — POST /api/projects/{id}/thumbnail {mode,t,prompt}.
func (s *Server) handleProjectThumbnail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.st.Project(id); !ok {
		httpErr(w, http.StatusNotFound, "không tìm thấy dự án %q", id)
		return
	}
	var body struct {
		Mode   string  `json:"mode"`
		T      float64 `json:"t"`
		Prompt string  `json:"prompt"`
	}
	if r.ContentLength != 0 {
		if err := readJSON(r, &body); err != nil {
			httpErr(w, http.StatusBadRequest, "%s", err)
			return
		}
	}
	if body.Mode == "" {
		body.Mode = "frame"
	}
	if body.Mode != "frame" && body.Mode != "ai" {
		httpErr(w, http.StatusBadRequest, "mode không hợp lệ %q (chỉ hỗ trợ frame|ai)", body.Mode)
		return
	}
	dir := s.ProjectDir(id)
	job := s.Jobs.Submit("thumbnail", id, "Tạo thumbnail", func(upd func(float64, string)) (string, error) {
		cur, ok := s.st.Project(id)
		if !ok {
			return "", fmt.Errorf("dự án %q đã bị xoá", id)
		}
		var rel string
		var err error
		if body.Mode == "ai" {
			rel, err = s.thumbAI(cur, body.Prompt, dir, upd)
		} else {
			rel, err = s.thumbFrame(cur, body.T, dir, upd)
		}
		if err != nil {
			return "", err
		}
		cur.ThumbFile = rel
		s.st.SaveProject(&cur)
		s.Hub.Broadcast("project", cur)
		return rel, nil
	})
	writeJSON(w, http.StatusOK, job)
}

// thumbFrame trích 1 frame từ video output (hoặc asset video đầu tiên) làm thumbnail.
func (s *Server) thumbFrame(p store.Project, t float64, dir string, upd func(float64, string)) (string, error) {
	src := ""
	if strings.TrimSpace(p.OutputFile) != "" {
		src = filepath.Join(s.DataDir, p.OutputFile)
	} else {
		for _, a := range s.st.AssetsByProject(p.ID) {
			if a.Kind == "video" {
				src = filepath.Join(s.DataDir, a.Path)
				break
			}
		}
	}
	if src == "" {
		return "", errors.New("dự án chưa có video (output hoặc asset video) để trích thumbnail")
	}
	if t <= 0 {
		t = 1.0
	}
	upd(20, "Đang trích frame từ video…")
	if err := media.Thumbnail(src, filepath.Join(dir, "thumb.jpg"), t, 720); err != nil {
		return "", err
	}
	return "projects/" + p.ID + "/thumb.jpg", nil
}

// thumbAI tạo thumbnail bằng Gemini từ prompt (hoặc từ mô tả dự án).
func (s *Server) thumbAI(p store.Project, prompt, dir string, upd func(float64, string)) (string, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		if strings.TrimSpace(p.BriefDesc) == "" {
			return "", errors.New("thiếu prompt và mô tả dự án (briefDesc) để tạo thumbnail AI")
		}
		prompt = "Tạo ảnh thumbnail bắt mắt, sắc nét cho video: " + p.BriefDesc
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	upd(20, "Đang tạo ảnh bằng Gemini…")
	if err := gemini.NewFromSettings(s.st).GenerateImage(ctx, prompt, filepath.Join(dir, "thumb.png")); err != nil {
		return "", err
	}
	return "projects/" + p.ID + "/thumb.png", nil
}

// handleProjectPublish — POST /api/projects/{id}/publish: đóng gói xuất bản → zip.
func (s *Server) handleProjectPublish(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.st.Project(id); !ok {
		httpErr(w, http.StatusNotFound, "không tìm thấy dự án %q", id)
		return
	}
	dir := s.ProjectDir(id)
	job := s.Jobs.Submit("publish", id, "Đóng gói xuất bản", func(upd func(float64, string)) (string, error) {
		cur, ok := s.st.Project(id)
		if !ok {
			return "", fmt.Errorf("dự án %q đã bị xoá", id)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		zipAbs, err := publishpkg.Build(ctx, s.st, &cur, dir, upd)
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(s.DataDir, zipAbs)
		if err != nil {
			return "", fmt.Errorf("không tính được đường dẫn tương đối của gói zip: %w", err)
		}
		return filepath.ToSlash(rel), nil
	})
	writeJSON(w, http.StatusOK, job)
}

// handleProjectRenderFinal — POST /api/projects/{id}/render-final:
// re-encode output draft về đúng kích thước dự án → outputs/final.mp4.
func (s *Server) handleProjectRenderFinal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.st.Project(id); !ok {
		httpErr(w, http.StatusNotFound, "không tìm thấy dự án %q", id)
		return
	}
	dir := s.ProjectDir(id)
	job := s.Jobs.Submit("render", id, "Render bản final", func(upd func(float64, string)) (string, error) {
		cur, ok := s.st.Project(id)
		if !ok {
			return "", fmt.Errorf("dự án %q đã bị xoá", id)
		}
		src, err := s.renderSource(cur, dir)
		if err != nil {
			return "", err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()
		upd(10, fmt.Sprintf("Đang re-encode về %dx%d…", cur.Width, cur.Height))
		tmp := filepath.Join(dir, "tmp", "final-render.mp4")
		if err := media.ReEncode(ctx, src, tmp, cur.Width, cur.Height); err != nil {
			return "", err
		}
		if err := os.Rename(tmp, filepath.Join(dir, "outputs", "final.mp4")); err != nil {
			return "", fmt.Errorf("không di chuyển được file final: %w", err)
		}
		rel := "projects/" + id + "/outputs/final.mp4"
		if p2, ok := s.st.Project(id); ok {
			p2.OutputFile, p2.Progress, p2.Status = rel, 6, "done"
			s.st.SaveProject(&p2)
			s.Hub.Broadcast("project", p2)
		}
		return rel, nil
	})
	writeJSON(w, http.StatusOK, job)
}

// renderSource chọn video nguồn cho render final: OutputFile hiện tại,
// nếu không có thì file .mp4 mới nhất trong outputs/.
func (s *Server) renderSource(p store.Project, dir string) (string, error) {
	if strings.TrimSpace(p.OutputFile) != "" {
		src := filepath.Join(s.DataDir, p.OutputFile)
		if _, err := os.Stat(src); err == nil {
			return src, nil
		}
	}
	outDir := filepath.Join(dir, "outputs")
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return "", fmt.Errorf("không đọc được thư mục outputs: %w", err)
	}
	best, bestTime := "", time.Time{}
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".mp4") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(bestTime) {
			best, bestTime = filepath.Join(outDir, e.Name()), info.ModTime()
		}
	}
	if best == "" {
		return "", errors.New("chưa có video output để render final")
	}
	return best, nil
}
