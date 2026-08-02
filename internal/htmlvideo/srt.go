package htmlvideo

import (
	"fmt"
	"os"
	"strings"
)

// maxCueChars — độ dài tối đa mỗi cue phụ đề.
const maxCueChars = 80

// writeSRT sinh phụ đề SRT từ VoiceText các cảnh: timing cộng dồn theo thời
// lượng thực tế, mỗi cảnh tách câu thành cue ≤ 80 ký tự, chia thời gian theo
// tỉ lệ độ dài chữ.
func writeSRT(jobs []*sceneJob, dst string) error {
	var b strings.Builder
	idx := 1
	var t float64
	for _, j := range jobs {
		cues := splitCues(j.scene.VoiceText)
		if len(cues) == 0 {
			t += j.durS
			continue
		}
		total := 0
		for _, c := range cues {
			total += len([]rune(c))
		}
		start := t
		for _, c := range cues {
			end := start + j.durS*float64(len([]rune(c)))/float64(total)
			fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", idx, srtTime(start), srtTime(end), c)
			idx++
			start = end
		}
		t += j.durS
	}
	if err := os.WriteFile(dst, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("không ghi được file %s: %w", dst, err)
	}
	return nil
}

// srtTime định dạng giây → "HH:MM:SS,mmm".
func srtTime(s float64) string {
	if s < 0 {
		s = 0
	}
	ms := int(s*1000 + 0.5)
	return fmt.Sprintf("%02d:%02d:%02d,%03d",
		ms/3600000, ms%3600000/60000, ms%60000/1000, ms%1000)
}

// splitCues tách lời đọc thành các cue ≤ maxCueChars: tách câu trước,
// gộp/chia tiếp theo khoảng trắng nếu câu quá dài.
func splitCues(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var cues []string
	cur := ""
	for _, sentence := range splitSentences(text) {
		for _, part := range splitLong(sentence, maxCueChars) {
			switch {
			case cur == "":
				cur = part
			case len([]rune(cur))+1+len([]rune(part)) <= maxCueChars:
				cur += " " + part
			default:
				cues = append(cues, cur)
				cur = part
			}
		}
	}
	if cur != "" {
		cues = append(cues, cur)
	}
	return cues
}

// splitSentences tách văn bản thành câu theo dấu kết câu / xuống dòng.
func splitSentences(text string) []string {
	var out []string
	var cur strings.Builder
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			out = append(out, s)
		}
		cur.Reset()
	}
	for _, r := range text {
		cur.WriteRune(r)
		switch r {
		case '.', '!', '?', '…', ';', '\n':
			flush()
		}
	}
	flush()
	return out
}

// splitLong chia câu quá dài theo khoảng trắng, mỗi đoạn ≤ limit ký tự
// (từ đơn dài hơn limit được giữ nguyên).
func splitLong(s string, limit int) []string {
	if len([]rune(s)) <= limit {
		return []string{s}
	}
	var out []string
	cur := ""
	for _, w := range strings.Fields(s) {
		switch {
		case cur == "":
			cur = w
		case len([]rune(cur))+1+len([]rune(w)) <= limit:
			cur += " " + w
		default:
			out = append(out, cur)
			cur = w
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
