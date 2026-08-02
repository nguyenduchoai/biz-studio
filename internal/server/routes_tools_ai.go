package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"bizstudio/internal/gemini"
	"bizstudio/internal/media"
	"bizstudio/internal/vox"
)

// handleToolASR — job kind=asr: tách audio 16kHz → Gemini bóc băng → <src>.srt.
func (s *Server) handleToolASR(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
		Lang string `json:"lang"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	src, ok := s.toolSrcPath(w, body.Path)
	if !ok {
		return
	}
	lang := strings.TrimSpace(body.Lang)
	if lang == "" {
		lang = "tiếng Việt"
	}

	j := s.Jobs.Submit("asr", "", "Bóc băng: "+filepath.Base(src), func(upd func(float64, string)) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		upd(5, "Tách audio 16kHz…")
		wav := filepath.Join(s.DataDir, "tmp", s.st.NewID("asr")+".wav")
		if err := media.ExtractAudioWav16k(ctx, src, wav); err != nil {
			return "", fmt.Errorf("tách audio thất bại: %w", err)
		}
		defer os.Remove(wav)

		upd(35, "Gửi Gemini bóc băng…")
		prompt := fmt.Sprintf(
			"Bóc băng audio này thành phụ đề SRT chuẩn (số thứ tự, HH:MM:SS,mmm --> HH:MM:SS,mmm, mỗi cue tối đa 2 dòng), ngôn ngữ %s. Chỉ trả về nội dung SRT, không giải thích gì thêm.",
			lang)
		text, err := gemini.NewFromSettings(s.st).GenerateWithFiles(ctx, prompt, []string{wav})
		if err != nil {
			return "", err
		}
		srt := stripCodeFence(text)
		if strings.TrimSpace(srt) == "" {
			return "", fmt.Errorf("Gemini không trả về nội dung SRT nào")
		}

		dst := src + ".srt"
		if err := os.WriteFile(dst, []byte(srt+"\n"), 0o644); err != nil {
			return "", fmt.Errorf("ghi file SRT %s: %w", dst, err)
		}
		upd(95, "Đã ghi phụ đề")
		return s.toolRelPath(dst), nil
	})
	writeJSON(w, http.StatusOK, j)
}

// ocrCue — 1 dòng chữ bóc được từ khung hình số idx (1-based).
type ocrCue struct {
	idx  int
	text string
}

// handleToolOCR — job kind=ocr: trích frame theo fps → Gemini bóc chữ theo lô ≤10
// frame → gộp thành SRT <src>.ocr.srt (timestamp suy từ fps, chất lượng tương đối).
func (s *Server) handleToolOCR(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string  `json:"path"`
		FPS  float64 `json:"fps"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	src, ok := s.toolSrcPath(w, body.Path)
	if !ok {
		return
	}
	fps := body.FPS
	if fps <= 0 {
		fps = 0.5
	}

	j := s.Jobs.Submit("ocr", "", "OCR video: "+filepath.Base(src), func(upd func(float64, string)) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
		defer cancel()

		upd(3, "Trích khung hình…")
		tmpDir := filepath.Join(s.DataDir, "tmp", s.st.NewID("ocr"))
		frames, err := media.ExtractFrames(ctx, src, tmpDir, fps)
		if err != nil {
			return "", err
		}
		defer os.RemoveAll(tmpDir)

		cli := gemini.NewFromSettings(s.st)
		const batchSize = 10
		var cues []ocrCue
		for start := 0; start < len(frames); start += batchSize {
			end := min(start+batchSize, len(frames))
			upd(5+float64(end)/float64(len(frames))*88,
				fmt.Sprintf("Bóc chữ khung %d–%d/%d…", start+1, end, len(frames)))
			prompt := fmt.Sprintf(
				"Các ảnh sau là khung hình trích từ video với tốc độ %g khung hình/giây. "+
					"Ảnh thứ nhất trong lô này là khung số %d của video (ảnh kế tiếp là khung %d, %d…); "+
					"thời điểm giây của một khung = (số khung - 1) / %g. "+
					"Đọc toàn bộ chữ hiển thị trên từng khung hình. "+
					"Trả về mỗi khung có chữ đúng MỘT dòng theo dạng: <số khung>|<chữ đọc được>. "+
					"Khung không có chữ thì bỏ qua. Không giải thích gì thêm.",
				fps, start+1, start+2, start+3, fps)
			resp, err := cli.GenerateWithFiles(ctx, prompt, frames[start:end])
			if err != nil {
				return "", fmt.Errorf("bóc chữ khung %d–%d: %w", start+1, end, err)
			}
			cues = append(cues, parseOCRLines(stripCodeFence(resp), len(frames))...)
		}
		if len(cues) == 0 {
			return "", fmt.Errorf("không bóc được chữ nào từ video")
		}

		dst := src + ".ocr.srt"
		if err := os.WriteFile(dst, []byte(buildOCRSRT(cues, fps)), 0o644); err != nil {
			return "", fmt.Errorf("ghi file SRT %s: %w", dst, err)
		}
		upd(97, "Đã ghi phụ đề OCR")
		return s.toolRelPath(dst), nil
	})
	writeJSON(w, http.StatusOK, j)
}

var ocrLineRe = regexp.MustCompile(`^\s*(?:khung\s*)?(\d+)\s*[|:]\s*(.+?)\s*$`)

// parseOCRLines bóc các dòng "<số khung>|<text>" từ output của LLM.
func parseOCRLines(resp string, maxFrame int) []ocrCue {
	var out []ocrCue
	for _, ln := range strings.Split(resp, "\n") {
		m := ocrLineRe.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		idx, err := strconv.Atoi(m[1])
		if err != nil || idx < 1 || idx > maxFrame {
			continue
		}
		out = append(out, ocrCue{idx: idx, text: m[2]})
	}
	return out
}

// buildOCRSRT gộp cue OCR thành nội dung SRT (sort theo khung, dedup theo khung).
func buildOCRSRT(cues []ocrCue, fps float64) string {
	sort.SliceStable(cues, func(i, j int) bool { return cues[i].idx < cues[j].idx })
	var b strings.Builder
	n, lastIdx := 0, -1
	for _, c := range cues {
		if c.idx == lastIdx {
			continue
		}
		lastIdx = c.idx
		n++
		startS := float64(c.idx-1) / fps
		endS := float64(c.idx) / fps
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", n, srtTimestamp(startS), srtTimestamp(endS), c.text)
	}
	return b.String()
}

// srtTimestamp: giây → "HH:MM:SS,mmm".
func srtTimestamp(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	ms := int(math.Round(sec * 1000))
	h := ms / 3600000
	ms %= 3600000
	m := ms / 60000
	ms %= 60000
	s := ms / 1000
	ms %= 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

// handleToolScenes — sinh kịch bản cảnh đồng bộ (timeout 180s):
// Gemini nếu có API key, ngược lại Claude CLI.
func (s *Server) handleToolScenes(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Content string `json:"content"`
		Count   int    `json:"count"`
		Style   string `json:"style"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	content := strings.TrimSpace(body.Content)
	if content == "" {
		httpErr(w, http.StatusBadRequest, "thiếu nội dung để tạo kịch bản")
		return
	}
	count := body.Count
	if count <= 0 {
		count = 8
	}
	style := strings.TrimSpace(body.Style)
	if style == "" {
		style = "tự nhiên, cuốn hút"
	}

	prompt := fmt.Sprintf(`Tạo kịch bản video gồm %d cảnh từ nội dung dưới đây, phong cách %s.
Trả về DUY NHẤT một JSON mảng theo đúng dạng:
[{"title":"tiêu đề ngắn","voiceText":"lời đọc của cảnh","mediaKeyword":"từ khóa tìm ảnh minh họa","duration":4}]
Không giải thích, không markdown, không văn bản nào khác ngoài JSON.

Nội dung:
%s`, count, style, content)

	ctx, cancel := context.WithTimeout(r.Context(), 180*time.Second)
	defer cancel()

	var raw string
	var err error
	if strings.TrimSpace(s.st.Settings().GeminiAPIKey) != "" {
		raw, err = gemini.NewFromSettings(s.st).GenerateText(ctx, "", prompt)
	} else {
		raw, err = runClaudePrompt(ctx, s.st.Settings().ClaudeBin, prompt)
	}
	if err != nil {
		httpErr(w, http.StatusInternalServerError, "tạo kịch bản thất bại: %v", err)
		return
	}

	var scenes []vox.Scene
	if err := json.Unmarshal([]byte(extractJSONArray(stripCodeFence(raw))), &scenes); err != nil {
		httpErr(w, http.StatusInternalServerError, "LLM trả về JSON không hợp lệ: %v", err)
		return
	}
	if len(scenes) == 0 {
		httpErr(w, http.StatusInternalServerError, "LLM không trả về cảnh nào")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"scenes": scenes})
}

// runClaudePrompt chạy Claude CLI: <bin> -p --output-format text, prompt qua stdin.
func runClaudePrompt(ctx context.Context, bin, prompt string) (string, error) {
	bin = strings.TrimSpace(bin)
	if bin == "" {
		bin = "claude"
	}
	cmd := exec.CommandContext(ctx, bin, "-p", "--output-format", "text")
	cmd.Stdin = strings.NewReader(prompt)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(se.String())
		if len(msg) > 400 {
			msg = msg[:400] + "…"
		}
		return "", fmt.Errorf("chạy Claude CLI (%s) thất bại: %w — %s", bin, err, msg)
	}
	out := strings.TrimSpace(so.String())
	if out == "" {
		return "", fmt.Errorf("Claude CLI không trả về nội dung nào")
	}
	return out, nil
}

// stripCodeFence bỏ khối ```lang … ``` bao quanh output LLM (nếu có).
func stripCodeFence(v string) string {
	t := strings.TrimSpace(v)
	if !strings.HasPrefix(t, "```") {
		return t
	}
	t = strings.TrimPrefix(t, "```")
	if i := strings.Index(t, "\n"); i >= 0 {
		t = t[i+1:]
	} else {
		t = ""
	}
	if i := strings.LastIndex(t, "```"); i >= 0 {
		t = t[:i]
	}
	return strings.TrimSpace(t)
}

// extractJSONArray cắt từ '[' đầu tiên tới ']' cuối cùng (phòng LLM kèm lời dẫn).
func extractJSONArray(v string) string {
	i := strings.Index(v, "[")
	j := strings.LastIndex(v, "]")
	if i >= 0 && j > i {
		return v[i : j+1]
	}
	return v
}
