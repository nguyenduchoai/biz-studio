package whisper

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"bizstudio/internal/store"
	"bizstudio/internal/util"
)

// SRT xuất phụ đề .srt theo từng segment (mốc câu).
func SRT(tr *Transcript) string {
	if tr == nil {
		return ""
	}
	var b strings.Builder
	n := 0
	for _, s := range tr.Segments {
		text := strings.TrimSpace(s.Text)
		if text == "" {
			continue
		}
		end := s.End
		if end <= s.Start {
			end = s.Start + 0.5
		}
		n++
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", n, srtTime(s.Start), srtTime(end), text)
	}
	return b.String()
}

// srtTime định dạng HH:MM:SS,mmm.
func srtTime(t float64) string {
	if t < 0 {
		t = 0
	}
	ms := int64(math.Round(t * 1000))
	h := ms / 3600000
	ms -= h * 3600000
	m := ms / 60000
	ms -= m * 60000
	s := ms / 1000
	ms -= s * 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

// assTime định dạng H:MM:SS.cc (centisecond) cho ASS.
func assTime(t float64) string {
	if t < 0 {
		t = 0
	}
	cs := int64(math.Round(t * 100))
	h := cs / 360000
	cs -= h * 360000
	m := cs / 6000
	cs -= m * 6000
	s := cs / 100
	cs -= s * 100
	return fmt.Sprintf("%d:%02d:%02d.%02d", h, m, s, cs)
}

// KaraokeStyle — kiểu chữ phụ đề karaoke (lấy từ Style Kit đang dùng).
type KaraokeStyle struct {
	FontName  string // tên font hệ thống; rỗng → "Arial Unicode MS" (đủ dấu tiếng Việt)
	FontSize  int    // 0 → tự tính theo chiều cao khung
	Primary   string // hex — màu chữ khi CHƯA đọc tới
	Highlight string // hex — màu chữ khi ĐANG đọc tới
	Outline   string // hex — màu viền
	MarginV   int    // lề dưới (px theo PlayRes); 0 → tự tính

	// Khung hình tham chiếu — nên đặt đúng kích thước video sẽ burn để chữ
	// không bị co kéo. 0 → 1080×1920 (khung mặc định của app).
	PlayResX int
	PlayResY int
}

// fontFallback — font chắc chắn có đủ dấu tiếng Việt trên macOS.
const fontFallback = "Arial Unicode MS"

// KaraokeASS xuất phụ đề ASS có hiệu ứng \k tô sáng TỪNG TỪ.
// Segment nào không có mốc từng từ thì hiện nguyên câu (không có hiệu ứng).
func KaraokeASS(tr *Transcript, style KaraokeStyle) string {
	if tr == nil {
		return ""
	}
	st := style.normalize()
	var b strings.Builder
	writeASSHeader(&b, st)
	for _, seg := range tr.Segments {
		text := strings.TrimSpace(seg.Text)
		if text == "" && len(seg.Words) == 0 {
			continue
		}
		end := seg.End
		if end <= seg.Start {
			end = seg.Start + 0.5
		}
		fmt.Fprintf(&b, "Dialogue: 0,%s,%s,Karaoke,,0,0,0,,%s\n",
			assTime(seg.Start), assTime(end), karaokeLine(seg))
	}
	return b.String()
}

// normalize bù giá trị mặc định, đặc biệt là font đủ dấu tiếng Việt.
func (s KaraokeStyle) normalize() KaraokeStyle {
	if s.PlayResX <= 0 || s.PlayResY <= 0 {
		s.PlayResX, s.PlayResY = 1080, 1920
	}
	s.FontName = resolveFont(s.FontName)
	if s.FontSize <= 0 {
		s.FontSize = s.PlayResY / 24
		if s.FontSize < 24 {
			s.FontSize = 24
		}
	}
	if s.MarginV <= 0 {
		s.MarginV = s.PlayResY / 12
	}
	if strings.TrimSpace(s.Primary) == "" {
		s.Primary = "#FFFFFF"
	}
	if strings.TrimSpace(s.Highlight) == "" {
		s.Highlight = "#FFD400"
	}
	if strings.TrimSpace(s.Outline) == "" {
		s.Outline = "#000000"
	}
	return s
}

func writeASSHeader(b *strings.Builder, s KaraokeStyle) {
	outline := s.FontSize / 16
	if outline < 2 {
		outline = 2
	}
	margin := s.PlayResX / 18
	fmt.Fprintf(b, `[Script Info]
; Phụ đề karaoke sinh bởi Biz Studio
ScriptType: v4.00+
WrapStyle: 0
ScaledBorderAndShadow: yes
YCbCr Matrix: TV.709
PlayResX: %d
PlayResY: %d

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Karaoke,%s,%d,%s,%s,%s,&H80000000,-1,0,0,0,100,100,0,0,1,%d,1,2,%d,%d,%d,1

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
`,
		s.PlayResX, s.PlayResY,
		s.FontName, s.FontSize,
		assColor(s.Highlight), // PrimaryColour: màu sau khi \k quét qua (đang/đã đọc)
		assColor(s.Primary),   // SecondaryColour: màu trước khi đọc tới
		assColor(s.Outline),
		outline, margin, margin, s.MarginV)
}

// karaokeLine dựng chuỗi {\k..} cho một segment; \k tính bằng centisecond.
func karaokeLine(seg Segment) string {
	if len(seg.Words) == 0 {
		return assEscape(strings.TrimSpace(seg.Text))
	}
	var b strings.Builder
	cursor := seg.Start
	for i, w := range seg.Words {
		text := assEscape(strings.TrimSpace(w.Text))
		if text == "" {
			continue
		}
		if gap := w.Start - cursor; gap >= 0.02 {
			fmt.Fprintf(&b, `{\k%d}`, centi(gap)) // khoảng nghỉ trước từ
		}
		dur := w.End - w.Start
		if dur <= 0 {
			dur = 0.08
		}
		if i < len(seg.Words)-1 {
			text += " " // dấu cách thuộc về từ vừa đọc
		}
		fmt.Fprintf(&b, `{\k%d}%s`, centi(dur), text)
		cursor = w.End
	}
	if b.Len() == 0 {
		return assEscape(strings.TrimSpace(seg.Text))
	}
	return b.String()
}

// centi đổi giây → centisecond (tối thiểu 1 để không mất syllable).
func centi(sec float64) int {
	v := int(math.Round(sec * 100))
	if v < 1 {
		v = 1
	}
	return v
}

// assEscape thoát ký tự đặc biệt của ASS ({ } \ và xuống dòng).
func assEscape(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, "{", `\{`)
	v = strings.ReplaceAll(v, "}", `\}`)
	v = strings.ReplaceAll(v, "\r\n", `\N`)
	v = strings.ReplaceAll(v, "\n", `\N`)
	return v
}

// assColor đổi "#RRGGBB" → "&H00BBGGRR" (ASS đảo thứ tự byte màu).
func assColor(hex string) string {
	h := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(hex), "#"))
	if len(h) == 3 { // #abc → #aabbcc
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if len(h) != 6 {
		return "&H00FFFFFF"
	}
	h = strings.ToUpper(h)
	return "&H00" + h[4:6] + h[2:4] + h[0:2]
}

// StyleFromKit dựng KaraokeStyle từ bộ Style Kit đang dùng (font, màu chữ, màu
// nhấn). w/h là kích thước video sẽ burn (0 → khung mặc định 1080×1920).
func StyleFromKit(st *store.Store, w, h int) KaraokeStyle {
	s := KaraokeStyle{PlayResX: w, PlayResY: h}
	if st == nil {
		return s.normalize()
	}
	k, ok := st.ActiveStyleKit()
	if !ok {
		return s.normalize()
	}
	s.FontName = firstFontFamily(k.FontBody)
	s.Primary = k.TextMain
	s.Highlight = k.Accent
	return s.normalize()
}

var (
	fontCacheMu sync.Mutex
	fontCache   = map[string]bool{}
)

// resolveFont chọn font THẬT có trên máy: font của Style Kit nếu hệ thống có,
// ngược lại "Arial Unicode MS" — font đủ dấu tiếng Việt.
func resolveFont(name string) string {
	name = strings.TrimSpace(name)
	if name != "" && fontInstalled(name) {
		return name
	}
	if fontInstalled(fontFallback) {
		return fontFallback
	}
	if name != "" {
		return name // không kiểm tra được → để trình render tự thay thế
	}
	return fontFallback
}

// fontInstalled hỏi fontconfig (fc-match) xem máy có đúng font này không.
// Không có fc-match → trả false (caller giữ nguyên tên font).
func fontInstalled(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	fontCacheMu.Lock()
	defer fontCacheMu.Unlock()
	if v, ok := fontCache[name]; ok {
		return v
	}
	ok := false
	if util.Exists("fc-match") {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if out, err := util.Run(ctx, "fc-match", "--format=%{family}", name); err == nil {
			for _, fam := range strings.Split(out, ",") {
				if strings.EqualFold(strings.TrimSpace(fam), name) {
					ok = true
					break
				}
			}
		}
	}
	fontCache[name] = ok
	return ok
}

// firstFontFamily lấy font đầu tiên dùng được trong CSS font stack.
// Bỏ qua các họ chung (sans-serif, -apple-system…) vì trình render phụ đề cần
// TÊN FONT thật; không tìm được thì trả rỗng để dùng font fallback.
func firstFontFamily(stack string) string {
	for _, part := range strings.Split(stack, ",") {
		name := strings.TrimSpace(part)
		name = strings.Trim(name, `"'`)
		if name == "" {
			continue
		}
		switch strings.ToLower(name) {
		case "sans-serif", "serif", "monospace", "cursive", "fantasy",
			"system-ui", "ui-monospace", "ui-sans-serif", "ui-serif",
			"-apple-system", "blinkmacsystemfont":
			continue
		}
		return name
	}
	return ""
}
