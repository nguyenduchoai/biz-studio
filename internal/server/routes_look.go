package server

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"bizstudio/internal/htmlvideo"
	"bizstudio/internal/media"
)

// Routes cho phần "diện mạo": chỉnh màu, tiếng động, font tiếng Việt.

const fontDownloadTimeout = 5 * time.Minute

func (s *Server) routesLook(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/tools/grades", s.handleGradeList)
	mux.HandleFunc("POST /api/tools/grade", s.handleGradeApply)
	mux.HandleFunc("POST /api/tools/grade/preview", s.handleGradePreview)

	mux.HandleFunc("GET /api/tools/sfx", s.handleSfxList)
	mux.HandleFunc("POST /api/tools/sfx/mix", s.handleSfxMix)

	mux.HandleFunc("GET /api/tools/font", s.handleFontStatus)
	mux.HandleFunc("POST /api/tools/font", s.handleFontDownload)
}

// ---------- chỉnh màu ----------

// handleGradeList — GET /api/tools/grades: danh sách kiểu màu dựng sẵn.
func (s *Server) handleGradeList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, media.GradePresets())
}

// handleGradeApply — POST /api/tools/grade: job chỉnh màu cả video.
// output = <src>.<preset>.mp4 cùng thư mục.
func (s *Server) handleGradeApply(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path     string  `json:"path"`
		Preset   string  `json:"preset"`
		Strength float64 `json:"strength"` // 0..1, 0 = dùng hết
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	src, ok := s.toolSrcPath(w, body.Path)
	if !ok {
		return
	}
	p, found := media.FindGrade(strings.TrimSpace(body.Preset))
	if !found {
		httpErr(w, http.StatusBadRequest, "không có kiểu màu %q", body.Preset)
		return
	}
	dst := strings.TrimSuffix(src, filepath.Ext(src)) + "." + p.ID + ".mp4"
	j := s.Jobs.Submit("grade", "", "Chỉnh màu "+p.Name+": "+filepath.Base(src),
		func(upd func(float64, string)) (string, error) {
			ctx, cancel := context.WithTimeout(context.Background(), toolJobTimeout)
			defer cancel()
			upd(10, "Đang áp kiểu màu "+p.Name+"…")
			if err := media.ApplyGrade(ctx, src, p.ID, dst, body.Strength); err != nil {
				return "", err
			}
			upd(98, p.Name+" — xong")
			return s.toolRelPath(dst), nil
		})
	writeJSON(w, http.StatusOK, j)
}

// handleGradePreview — POST /api/tools/grade/preview: xuất MỘT khung hình đã
// chỉnh màu để xem thử, chạy đồng bộ vì rất nhanh.
func (s *Server) handleGradePreview(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path     string  `json:"path"`
		Preset   string  `json:"preset"`
		AtSec    float64 `json:"atSec"`
		Strength float64 `json:"strength"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	src, ok := s.toolSrcPath(w, body.Path)
	if !ok {
		return
	}
	p, found := media.FindGrade(strings.TrimSpace(body.Preset))
	if !found {
		httpErr(w, http.StatusBadRequest, "không có kiểu màu %q", body.Preset)
		return
	}
	dst := filepath.Join(s.DataDir, "tmp", "grade-preview",
		fmt.Sprintf("%s-%s.jpg", sanitizeFileName(strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))), p.ID))
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	if err := media.GradePreview(ctx, src, p.ID, dst, body.AtSec, body.Strength); err != nil {
		httpErr(w, http.StatusBadRequest, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": s.toolRelPath(dst), "preset": p.ID, "name": p.Name})
}

// ---------- tiếng động ----------

// handleSfxList — GET /api/tools/sfx: danh sách hiệu ứng, tổng hợp file wav
// lần đầu gọi để nghe thử được ngay.
func (s *Server) handleSfxList(w http.ResponseWriter, r *http.Request) {
	dir := s.sfxDir()
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
	if err := media.EnsureAllSfx(ctx, dir); err != nil {
		httpErr(w, http.StatusInternalServerError, "%v", err)
		return
	}
	type item struct {
		media.SfxPreset
		Path string `json:"path"`
	}
	list := media.SfxPresets()
	out := make([]item, 0, len(list))
	for _, p := range list {
		out = append(out, item{SfxPreset: p, Path: s.toolRelPath(filepath.Join(dir, p.ID+".wav"))})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleSfxMix — POST /api/tools/sfx/mix: chèn hiệu ứng vào các mốc thời gian.
// Mỗi mốc chỉ cần {sfx, atSec, gain}; sfx là id trong thư viện HOẶC đường dẫn
// file riêng của người dùng.
func (s *Server) handleSfxMix(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
		Cues []struct {
			Sfx   string  `json:"sfx"`
			AtSec float64 `json:"atSec"`
			Gain  float64 `json:"gain"`
		} `json:"cues"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	src, ok := s.toolSrcPath(w, body.Path)
	if !ok {
		return
	}
	if len(body.Cues) == 0 {
		httpErr(w, http.StatusBadRequest, "chưa chọn hiệu ứng nào để chèn")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
	cues := make([]media.SfxCue, 0, len(body.Cues))
	for _, c := range body.Cues {
		id := strings.TrimSpace(c.Sfx)
		var p string
		if _, found := media.FindSfx(id); found {
			var err error
			if p, err = media.EnsureSfx(ctx, s.sfxDir(), id); err != nil {
				httpErr(w, http.StatusInternalServerError, "%v", err)
				return
			}
		} else {
			abs, okp := s.toolSrcPath(w, id)
			if !okp {
				return
			}
			p = abs
		}
		cues = append(cues, media.SfxCue{Path: p, AtSec: c.AtSec, Gain: c.Gain})
	}
	dst := strings.TrimSuffix(src, filepath.Ext(src)) + ".sfx.mp4"
	j := s.Jobs.Submit("sfx", "", "Chèn tiếng động: "+filepath.Base(src),
		func(upd func(float64, string)) (string, error) {
			jctx, jcancel := context.WithTimeout(context.Background(), toolJobTimeout)
			defer jcancel()
			upd(20, fmt.Sprintf("Đang chèn %d hiệu ứng…", len(cues)))
			if err := media.MixSfx(jctx, src, dst, cues); err != nil {
				return "", err
			}
			upd(98, fmt.Sprintf("Đã chèn %d hiệu ứng", len(cues)))
			return s.toolRelPath(dst), nil
		})
	writeJSON(w, http.StatusOK, j)
}

// sfxDir — data/sfx.
func (s *Server) sfxDir() string { return filepath.Join(s.DataDir, "sfx") }

// ---------- font tiếng Việt ----------

// handleFontStatus — GET /api/tools/font.
func (s *Server) handleFontStatus(w http.ResponseWriter, r *http.Request) {
	ready := htmlvideo.VietFontReady(s.DataDir)
	note := "Chưa tải — video đang dùng font của hệ điều hành. Máy thiếu chữ có dấu chồng tầng thì chữ hiện ra hai kiểu trong cùng một dòng."
	if ready {
		note = "Đã sẵn sàng — mọi máy render ra cùng một kiểu chữ, không lo thiếu dấu."
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ready":  ready,
		"family": "Be Vietnam Pro",
		"note":   note,
		"dir":    s.toolRelPath(htmlvideo.FontsDir(s.DataDir)),
	})
}

// handleFontDownload — POST /api/tools/font: tải font về data/fonts.
func (s *Server) handleFontDownload(w http.ResponseWriter, r *http.Request) {
	j := s.Jobs.Submit("font", "", "Tải font tiếng Việt Be Vietnam Pro",
		func(upd func(float64, string)) (string, error) {
			ctx, cancel := context.WithTimeout(context.Background(), fontDownloadTimeout)
			defer cancel()
			upd(20, "Đang tải Be Vietnam Pro…")
			if err := htmlvideo.EnsureVietFont(ctx, s.DataDir); err != nil {
				return "", err
			}
			upd(98, "Đã tải xong — video sẽ dùng font này từ lần render sau")
			return s.toolRelPath(htmlvideo.FontsDir(s.DataDir)), nil
		})
	writeJSON(w, http.StatusOK, j)
}
