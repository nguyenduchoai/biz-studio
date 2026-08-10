package htmlvideo

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// Chuyện đã xảy ra: khuôn cảnh dùng ✓ ✦ ▍ ▶ làm dấu tích, ngôi sao, con trỏ và
// nút play. Font tiếng Việt đóng gói kèm (Be Vietnam Pro) chỉ có 459 glyph và
// KHÔNG có bốn ký tự đó — đọc thẳng bảng cmap của font ra thế. Nên chúng rơi
// sang font của hệ điều hành, tức lọt đúng khỏi cái mà việc đóng gói font sinh
// ra để tránh: hai máy render ra hai kiểu. Trên macOS nhìn vẫn ổn nên lỗi này
// không bao giờ tự lộ ra; chỉ máy Linux/Windows thiếu font mới thấy.
//
// Nay bốn thứ đó vẽ bằng hình học CSS. Test này canh không ai vô tình đưa ký tự
// lạ trở lại — rẻ hơn nhiều so với phát hiện qua một video đã xuất bản.
func TestKhongDungKyTuLaTrongKhuonCanh(t *testing.T) {
	raw, err := os.ReadFile("templates/scene.html")
	if err != nil {
		t.Fatalf("đọc khuôn cảnh: %v", err)
	}
	// Bỏ phần chú thích: chú thích có nhắc lại bốn ký tự để giải thích vì sao
	// không dùng chúng, đó là tài liệu chứ không phải thứ được render.
	src := stripComments(string(raw))

	// Dải ký hiệu KHÔNG được dùng. Đây là nơi ở của mũi tên, khung kẻ, khối
	// đặc, hình học và dingbat — font chữ hiếm khi có, và đúng bốn ký tự cũ
	// đều nằm trong đây.
	cam := []struct {
		lo, hi rune
		ten    string
	}{
		{0x2190, 0x21FF, "mũi tên"},
		{0x2200, 0x22FF, "ký hiệu toán"},
		{0x2500, 0x257F, "khung kẻ"},
		{0x2580, 0x259F, "khối đặc"},
		{0x25A0, 0x25FF, "hình học"},
		{0x2600, 0x27BF, "biểu tượng & dingbat"},
		{0x2B00, 0x2BFF, "mũi tên & hình bổ sung"},
		{0x1F300, 0x1FAFF, "emoji"},
	}
	for i, r := range src {
		for _, c := range cam {
			if r >= c.lo && r <= c.hi {
				t.Errorf("dòng có ký tự %q (U+%04X, nhóm %s) tại vị trí %d — "+
					"font đóng gói không chắc có glyph này, hãy vẽ bằng CSS thay vì dùng ký tự.\n  ngữ cảnh: %s",
					string(r), r, c.ten, i, quanh(src, i))
			}
		}
	}
}

// Bốn hình vẽ CSS thay cho bốn ký tự cũ phải còn nguyên. Xoá nhầm một cái thì
// dấu tích / nút play biến mất khỏi video mà không có lỗi nào báo.
func TestConDuHinhVeCSS(t *testing.T) {
	raw, err := os.ReadFile("templates/scene.html")
	if err != nil {
		t.Fatalf("đọc khuôn cảnh: %v", err)
	}
	s := string(raw)
	for _, c := range []struct{ sel, vaiTro string }{
		{".tick::after", "dấu tích của gạch đầu dòng"},
		{".core::after", "tam giác play của cảnh kết"},
		{".spark", "ngôi sao của huy hiệu hero"},
		{".caret", "con trỏ của cảnh code"},
	} {
		if !strings.Contains(s, c.sel) {
			t.Errorf("thiếu %s — %s sẽ không hiện", c.sel, c.vaiTro)
		}
	}
}

// stripComments bỏ chú thích /* */, <!-- --> và // để chỉ soi phần thật sự
// được render. Chú thích trong mã nguồn viết tiếng Việt và có dùng mũi tên
// làm dấu nối ý ("đen → rõ") — đó là chữ cho người đọc mã, không ra màn hình.
func stripComments(s string) string {
	for _, p := range [][2]string{{"/*", "*/"}, {"<!--", "-->"}} {
		var b strings.Builder
		rest := s
		for {
			i := strings.Index(rest, p[0])
			if i < 0 {
				b.WriteString(rest)
				break
			}
			b.WriteString(rest[:i])
			j := strings.Index(rest[i:], p[1])
			if j < 0 {
				break
			}
			rest = rest[i+j+len(p[1]):]
		}
		s = b.String()
	}
	// chú thích một dòng: bỏ "//" tới hết dòng, trừ "://" của địa chỉ web
	var out []string
	for _, ln := range strings.Split(s, "\n") {
		if i := strings.Index(ln, "//"); i >= 0 && (i == 0 || ln[i-1] != ':') {
			ln = ln[:i]
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

func quanh(s string, i int) string {
	lo, hi := i-40, i+40
	if lo < 0 {
		lo = 0
	}
	if hi > len(s) {
		hi = len(s)
	}
	return fmt.Sprintf("…%s…", strings.ReplaceAll(s[lo:hi], "\n", " "))
}
