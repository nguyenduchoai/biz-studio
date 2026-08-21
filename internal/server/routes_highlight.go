package server

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"bizstudio/internal/highlight"
	"bizstudio/internal/vtemplate"
	"bizstudio/internal/whisper"
)

const highlightTimeout = 90 * time.Minute

// routesHighlight — rút video dài thành clip ngắn.
func (s *Server) routesHighlight(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/studio/highlight", s.handleHighlight)
	mux.HandleFunc("GET /api/studio/highlight/genres", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, highlight.Genres())
	})
}

// handleHighlight rút một video dài thành clip ngắn theo bốn bước: bóc băng →
// AI chấm điểm từng đoạn → chọn cho vừa thời lượng → cắt ghép theo thứ tự thời
// gian, rồi chuẩn hoá cho nền tảng nếu được yêu cầu.
func (s *Server) handleHighlight(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path     string `json:"path"`     // tương đối trong data dir
		Seconds  int    `json:"seconds"`  // thời lượng clip muốn có
		Platform string `json:"platform"` // rỗng = không chuẩn hoá
		Goal     string `json:"goal"`     // clip nhắm vào ý gì (tuỳ chọn)
		MinScore int    `json:"minScore"` // 0 = 6
		Lang     string `json:"lang"`     // ngôn ngữ bóc băng; rỗng = tự đoán
		Genre    string `json:"genre"`    // thể loại nội dung; rỗng = "auto"
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	src, ok := s.toolSrcPath(w, body.Path)
	if !ok {
		return
	}
	secs := body.Seconds
	if secs <= 0 {
		secs = 60
	}
	minScore := float64(body.MinScore)
	if minScore <= 0 {
		minScore = 6
	}

	// Trần nền tảng thắng thời lượng người dùng gõ: vượt trần là video bị xếp
	// sang loại khác, không còn là clip ngắn nữa.
	var plat *vtemplate.Platform
	if pid := strings.TrimSpace(body.Platform); pid != "" {
		p, found := vtemplate.FindPlatform(pid)
		if !found {
			httpErr(w, http.StatusBadRequest, "không có nền tảng %q", pid)
			return
		}
		plat = &p
		if p.MaxSec > 0 && secs > p.MaxSec {
			secs = p.MaxSec
		}
	}

	base := strings.TrimSuffix(src, filepath.Ext(src))
	dst := fmt.Sprintf("%s.short%ds.mp4", base, secs)

	j := s.Jobs.Submit("highlight", "", "Rút clip "+fmt.Sprint(secs)+"s: "+filepath.Base(src),
		func(upd func(float64, string)) (string, error) {
			ctx, cancel := context.WithTimeout(context.Background(), highlightTimeout)
			defer cancel()

			upd(5, "Bóc băng để biết video nói gì…")
			tr, err := whisper.Transcribe(ctx, s.st, src, body.Lang, func(p float64, d string) {
				upd(5+p*0.45, d) // bóc băng chiếm phần lớn thời gian
			})
			if err != nil {
				return "", fmt.Errorf("bóc băng thất bại: %w", err)
			}
			cands := highlight.Candidates(tr)
			if len(cands) == 0 {
				return "", fmt.Errorf("không tách được đoạn nào từ bản bóc băng — video có tiếng nói không?")
			}

			// Chấm theo lô: mỗi lô một lượt gọi AI, nên báo tiến trình theo lô
			// thay vì để thanh tiến trình đứng im hàng phút với video dài.
			genre := highlight.FindGenre(body.Genre)
			upd(55, fmt.Sprintf("AI đang chấm %d đoạn (%s)…", len(cands), genre.Name))
			scored, srep, err := highlight.Score(ctx, s.st, cands, secs, body.Goal, genre.ID,
				func(done, total int) {
					upd(55+float64(done)/float64(total)*15,
						fmt.Sprintf("AI đang chấm %d/%d đoạn (%s)…", done, total, genre.Name))
				})
			if err != nil {
				return "", err
			}
			chosen := highlight.Pick(scored, secs, minScore)
			if len(chosen) == 0 {
				return "", fmt.Errorf("không đoạn nào đạt điểm %.0f trở lên — hạ ngưỡng điểm hoặc tăng thời lượng đích", minScore)
			}

			upd(72, fmt.Sprintf("Cắt %d đoạn đắt nhất trong %d đoạn…", len(chosen), len(cands)))
			maxSec := secs
			if plat != nil && plat.MaxSec > 0 && plat.MaxSec < maxSec {
				maxSec = plat.MaxSec
			}
			cut := dst
			if plat != nil {
				cut = base + ".short.cut.mp4" // bản chưa chuẩn hoá, làm trung gian
			}
			rep, err := highlight.Build(ctx, src, cut, tr, chosen, maxSec)
			if err != nil {
				return "", err
			}
			note := fmt.Sprintf("%.0fs → %.0fs · giữ %d/%d đoạn",
				rep.SourceSec, rep.OutputSec, rep.Kept, len(cands))
			if rep.Truncated {
				note += " · đoạn cuối bị cắt cụt cho vừa trần"
			}
			if srep.Warn != "" {
				note += " · ⚠ " + srep.Warn
			}

			if plat != nil {
				upd(88, fmt.Sprintf("Chuẩn hoá cho %s…", plat.Name))
				nrep, err := vtemplate.NormalizeForPlatform(ctx, cut, dst, plat.ID)
				if err != nil {
					return "", err
				}
				note += fmt.Sprintf(" · %dx%d → %dx%d", nrep.FromW, nrep.FromH, nrep.ToW, nrep.ToH)
				if nrep.TextWarn != "" {
					note += " · ⚠ " + nrep.TextWarn
				}
			}
			upd(98, note)
			s.Log("info", "highlight", "Rút clip "+filepath.Base(dst)+" — "+note)
			return s.toolRelPath(dst), nil
		})
	writeJSON(w, http.StatusOK, j)
}
