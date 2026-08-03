package server

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"bizstudio/internal/dubbing"
)

func (s *Server) routesDubbing(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/tools/dub", s.handleToolDub)
}

// dubRequest — body của POST /api/tools/dub.
// fitTiming dùng con trỏ để phân biệt "không gửi" (mặc định bật) với false.
type dubRequest struct {
	VideoPath       string  `json:"videoPath"`
	SrtPath         string  `json:"srtPath"`
	Voice           string  `json:"voice"`
	Engine          string  `json:"engine"`
	Style           string  `json:"style"`
	TargetLang      string  `json:"targetLang"`
	TranslateEngine string  `json:"translateEngine"`
	KeepOriginal    bool    `json:"keepOriginal"`
	OriginalVolume  float64 `json:"originalVolume"`
	FitTiming       *bool   `json:"fitTiming"`
	MaxSpeed        float64 `json:"maxSpeed"`
}

// handleToolDub — job kind=dub: lồng tiếng video theo phụ đề SRT.
// Cần ít nhất videoPath hoặc srtPath. Chưa có SRT → tự bóc băng bằng Gemini.
func (s *Server) handleToolDub(w http.ResponseWriter, r *http.Request) {
	var body dubRequest
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}

	video := strings.TrimSpace(body.VideoPath)
	srt := strings.TrimSpace(body.SrtPath)
	if video == "" && srt == "" {
		httpErr(w, http.StatusBadRequest, "cần chọn video hoặc file phụ đề .srt để lồng tiếng")
		return
	}
	if video != "" {
		var ok bool
		if video, ok = s.toolSrcPath(w, video); !ok {
			return
		}
	}
	if srt != "" {
		var ok bool
		if srt, ok = s.toolSrcPath(w, srt); !ok {
			return
		}
	}

	cfg := dubbing.Config{
		Voice:           body.Voice,
		Engine:          body.Engine,
		Style:           body.Style,
		TargetLang:      body.TargetLang,
		TranslateEngine: body.TranslateEngine,
		KeepOriginal:    body.KeepOriginal,
		OriginalVolume:  body.OriginalVolume,
		FitTiming:       body.FitTiming == nil || *body.FitTiming,
		MaxSpeed:        body.MaxSpeed,
	}
	workDir := filepath.Join(s.DataDir, "tmp", s.st.NewID("dub"))

	j := s.Jobs.Submit("dub", "", "Lồng tiếng: "+dubLabel(video, srt), func(upd func(float64, string)) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), toolJobTimeout)
		defer cancel()

		res, err := dubbing.Run(ctx, s.st, video, srt, cfg, workDir, upd)
		if err != nil {
			return "", err
		}
		out := res.VideoPath
		if out == "" {
			out = res.AudioPath
		}
		if out == "" {
			return "", fmt.Errorf("lồng tiếng không tạo được file kết quả nào")
		}
		return s.toolRelPath(out), nil
	})
	writeJSON(w, http.StatusOK, j)
}

// dubLabel — nhãn hiển thị job theo file nguồn (ưu tiên video).
func dubLabel(video, srt string) string {
	if video != "" {
		return shortText(filepath.Base(video), 50)
	}
	return shortText(filepath.Base(srt), 50)
}
