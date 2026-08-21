package server

import (
	"strings"
	"testing"
)

// Mọi nguyên âm có dấu phải về đúng nguyên âm gốc. Bản đầu tra theo vị trí
// trong hai chuỗi song song viết tay và lệch một ký tự — "đ" ra "y", tên file
// hợp tuyển sai mà nhìn mã không thấy gì bất thường.
func TestDeaccentCoversVietnamese(t *testing.T) {
	groups := map[rune]string{
		'a': "àáạảãâầấậẩẫăằắặẳẵ", 'e': "èéẹẻẽêềếệểễ", 'i': "ìíịỉĩ",
		'o': "òóọỏõôồốộổỗơờớợởỡ", 'u': "ùúụủũưừứựửữ", 'y': "ỳýỵỷỹ", 'd': "đ",
	}
	n := 0
	for want, chars := range groups {
		for _, c := range chars {
			n++
			if got := deaccent(c); got != string(want) {
				t.Errorf("deaccent(%q) = %q, muốn %q", c, got, string(want))
			}
		}
	}
	if n < 60 {
		t.Errorf("chỉ phủ %d chữ có dấu — thiếu so với bảng chữ tiếng Việt", n)
	}
	// Chữ không dấu và ký tự khác giữ nguyên.
	for _, c := range []rune{'a', 'z', '0', '-', ' '} {
		if got := deaccent(c); got != string(c) {
			t.Errorf("deaccent(%q) = %q, phải giữ nguyên", c, got)
		}
	}
}

func TestSlugTitle(t *testing.T) {
	cases := map[string]string{
		"Chuyện khởi nghiệp thất bại":    "chuyen-khoi-nghiep-that-bai",
		"  Sai lầm tuyển người  ":        "sai-lam-tuyen-nguoi",
		"Đừng làm thế!!! (bài học 2026)": "dung-lam-the-bai-hoc-2026",
		"":                   "hop-tuyen",
		"???":                "hop-tuyen",
		"ĐẦU TƯ & TÀI CHÍNH": "dau-tu-tai-chinh",
	}
	for in, want := range cases {
		if got := slugTitle(in); got != want {
			t.Errorf("slugTitle(%q) = %q, muốn %q", in, got, want)
		}
	}
}

// Tên file dài quá thì một số hệ thống tệp từ chối; cắt rồi không được để lại
// dấu gạch cụt ở cuối.
func TestSlugTitleCapsLength(t *testing.T) {
	got := slugTitle(strings.Repeat("chuyện dài ", 20))
	if len(got) > 40 {
		t.Errorf("slug dài %d ký tự, quá trần 40: %q", len(got), got)
	}
	if strings.HasPrefix(got, "-") || strings.HasSuffix(got, "-") {
		t.Errorf("slug còn gạch thừa ở đầu/cuối: %q", got)
	}
}
