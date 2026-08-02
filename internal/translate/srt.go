// Package translate — dịch văn bản/phụ đề SRT bằng Claude CLI hoặc Gemini,
// giữ nguyên timing của phụ đề.
package translate

import (
	"fmt"
	"strconv"
	"strings"
)

// Cue — một khối phụ đề SRT.
type Cue struct {
	Index  int
	Timing string
	Lines  []string
}

// ParseSRT tách nội dung SRT thành các cue (bỏ qua block hỏng, giữ nguyên timing).
func ParseSRT(content string) []Cue {
	content = strings.TrimPrefix(content, "\ufeff") // bỏ BOM nếu có
	content = strings.ReplaceAll(content, "\r\n", "\n")

	var cues []Cue
	var block []string
	flush := func() {
		if c, ok := parseBlock(block); ok {
			cues = append(cues, c)
		}
		block = block[:0]
	}
	for _, ln := range strings.Split(content, "\n") {
		if strings.TrimSpace(ln) == "" {
			flush()
			continue
		}
		block = append(block, ln)
	}
	flush()

	// Block thiếu số thứ tự → đánh lại theo vị trí.
	for i := range cues {
		if cues[i].Index == 0 {
			cues[i].Index = i + 1
		}
	}
	return cues
}

// parseBlock: dòng 1 là số thứ tự (tùy chọn), tiếp theo là timing "-->", còn lại là text.
func parseBlock(block []string) (Cue, bool) {
	if len(block) == 0 {
		return Cue{}, false
	}
	var c Cue
	i := 0
	if n, err := strconv.Atoi(strings.TrimSpace(block[0])); err == nil && len(block) > 1 {
		c.Index = n
		i = 1
	}
	if i >= len(block) || !strings.Contains(block[i], "-->") {
		return Cue{}, false
	}
	c.Timing = strings.TrimSpace(block[i])
	i++
	c.Lines = append(c.Lines, block[i:]...)
	return c, true
}

// BuildSRT ghép các cue lại thành nội dung SRT hoàn chỉnh (giữ nguyên timing).
func BuildSRT(cues []Cue) string {
	var b strings.Builder
	for i, c := range cues {
		idx := c.Index
		if idx == 0 {
			idx = i + 1
		}
		fmt.Fprintf(&b, "%d\n%s\n", idx, c.Timing)
		for _, ln := range c.Lines {
			b.WriteString(ln)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}
