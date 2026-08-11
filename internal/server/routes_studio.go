package server

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"bizstudio/internal/tts"
	"bizstudio/internal/vtemplate"
)

// Routes cho "xưởng làm sẵn": khuôn theo lĩnh vực, preset nền tảng, tone nhạc,
// và danh sách giọng gom theo ngôn ngữ.

func (s *Server) routesStudio(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/studio/templates", s.handleTemplates)
	mux.HandleFunc("GET /api/studio/platforms", s.handlePlatforms)
	mux.HandleFunc("POST /api/studio/normalize", s.handleNormalize)
	mux.HandleFunc("GET /api/studio/moods", s.handleMoods)
	mux.HandleFunc("POST /api/studio/mood-for", s.handleMoodFor)
	mux.HandleFunc("GET /api/studio/voice-langs", s.handleVoiceLangs)
}

func (s *Server) handleTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"categories": vtemplate.Categories(),
		"templates":  vtemplate.All(),
	})
}

func (s *Server) handlePlatforms(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, vtemplate.Platforms())
}

// handleNormalize — chuẩn hoá một video cho đúng chuẩn phát của nền tảng.
func (s *Server) handleNormalize(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path     string `json:"path"`
		Platform string `json:"platform"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	src, ok := s.toolSrcPath(w, body.Path)
	if !ok {
		return
	}
	p, found := vtemplate.FindPlatform(body.Platform)
	if !found {
		httpErr(w, http.StatusBadRequest, "không có nền tảng %q", body.Platform)
		return
	}
	dst := strings.TrimSuffix(src, filepath.Ext(src)) + "." + p.ID + ".mp4"
	j := s.Jobs.Submit("normalize", "", "Chuẩn hoá cho "+p.Name+": "+filepath.Base(src),
		func(upd func(float64, string)) (string, error) {
			ctx, cancel := context.WithTimeout(context.Background(), toolJobTimeout)
			defer cancel()
			upd(15, fmt.Sprintf("Đưa về %dx%d, cân độ to %.0f LUFS…", p.Width, p.Height, p.LUFS))
			rep, err := vtemplate.NormalizeForPlatform(ctx, src, dst, p.ID)
			if err != nil {
				return "", err
			}
			note := fmt.Sprintf("%dx%d → %dx%d · %.0fs", rep.FromW, rep.FromH, rep.ToW, rep.ToH, rep.Duration)
			if rep.Padded {
				note += " · đã thêm viền cho vừa khung (không bóp méo hình)"
			}
			if rep.OverLimit {
				note += " · ⚠ " + rep.Note
			}
			if rep.TextWarn != "" {
				note += " · ⚠ " + rep.TextWarn
			}
			upd(98, note)
			s.Log("info", "studio", "Chuẩn hoá "+filepath.Base(dst)+" — "+note)
			return s.toolRelPath(dst), nil
		})
	writeJSON(w, http.StatusOK, j)
}

func (s *Server) handleMoods(w http.ResponseWriter, r *http.Request) {
	dir := filepath.Join(s.DataDir, "music")
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
	if err := vtemplate.EnsureAllMoods(ctx, dir); err != nil {
		httpErr(w, http.StatusInternalServerError, "%v", err)
		return
	}
	type item struct {
		vtemplate.Mood
		Path string `json:"path"`
	}
	list := vtemplate.Moods()
	out := make([]item, 0, len(list))
	for _, m := range list {
		out = append(out, item{Mood: m, Path: s.toolRelPath(filepath.Join(dir, m.ID+".wav"))})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleMoodFor gợi ý tone nhạc theo nội dung kịch bản.
func (s *Server) handleMoodFor(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text string `json:"text"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	id := vtemplate.MoodForScript(body.Text)
	if id == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"mood": "", "note": "Không đủ tin để đoán tone — bạn tự chọn cho chắc.",
		})
		return
	}
	m, _ := vtemplate.FindMood(id)
	writeJSON(w, http.StatusOK, map[string]any{"mood": m.ID, "name": m.Name, "desc": m.Desc})
}

// handleVoiceLangs trả danh sách giọng đã gom theo ngôn ngữ.
func (s *Server) handleVoiceLangs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, tts.GroupByLang(tts.VoicesFor(s.st)))
}
