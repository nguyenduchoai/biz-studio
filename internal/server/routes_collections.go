package server

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"bizstudio/internal/highlight"
	"bizstudio/internal/vtemplate"
	"bizstudio/internal/whisper"
)

// routesCollections — từ MỘT video dài dựng NHIỀU hợp tuyển theo chủ đề.
func (s *Server) routesCollections(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/studio/collections", s.handleCollections)
}

// handleCollections bóc băng → chấm điểm → gom nhóm theo chủ đề → dựng mỗi
// nhóm một video.
//
// Khác "Rút clip ngắn" ở đúng một chỗ, nhưng là chỗ quyết định: rút clip lấy
// các đoạn đắt nhất bất kể nói về cái gì rồi ghép làm một; hợp tuyển đọc xem
// chúng nói về cái gì rồi tách ra nhiều video, mỗi video một chủ đề.
func (s *Server) handleCollections(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path        string `json:"path"`
		SecondsEach int    `json:"secondsEach"` // thời lượng mỗi hợp tuyển
		Max         int    `json:"max"`         // tối đa bao nhiêu hợp tuyển
		Platform    string `json:"platform"`
		MinScore    int    `json:"minScore"`
		Lang        string `json:"lang"`
		Genre       string `json:"genre"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	src, ok := s.toolSrcPath(w, body.Path)
	if !ok {
		return
	}
	secs := body.SecondsEach
	if secs <= 0 {
		secs = 60
	}
	maxCol := body.Max
	if maxCol <= 0 {
		maxCol = 4
	}
	minScore := float64(body.MinScore)
	if minScore <= 0 {
		minScore = 6
	}

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

	j := s.Jobs.Submit("collections", "", "Gom hợp tuyển: "+filepath.Base(src),
		func(upd func(float64, string)) (string, error) {
			ctx, cancel := context.WithTimeout(context.Background(), highlightTimeout)
			defer cancel()

			upd(4, "Bóc băng để biết video nói gì…")
			tr, err := whisper.Transcribe(ctx, s.st, src, body.Lang, func(p float64, d string) {
				upd(4+p*0.36, d)
			})
			if err != nil {
				return "", fmt.Errorf("bóc băng thất bại: %w", err)
			}
			cands := highlight.Candidates(tr)
			if len(cands) == 0 {
				return "", fmt.Errorf("không tách được đoạn nào từ bản bóc băng — video có tiếng nói không?")
			}

			genre := highlight.FindGenre(body.Genre)
			upd(42, fmt.Sprintf("AI đang chấm %d đoạn (%s)…", len(cands), genre.Name))
			scored, srep, err := highlight.Score(ctx, s.st, cands, secs, "", genre.ID,
				func(done, total int) {
					upd(42+float64(done)/float64(total)*20,
						fmt.Sprintf("AI đang chấm %d/%d đoạn…", done, total))
				})
			if err != nil {
				return "", err
			}

			upd(64, "AI đang gom các đoạn theo chủ đề…")
			cols, err := highlight.Cluster(ctx, s.st, scored, minScore, maxCol, secs, genre.ID)
			if err != nil {
				return "", err
			}

			base := strings.TrimSuffix(src, filepath.Ext(src))
			var names []string
			for i := range cols {
				col := &cols[i]
				pct := 66 + float64(i)/float64(len(cols))*30
				upd(pct, fmt.Sprintf("Dựng hợp tuyển %d/%d — %s (%d đoạn)…",
					i+1, len(cols), col.Title, len(col.Clips)))

				dst := fmt.Sprintf("%s.hoptuyen%d-%s.mp4", base, i+1, slugTitle(col.Title))
				cut := dst
				if plat != nil {
					cut = fmt.Sprintf("%s.hoptuyen%d.cut.mp4", base, i+1)
				}
				if _, err := highlight.Build(ctx, src, cut, tr, col.Clips, secs); err != nil {
					return "", fmt.Errorf("dựng hợp tuyển %q thất bại: %w", col.Title, err)
				}
				if plat != nil {
					if _, err := vtemplate.NormalizeForPlatform(ctx, cut, dst, plat.ID); err != nil {
						return "", fmt.Errorf("chuẩn hoá hợp tuyển %q thất bại: %w", col.Title, err)
					}
				}
				col.Output = s.toolRelPath(dst)
				names = append(names, col.Title)
				s.Log("info", "collections",
					fmt.Sprintf("Hợp tuyển %d/%d: %s — %d đoạn, %.0fs → %s",
						i+1, len(cols), col.Title, len(col.Clips), col.Sec, filepath.Base(dst)))
			}

			note := fmt.Sprintf("%d hợp tuyển: %s", len(cols), strings.Join(names, " · "))
			if srep.Warn != "" {
				note += " · ⚠ " + srep.Warn
			}
			upd(99, note)
			s.Log("info", "collections", note)
			// Trả video ĐẦU TIÊN để xem trước ngay được; danh sách đầy đủ nằm ở
			// nhật ký. Job chỉ mang được một đường dẫn output.
			return cols[0].Output, nil
		})
	writeJSON(w, http.StatusOK, j)
}

var reNonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// slugTitle biến tiêu đề tiếng Việt thành phần tên file an toàn.
//
// Bỏ dấu chứ không giữ: tên file có dấu đi qua zip/USB/máy Windows là hỏng mã,
// người dùng nhận về một mớ ký tự lạ không mở nổi.
func slugTitle(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		b.WriteString(deaccent(r))
	}
	out := reNonSlug.ReplaceAllString(b.String(), "-")
	out = strings.Trim(out, "-")
	if len(out) > 40 {
		out = strings.Trim(out[:40], "-")
	}
	if out == "" {
		out = "hop-tuyen"
	}
	return out
}

// deaccentMap dựng từ bảng nhóm nguyên âm, KHÔNG viết tay hai chuỗi song song.
//
// Bản đầu viết hai chuỗi "àáạả…" và "aaaa…" rồi tra theo vị trí. Đếm sai một ký
// tự là mọi chữ phía sau lệch hết — "đ" ra "y" — mà nhìn mã thì không thấy gì
// bất thường. Dựng bằng mã thì lệch là chuyện không thể xảy ra.
var deaccentMap = func() map[rune]rune {
	groups := map[rune]string{
		'a': "àáạảãâầấậẩẫăằắặẳẵ",
		'e': "èéẹẻẽêềếệểễ",
		'i': "ìíịỉĩ",
		'o': "òóọỏõôồốộổỗơờớợởỡ",
		'u': "ùúụủũưừứựửữ",
		'y': "ỳýỵỷỹ",
		'd': "đ",
	}
	m := map[rune]rune{}
	for plain, chars := range groups {
		for _, c := range chars {
			m[c] = plain
		}
	}
	return m
}()

// deaccent đổi một chữ cái tiếng Việt có dấu về chữ không dấu.
func deaccent(r rune) string {
	if p, ok := deaccentMap[r]; ok {
		return string(p)
	}
	return string(r)
}
