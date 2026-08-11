package htmlvideo

import (
	"os"
	"strings"
	"testing"
)

func TestNormalizeTransitionVaReveal(t *testing.T) {
	for in, want := range map[string]string{
		"fade": "fade", "dip": "dip", "page": "page", "PAGE": "page",
		"": "none", "none": "none", "bịa": "none",
	} {
		if got := normalizeTransition(in); got != want {
			t.Errorf("normalizeTransition(%q) = %q, muốn %q", in, got, want)
		}
	}
	for in, want := range map[string]string{
		"draw": "draw", "DRAW": "draw", " draw ": "draw",
		"": "none", "none": "none", "bịa": "none",
	} {
		if got := normalizeReveal(in); got != want {
			t.Errorf("normalizeReveal(%q) = %q, muốn %q", in, got, want)
		}
	}
}

// Hai hiệu ứng mới đều phải nằm TRỌN trong thời lượng cảnh. Nếu chồng mờ sang
// cảnh kế thì mỗi mối nối ăn mất một khúc, video ngắn dần và hình lệch khỏi
// giọng đọc đã thu — lỗi chỉ lộ ra ở cuối video dài, rất khó truy.
func TestHieuUngNamTrongThoiLuongCanh(t *testing.T) {
	raw, err := os.ReadFile("templates/scene.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	// Cú lật phải bị chặn theo DUR, không phải một hằng số cứng
	if !strings.Contains(s, "Math.min(0.62, DUR * 0.3)") {
		t.Error("thời lượng cú lật không còn bị chặn theo thời lượng cảnh")
	}
	// Nét vẽ phải quét theo TIẾN ĐỘ cảnh (p = t/DUR), không theo giây tuyệt đối
	if !strings.Contains(s, "(p - st.at) / st.dur") {
		t.Error("hiệu ứng vẽ không còn quét theo tiến độ cảnh")
	}
}

// Lớp màu phải xong TRƯỚC khi cảnh hết, chừa thời gian cho người xem nhìn bức
// tranh đã hoàn chỉnh. Tô xong đúng lúc cắt cảnh thì coi như chưa hề thấy.
func TestLopMauXongTruocKhiHetCanh(t *testing.T) {
	raw, err := os.ReadFile("templates/scene.html")
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	i := strings.Index(s, "var STEPS = [")
	if i < 0 {
		t.Fatal("không tìm thấy bảng bước vẽ")
	}
	block := s[i : i+400]
	// bước cuối: at 0.44 + dur 0.34 = 0.78 → còn 22% cảnh để đứng yên
	if !strings.Contains(block, "at: 0.44, dur: 0.34") {
		t.Errorf("mốc lớp màu đã đổi — kiểm lại nó còn xong trước khi hết cảnh không:\n%s", block)
	}
}
