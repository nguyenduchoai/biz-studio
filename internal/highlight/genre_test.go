package highlight

import (
	"strings"
	"testing"
)

// Gu chấm điểm phải THẬT SỰ đi vào prompt. Nếu không, chọn thể loại chỉ là một
// ô select trang trí: người dùng tưởng đã dặn máy, mà máy vẫn chấm y như cũ.
func TestGenreChangesTheScoringPrompt(t *testing.T) {
	cs := []Candidate{{Index: 0, Start: 0, End: 8, Text: "thử"}}

	seen := map[string]bool{}
	for _, g := range Genres() {
		p := buildScorePrompt(cs, 60, "", g)
		if !strings.Contains(p, g.Name) {
			t.Errorf("%s: prompt không nhắc tên thể loại", g.ID)
		}
		if !strings.Contains(p, g.high) {
			t.Errorf("%s: prompt thiếu tiêu chí 9-10 điểm của thể loại", g.ID)
		}
		if !strings.Contains(p, g.low) {
			t.Errorf("%s: prompt thiếu tiêu chí hạ điểm của thể loại", g.ID)
		}
		if seen[p] {
			t.Errorf("%s: prompt trùng y hệt một thể loại khác — chọn thể loại thành vô nghĩa", g.ID)
		}
		seen[p] = true
	}
}

// Gõ sai hay để trống thì rơi về "auto", không được chết hay trả rỗng: người
// dùng cũ không có trường này trong dữ liệu đã lưu.
func TestFindGenreFallsBackToAuto(t *testing.T) {
	for _, id := range []string{"", "   ", "khong-co-that", "KIENTHUC-sai"} {
		if g := FindGenre(id); g.ID != "auto" {
			t.Errorf("FindGenre(%q) = %s, muốn auto", id, g.ID)
		}
	}
	if g := FindGenre("  KienThuc "); g.ID != "kienthuc" {
		t.Errorf("FindGenre không chịu được hoa/thường và khoảng trắng, ra %s", g.ID)
	}
}

func TestGenresWellFormed(t *testing.T) {
	ids := map[string]bool{}
	for _, g := range Genres() {
		if ids[g.ID] {
			t.Errorf("trùng ID %q", g.ID)
		}
		ids[g.ID] = true
		for name, v := range map[string]string{"Name": g.Name, "Desc": g.Desc, "high": g.high, "low": g.low} {
			if strings.TrimSpace(v) == "" {
				t.Errorf("%s: thiếu %s", g.ID, name)
			}
		}
	}
	if Genres()[0].ID != "auto" {
		t.Error("thể loại đầu tiên phải là auto — đó là mặc định của giao diện")
	}
}
