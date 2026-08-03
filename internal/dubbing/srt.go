package dubbing

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"bizstudio/internal/translate"
)

// cue — một câu phụ đề đã quy đổi mốc thời gian ra giây.
type cue struct {
	Index int
	Start float64
	End   float64
	Text  string
}

// Slot trả độ dài khoảng thời gian dành cho câu này (giây).
func (c cue) Slot() float64 { return c.End - c.Start }

// loadCues đọc file SRT → danh sách cue có lời thoại, sắp xếp theo mốc bắt đầu.
func loadCues(path string) ([]cue, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("không đọc được file phụ đề %s: %w", path, err)
	}
	parsed := translate.ParseSRT(string(raw))
	if len(parsed) == 0 {
		return nil, fmt.Errorf("file phụ đề %s không có cue hợp lệ nào", path)
	}

	out := make([]cue, 0, len(parsed))
	for _, p := range parsed {
		text := strings.TrimSpace(strings.Join(p.Lines, " "))
		if text == "" {
			continue
		}
		start, end, err := parseTiming(p.Timing)
		if err != nil {
			return nil, fmt.Errorf("phụ đề %s, cue %d: %w", path, p.Index, err)
		}
		out = append(out, cue{Index: p.Index, Start: start, End: end, Text: text})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("file phụ đề %s không có câu thoại nào để lồng tiếng", path)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out, nil
}

// parseTiming tách dòng "HH:MM:SS,mmm --> HH:MM:SS,mmm" thành 2 mốc giây.
func parseTiming(timing string) (float64, float64, error) {
	left, right, ok := strings.Cut(timing, "-->")
	if !ok {
		return 0, 0, fmt.Errorf("dòng thời gian không hợp lệ: %q", strings.TrimSpace(timing))
	}
	start, err := parseTimestamp(left)
	if err != nil {
		return 0, 0, err
	}
	end, err := parseTimestamp(right)
	if err != nil {
		return 0, 0, err
	}
	if end < start {
		return 0, 0, fmt.Errorf("mốc kết thúc nhỏ hơn mốc bắt đầu: %q", strings.TrimSpace(timing))
	}
	return start, end, nil
}

// parseTimestamp đọc "HH:MM:SS,mmm" (chấp nhận "MM:SS,mmm" và dấu '.') ra giây.
// Phần toạ độ hiển thị phía sau timestamp (X1:… Y1:…) được bỏ qua.
func parseTimestamp(v string) (float64, error) {
	v = strings.TrimSpace(v)
	if i := strings.IndexAny(v, " \t"); i > 0 {
		v = v[:i]
	}
	v = strings.Replace(v, ",", ".", 1)
	parts := strings.Split(v, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, fmt.Errorf("mốc thời gian không hợp lệ: %q", v)
	}
	total := 0.0
	for _, p := range parts {
		n, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil || n < 0 {
			return 0, fmt.Errorf("mốc thời gian không hợp lệ: %q", v)
		}
		total = total*60 + n
	}
	return total, nil
}
