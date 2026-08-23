package consent

import (
	"strings"
	"testing"
)

// Thiếu ô nào phải nói RÕ ô đó. Gộp thành "chưa xác nhận" thì người bấm nhầm
// một ô phải dò lại cả ba.
func TestCheckNamesEachMissingBox(t *testing.T) {
	cases := []struct {
		name string
		g    Grant
		want []string
	}{
		{"thiếu quyền dùng", Grant{Adult: true, Permitted: true}, []string{"quyền sử dụng"}},
		{"thiếu đủ tuổi", Grant{Rights: true, Permitted: true}, []string{"18 tuổi"}},
		{"thiếu đồng ý", Grant{Rights: true, Adult: true}, []string{"đồng ý"}},
		{"thiếu cả ba", Grant{}, []string{"quyền sử dụng", "18 tuổi", "đồng ý"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := Check(c.g, KindVoice)
			if err == nil {
				t.Fatal("thiếu xác nhận mà vẫn cho qua")
			}
			for _, w := range c.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("thông báo không nhắc %q: %v", w, err)
				}
			}
		})
	}
}

func TestCheckPassesWhenComplete(t *testing.T) {
	g := Grant{Rights: true, Adult: true, Permitted: true}
	for _, k := range []Kind{KindVoice, KindFace} {
		if err := Check(g, k); err != nil {
			t.Errorf("đủ ba xác nhận mà vẫn chặn (%s): %v", k, err)
		}
	}
}

// Thông báo phải nói đúng là đang xin phép cho MẶT hay cho GIỌNG — người dùng
// đang ở hai màn hình khác nhau.
func TestCheckMessageMatchesKind(t *testing.T) {
	if err := Check(Grant{}, KindFace); !strings.Contains(err.Error(), "khuôn mặt") {
		t.Errorf("xin phép khuôn mặt mà thông báo không nhắc: %v", err)
	}
	if err := Check(Grant{}, KindVoice); !strings.Contains(err.Error(), "giọng") {
		t.Errorf("xin phép giọng mà thông báo không nhắc: %v", err)
	}
}

// Dòng nhật ký là bằng chứng — không được rỗng, và phải chịu được trường hợp
// người dùng không ghi tên đối tượng.
func TestLine(t *testing.T) {
	g := Grant{Rights: true, Adult: true, Permitted: true, Subject: "Chị Lan (đã ký giấy)"}
	line := Line(g, KindVoice)
	if !strings.Contains(line, "Chị Lan") {
		t.Errorf("dòng nhật ký mất tên đối tượng: %s", line)
	}
	if !strings.Contains(line, "giọng") {
		t.Errorf("dòng nhật ký không nói loại: %s", line)
	}
	blank := Line(Grant{Rights: true, Adult: true, Permitted: true}, KindFace)
	if strings.TrimSpace(blank) == "" || !strings.Contains(blank, "khuôn mặt") {
		t.Errorf("không ghi tên thì vẫn phải có dòng đọc được: %q", blank)
	}
}
