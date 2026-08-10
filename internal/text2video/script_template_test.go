package text2video

import (
	"strings"
	"testing"

	"bizstudio/internal/vtemplate"
)

// Khuôn nắn kịch bản qua đúng một đường: chuỗi prompt. Ghép hụt thì không có
// gì báo lỗi cả — AI vẫn trả về kịch bản hợp lệ, chỉ là không theo khuôn, và
// người dùng không có cách nào biết. Nên kiểm thẳng vào chuỗi prompt.
func TestPromptCoHuongDanKhuon(t *testing.T) {
	tpl, ok := vtemplate.Find("quang-cao-san-pham")
	if !ok {
		t.Fatal("không tìm thấy khuôn mồi để kiểm")
	}
	got := buildScriptPrompt("nội dung nguồn nào đó", 30, 0, tpl.ID)

	for _, phai := range []string{tpl.Script, tpl.Hook, tpl.Body, tpl.CTA} {
		if phai == "" {
			continue
		}
		if !strings.Contains(got, phai) {
			t.Errorf("prompt thiếu một phần của khuôn:\n  cần: %q", phai)
		}
	}
	// Phải nói rõ đây là hướng dẫn, không phải lời để đọc — thiếu câu này thì
	// model bê nguyên "Mở đầu bằng nỗi đau cụ thể" vào lời đọc.
	if !strings.Contains(got, "không đưa nguyên văn") {
		t.Error("prompt không dặn model tránh đọc nguyên văn lời hướng dẫn")
	}
	// Nguồn phải đứng SAU hướng dẫn: đặt trước thì phần hướng dẫn lọt vào giữa
	// nguồn và câu lệnh trả JSON, model dễ coi nó là nội dung cần kể.
	iKhuon, iNguon := strings.Index(got, tpl.Hook), strings.Index(got, "nội dung nguồn nào đó")
	if iKhuon < 0 || iNguon < 0 || iKhuon > iNguon {
		t.Errorf("thứ tự sai: hướng dẫn khuôn ở %d, nguồn ở %d — khuôn phải đứng trước", iKhuon, iNguon)
	}
}

// Không dùng khuôn thì prompt phải y hệt như trước khi có tính năng này —
// người đang dùng bình thường không được lãnh thêm chữ nào.
func TestKhongKhuonThiPromptKhongDoi(t *testing.T) {
	base := buildScriptPrompt("nguồn", 30, 0, "")
	for _, id := range []string{"", "   ", "khuon-khong-ton-tai"} {
		if got := buildScriptPrompt("nguồn", 30, 0, id); got != base {
			t.Errorf("id %q làm prompt đổi so với khi không có khuôn", id)
		}
	}
	if strings.Contains(base, "Khuôn kể chuyện") {
		t.Error("prompt không dùng khuôn mà vẫn có mục khuôn")
	}
}

// Mỗi khuôn phải ghép được, không sót khuôn nào — thêm khuôn mới mà quên điền
// một trường thì lỗi lộ ra ở đây chứ không phải ở kịch bản người dùng.
func TestMoiKhuonDeuGhepDuoc(t *testing.T) {
	for _, tpl := range vtemplate.All() {
		got := buildScriptPrompt("nguồn", 0, 0, tpl.ID)
		if !strings.Contains(got, "Khuôn kể chuyện") {
			t.Errorf("khuôn %q không ghép được vào prompt", tpl.ID)
			continue
		}
		if !strings.Contains(got, tpl.Hook) {
			t.Errorf("khuôn %q: prompt thiếu phần mở đầu", tpl.ID)
		}
	}
}
