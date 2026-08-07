package server

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"bizstudio/internal/capcut"
	"bizstudio/internal/recap"
)

// recapRendering — các phiên đang dựng. Hai lần dựng cùng một phiên chạy song
// song sẽ giẫm lên nhau khi cùng ghi clips/seg (đo thật: clip 0 byte, track
// tiếng cụt) — nên phiên nào đang dựng thì lần bấm sau bị từ chối ngay.
var recapRendering sync.Map

// Routes cho "Phim → Kể chuyện": chia phim theo chuyển cảnh, AI xem khung hình
// viết lời dẫn, đọc giọng, dựng video lời kể đè phim.

const recapJobTimeout = 4 * time.Hour // phim dài: dò cảnh + TTS từng cảnh đều tốn thời gian

func (s *Server) routesRecap(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/tools/recap/analyze", s.handleRecapAnalyze)
	mux.HandleFunc("GET /api/tools/recap", s.handleRecapGet)
	mux.HandleFunc("POST /api/tools/recap/save", s.handleRecapSave)
	mux.HandleFunc("POST /api/tools/recap/render", s.handleRecapRender)
	mux.HandleFunc("POST /api/tools/recap/capcut", s.handleRecapCapcut)
}

// handleRecapAnalyze — POST /api/tools/recap/analyze: job chia cảnh + AI viết lời.
func (s *Server) handleRecapAnalyze(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path      string  `json:"path"`
		Style     string  `json:"style"`
		Threshold float64 `json:"threshold"`
		MinScene  float64 `json:"minScene"`
		MaxScenes int     `json:"maxScenes"`
		Narration string  `json:"narration"` // "ai" | "none"
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	src, ok := s.toolSrcPath(w, body.Path)
	if !ok {
		return
	}
	id := s.st.NewID("rc")
	opt := recap.AnalyzeOpts{
		Threshold: body.Threshold, MinScene: body.MinScene,
		MaxScenes: body.MaxScenes, Style: body.Style, Narration: body.Narration,
	}
	j := s.Jobs.Submit("recap_analyze", "", "Chia cảnh & viết lời: "+filepath.Base(src),
		func(upd func(float64, string)) (string, error) {
			ctx, cancel := context.WithTimeout(context.Background(), recapJobTimeout)
			defer cancel()
			m, err := recap.Analyze(ctx, s.st, src, s.toolRelPath(src), id, opt, upd)
			if err != nil {
				return "", err
			}
			note := ""
			if m.NarrationNote != "" {
				note = " — " + m.NarrationNote
			}
			s.Log("info", "recap", fmt.Sprintf("Đã phân tích %s: %d cảnh%s", filepath.Base(src), len(m.Scenes), note))
			return s.toolRelPath(recap.ManifestPath(s.DataDir, id)), nil
		})
	writeJSON(w, http.StatusOK, j)
}

// handleRecapGet — GET /api/tools/recap?path=<manifest tương đối>.
func (s *Server) handleRecapGet(w http.ResponseWriter, r *http.Request) {
	p, ok := s.toolSrcPath(w, r.URL.Query().Get("path"))
	if !ok {
		return
	}
	m, err := recap.Load(p)
	if err != nil {
		httpErr(w, http.StatusBadRequest, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// handleRecapSave — POST /api/tools/recap/save: ghi lời dẫn người dùng sửa.
func (s *Server) handleRecapSave(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path   string `json:"path"`
		Scenes []struct {
			Index int    `json:"index"`
			Text  string `json:"text"`
		} `json:"scenes"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	p, ok := s.toolSrcPath(w, body.Path)
	if !ok {
		return
	}
	m, err := recap.Load(p)
	if err != nil {
		httpErr(w, http.StatusBadRequest, "%v", err)
		return
	}
	byIdx := map[int]string{}
	for _, sc := range body.Scenes {
		byIdx[sc.Index] = sc.Text
	}
	for i := range m.Scenes {
		if t, hit := byIdx[m.Scenes[i].Index]; hit {
			m.Scenes[i].Text = strings.TrimSpace(t)
		}
	}
	if err := m.Save(s.DataDir); err != nil {
		httpErr(w, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// handleRecapRender — POST /api/tools/recap/render: job dựng video kể chuyện.
func (s *Server) handleRecapRender(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path         string  `json:"path"` // manifest tương đối
		Voice        string  `json:"voice"`
		Engine       string  `json:"engine"`
		KeepOriginal *bool   `json:"keepOriginal"` // nil = true
		OrigVolume   float64 `json:"origVolume"`
		BurnSub      bool    `json:"burnSub"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	p, ok := s.toolSrcPath(w, body.Path)
	if !ok {
		return
	}
	m, err := recap.Load(p)
	if err != nil {
		httpErr(w, http.StatusBadRequest, "%v", err)
		return
	}
	hasText := false
	for _, sc := range m.Scenes {
		if strings.TrimSpace(sc.Text) != "" {
			hasText = true
			break
		}
	}
	if !hasText {
		httpErr(w, http.StatusBadRequest,
			"chưa cảnh nào có lời dẫn — viết lời (hoặc chạy lại phân tích với AI) rồi mới dựng")
		return
	}
	srcAbs, ok := s.toolSrcPath(w, m.Source)
	if !ok {
		return
	}
	opt := recap.RenderOpts{
		Voice: body.Voice, Engine: body.Engine,
		KeepOriginal: body.KeepOriginal == nil || *body.KeepOriginal,
		OrigVolume:   body.OrigVolume, BurnSub: body.BurnSub,
	}
	if _, busy := recapRendering.LoadOrStore(m.ID, true); busy {
		httpErr(w, http.StatusConflict, "phiên này đang được dựng — chờ xong rồi hãy bấm lại")
		return
	}
	j := s.Jobs.Submit("recap_render", "", "Dựng video kể chuyện: "+filepath.Base(srcAbs),
		func(upd func(float64, string)) (string, error) {
			defer recapRendering.Delete(m.ID)
			ctx, cancel := context.WithTimeout(context.Background(), recapJobTimeout)
			defer cancel()
			res, err := recap.Render(ctx, s.st, m, srcAbs, opt, upd)
			if err != nil {
				return "", err
			}
			s.Log("info", "recap", fmt.Sprintf("Đã dựng video kể chuyện %s — %.1fs, %d cảnh có lời",
				filepath.Base(res.VideoPath), res.Duration, res.Voiced))
			return s.toolRelPath(res.VideoPath), nil
		})
	writeJSON(w, http.StatusOK, j)
}

// handleRecapCapcut — POST /api/tools/recap/capcut: xuất phiên thành dự án
// CapCut (.draft) để dựng tiếp trong trình dựng người dùng quen tay.
func (s *Server) handleRecapCapcut(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	p, ok := s.toolSrcPath(w, body.Path)
	if !ok {
		return
	}
	m, err := recap.Load(p)
	if err != nil {
		httpErr(w, http.StatusBadRequest, "%v", err)
		return
	}
	srcAbs, ok := s.toolSrcPath(w, m.Source)
	if !ok {
		return
	}
	j := s.Jobs.Submit("recap_capcut", "", "Xuất CapCut: "+m.ID,
		func(upd func(float64, string)) (string, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			upd(20, "Dựng dự án CapCut từ danh sách cảnh…")
			res, err := capcut.FromRecap(ctx, s.DataDir, m, srcAbs)
			if err != nil {
				return "", err
			}
			note := fmt.Sprintf("%d cảnh có lời", res.Voiced)
			if res.Overflow > 0 {
				note += fmt.Sprintf(" · %d lời tràn cảnh nằm ở track lời thứ hai", res.Overflow)
			}
			if res.Clamped > 0 {
				note += fmt.Sprintf(" · %d lời bị dời/cắt hiển thị vì tràn quá dài", res.Clamped)
			}
			upd(98, note)
			s.Log("info", "recap", "Đã xuất dự án CapCut: "+filepath.Base(res.ZipPath)+" — "+note)
			return s.toolRelPath(res.ZipPath), nil
		})
	writeJSON(w, http.StatusOK, j)
}
