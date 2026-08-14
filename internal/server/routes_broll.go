package server

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"bizstudio/internal/broll"
	"bizstudio/internal/vtemplate"
)

const brollTimeout = 60 * time.Minute

// routesBroll — ghép clip tư liệu khớp lời đọc.
func (s *Server) routesBroll(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/studio/broll", s.handleBroll)
}

// handleBroll ghép một thư mục clip tư liệu thành dải hình khớp đúng độ dài của
// một bản thu tiếng. Tiếng là thứ dẫn: hình cắt cho vừa tiếng, không phải ngược
// lại — tiếng đã thu rồi, co kéo tiếng là hỏng lời đọc.
func (s *Server) handleBroll(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ClipsDir string  `json:"clipsDir"` // thư mục chứa clip, tương đối trong data
		Audio    string  `json:"audio"`    // file tiếng, tương đối trong data
		Aspect   string  `json:"aspect"`   // 9:16 | 3:4 | 16:9 | 1:1; rỗng = theo clip đầu
		MaxClip  float64 `json:"maxClip"`  // mỗi mẩu tối đa bao nhiêu giây; 0 = 5
		FPS      int     `json:"fps"`      // 0 = 30
		Shuffle  bool    `json:"shuffle"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	dir, ok := s.toolSrcDir(w, body.ClipsDir)
	if !ok {
		return
	}
	audio, ok := s.toolSrcPath(w, body.Audio)
	if !ok {
		return
	}
	clips, err := broll.ListClips(dir)
	if err != nil {
		httpErr(w, http.StatusBadRequest, "đọc thư mục clip thất bại: %v", err)
		return
	}
	if len(clips) == 0 {
		httpErr(w, http.StatusBadRequest,
			"thư mục %q không có file video nào (nhận .mp4 .mov .mkv .webm .m4v .avi)", body.ClipsDir)
		return
	}

	opt := broll.Opt{MaxClipSec: body.MaxClip, FPS: body.FPS, Shuffle: body.Shuffle}
	if a := strings.TrimSpace(body.Aspect); a != "" {
		opt.Width, opt.Height = vtemplate.AspectSize(a)
	}
	dst := strings.TrimSuffix(audio, filepath.Ext(audio)) + ".broll.mp4"

	j := s.Jobs.Submit("broll", "", fmt.Sprintf("Ghép %d clip tư liệu: %s", len(clips), filepath.Base(audio)),
		func(upd func(float64, string)) (string, error) {
			ctx, cancel := context.WithTimeout(context.Background(), brollTimeout)
			defer cancel()
			upd(10, fmt.Sprintf("Cắt %d clip thành mẩu và ghép cho vừa lời đọc…", len(clips)))
			rep, err := broll.Assemble(ctx, clips, audio, dst, opt)
			if err != nil {
				return "", err
			}
			note := fmt.Sprintf("lời đọc %.1fs → hình %.1fs · %d mẩu từ %d/%d clip · %dx%d",
				rep.AudioSec, rep.VideoSec, rep.Pieces, rep.Clips, len(clips), rep.Width, rep.Height)
			if rep.ShortFall {
				note += fmt.Sprintf(" · ⚠ tư liệu không đủ dài, đã phải dùng lại %d vòng — "+
					"thêm clip vào thư mục để hình đỡ lặp", rep.Reused)
			}
			upd(98, note)
			s.Log("info", "broll", "Ghép tư liệu "+filepath.Base(dst)+" — "+note)
			return s.toolRelPath(dst), nil
		})
	writeJSON(w, http.StatusOK, j)
}
