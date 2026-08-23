package timeline

import (
	"fmt"
	"os"
	"strings"
)

// WriteSRT ghi lớp phụ đề ra file .srt để ffmpeg đọc.
//
// Dùng SRT chứ không truyền chữ thẳng vào drawtext: drawtext bắt phải thoát ký
// tự cho từng dòng, mà chữ tiếng Việt của người dùng đầy dấu nháy và dấu hai
// chấm — sai một chỗ là vỡ cả chuỗi lọc.
func WriteSRT(d *Doc, path string) error {
	if len(d.Subs) == 0 {
		return fmt.Errorf("không có dòng phụ đề nào")
	}
	var b strings.Builder
	for i, c := range d.Subs {
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n",
			i+1, srtTime(c.Start), srtTime(c.End), strings.TrimSpace(c.Text))
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("ghi file phụ đề: %w", err)
	}
	return nil
}

// srtTime đổi giây thành HH:MM:SS,mmm.
func srtTime(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	ms := int(sec*1000 + 0.5)
	h := ms / 3600000
	ms -= h * 3600000
	m := ms / 60000
	ms -= m * 60000
	s := ms / 1000
	ms -= s * 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}
