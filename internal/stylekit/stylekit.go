// Package stylekit ghép bộ style hình ảnh đang chọn vào prompt sinh ảnh,
// để mọi cảnh trong một video trông như cùng một bộ phim.
package stylekit

import (
	"strings"

	"bizstudio/internal/store"
)

// Apply ghép style prompt + bảng màu vào nội dung cảnh.
// base là mô tả riêng của cảnh; trả nguyên base nếu chưa có bộ style nào.
func Apply(st *store.Store, base string) string {
	k, ok := st.ActiveStyleKit()
	if !ok {
		return base
	}
	return ApplyKit(k, base)
}

// ApplyKit ghép một bộ style cụ thể vào mô tả cảnh.
func ApplyKit(k store.StyleKit, base string) string {
	base = strings.TrimSpace(base)
	parts := make([]string, 0, 4)
	if base != "" {
		parts = append(parts, base)
	}
	if sp := strings.TrimSpace(k.StylePrompt); sp != "" {
		parts = append(parts, sp)
	}
	if pal := palettePhrase(k.Palette); pal != "" {
		parts = append(parts, pal)
	}
	if neg := strings.TrimSpace(k.Negative); neg != "" {
		parts = append(parts, "avoid: "+neg)
	}
	return strings.Join(parts, ". ")
}

// Theme trả theme khung chữ của bộ style đang chọn (rỗng → fallback của caller).
func Theme(st *store.Store) string {
	if k, ok := st.ActiveStyleKit(); ok {
		return strings.TrimSpace(k.Theme)
	}
	return ""
}

// palettePhrase mô tả bảng màu thành câu cho model sinh ảnh.
func palettePhrase(colors []string) string {
	clean := make([]string, 0, len(colors))
	for _, c := range colors {
		if c = strings.TrimSpace(c); c != "" {
			clean = append(clean, c)
		}
	}
	if len(clean) == 0 {
		return ""
	}
	return "color palette: " + strings.Join(clean, ", ")
}
