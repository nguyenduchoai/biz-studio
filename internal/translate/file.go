package translate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"bizstudio/internal/store"
)

const (
	srtBatchSize  = 30   // số cue mỗi lần gọi LLM
	txtChunkChars = 3000 // độ dài mỗi chunk khi dịch file text
)

// File dịch một file (.srt theo batch cue, khác → chunk theo đoạn),
// ghi kết quả ra "<tên gốc>.<mã ngôn ngữ>.<ext>" cùng thư mục, trả đường dẫn tuyệt đối.
func File(ctx context.Context, st *store.Store, path, mode, engine, targetLang string, upd func(float64, string)) (string, error) {
	if upd == nil {
		upd = func(float64, string) {}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("không đọc được file %s: %w", path, err)
	}
	lang := normLang(targetLang)

	var out string
	if strings.EqualFold(filepath.Ext(path), ".srt") {
		out, err = translateSRT(ctx, st, string(raw), mode, engine, lang, upd)
	} else {
		out, err = translateChunks(ctx, st, string(raw), mode, engine, lang, upd)
	}
	if err != nil {
		return "", err
	}

	dst := outPath(path, lang)
	if err := os.WriteFile(dst, []byte(out), 0o644); err != nil {
		return "", fmt.Errorf("ghi file kết quả %s: %w", dst, err)
	}
	abs, err := filepath.Abs(dst)
	if err != nil {
		return dst, nil
	}
	return abs, nil
}

// translateSRT dịch theo batch ~30 cue, giữ nguyên timing.
func translateSRT(ctx context.Context, st *store.Store, content, mode, engine, lang string, upd func(float64, string)) (string, error) {
	cues := ParseSRT(content)
	if len(cues) == 0 {
		return "", fmt.Errorf("file SRT không có cue hợp lệ nào")
	}
	system := systemPrompt(mode, lang)
	for start := 0; start < len(cues); start += srtBatchSize {
		end := min(start+srtBatchSize, len(cues))
		if err := translateBatch(ctx, st, engine, system, cues[start:end]); err != nil {
			return "", fmt.Errorf("dịch cue %d–%d: %w", start+1, end, err)
		}
		upd(float64(end)/float64(len(cues))*100, fmt.Sprintf("Đã dịch %d/%d cue", end, len(cues)))
	}
	return BuildSRT(cues), nil
}

var numLineRe = regexp.MustCompile(`^\s*\[(\d+)\]\s*(.*)$`)

// translateBatch gộp mỗi cue thành 1 dòng đánh số [n], yêu cầu LLM trả đúng n dòng
// đánh số rồi map lại; dòng thiếu giữ nguyên bản gốc.
func translateBatch(ctx context.Context, st *store.Store, engine, system string, batch []Cue) error {
	var b strings.Builder
	for i, c := range batch {
		fmt.Fprintf(&b, "[%d] %s\n", i+1, strings.Join(c.Lines, " "))
	}
	user := fmt.Sprintf(
		"Dịch %d dòng phụ đề sau. Trả về ĐÚNG %d dòng, mỗi dòng bắt đầu bằng số thứ tự dạng [n], giữ nguyên thứ tự, không thêm bất kỳ nội dung nào khác:\n\n%s",
		len(batch), len(batch), b.String())

	resp, err := runEngine(ctx, st, engine, system, user)
	if err != nil {
		return err
	}
	got := map[int]string{}
	for _, ln := range strings.Split(resp, "\n") {
		if m := numLineRe.FindStringSubmatch(ln); m != nil {
			n, _ := strconv.Atoi(m[1])
			if txt := strings.TrimSpace(m[2]); txt != "" {
				got[n] = txt
			}
		}
	}
	for i := range batch {
		if txt, ok := got[i+1]; ok {
			batch[i].Lines = []string{txt}
		}
	}
	return nil
}

// translateChunks dịch file text theo chunk ~3000 ký tự (gom theo đoạn).
func translateChunks(ctx context.Context, st *store.Store, content, mode, engine, lang string, upd func(float64, string)) (string, error) {
	chunks := splitChunks(content, txtChunkChars)
	if len(chunks) == 0 {
		return "", fmt.Errorf("file không có nội dung để dịch")
	}
	system := systemPrompt(mode, lang)
	outs := make([]string, 0, len(chunks))
	for i, ch := range chunks {
		res, err := runEngine(ctx, st, engine, system, ch)
		if err != nil {
			return "", fmt.Errorf("dịch đoạn %d/%d: %w", i+1, len(chunks), err)
		}
		outs = append(outs, res)
		upd(float64(i+1)/float64(len(chunks))*100, fmt.Sprintf("Đã dịch %d/%d đoạn", i+1, len(chunks)))
	}
	return strings.Join(outs, "\n\n"), nil
}

// splitChunks gom các đoạn (ngăn cách bởi dòng trống) thành chunk ≤ maxChars.
func splitChunks(content string, maxChars int) []string {
	paras := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n\n")
	var chunks []string
	var cur strings.Builder
	for _, p := range paras {
		if strings.TrimSpace(p) == "" {
			continue
		}
		if cur.Len() > 0 && cur.Len()+len(p)+2 > maxChars {
			chunks = append(chunks, cur.String())
			cur.Reset()
		}
		if cur.Len() > 0 {
			cur.WriteString("\n\n")
		}
		cur.WriteString(p)
	}
	if strings.TrimSpace(cur.String()) != "" {
		chunks = append(chunks, cur.String())
	}
	return chunks
}

// outPath: "<tên gốc>.<mã ngôn ngữ>.<ext>" cùng thư mục file gốc.
func outPath(src, lang string) string {
	ext := filepath.Ext(src)
	return fmt.Sprintf("%s.%s%s", strings.TrimSuffix(src, ext), langCode(lang), ext)
}

// langCode đoán mã viết tắt của ngôn ngữ đích, mặc định "vi".
func langCode(lang string) string {
	l := strings.ToLower(strings.TrimSpace(lang))
	switch {
	case strings.Contains(l, "việt") || strings.Contains(l, "viet") || l == "vi":
		return "vi"
	case strings.Contains(l, "anh") || strings.Contains(l, "english") || l == "en":
		return "en"
	case strings.Contains(l, "nhật") || strings.Contains(l, "japan") || l == "ja":
		return "ja"
	case strings.Contains(l, "hàn") || strings.Contains(l, "korea") || l == "ko":
		return "ko"
	case strings.Contains(l, "trung") || strings.Contains(l, "chin") || l == "zh":
		return "zh"
	case strings.Contains(l, "pháp") || strings.Contains(l, "fren") || strings.Contains(l, "fran") || l == "fr":
		return "fr"
	default:
		return "vi"
	}
}
