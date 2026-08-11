package vtemplate

import (
	"strings"
	"testing"

	"bizstudio/internal/store"
)

// Khuôn là thứ người dùng bấm một cái rồi tin theo. Một khuôn thiếu trường,
// trỏ vào nền tảng không tồn tại, hay ghi tone nhạc sai tên thì lỗi chỉ lộ ra
// lúc họ đã bấm — nên kiểm ở đây.
func TestTemplatesToanVen(t *testing.T) {
	ts := All()
	if len(ts) < 20 {
		t.Fatalf("có %d khuôn, quá ít để phủ các lĩnh vực thường gặp", len(ts))
	}
	seen := map[string]bool{}
	for _, tp := range ts {
		if seen[tp.ID] {
			t.Errorf("trùng id khuôn: %q", tp.ID)
		}
		seen[tp.ID] = true

		for _, f := range []struct{ name, val string }{
			{"ID", tp.ID}, {"Name", tp.Name}, {"Category", tp.Category},
			{"Desc", tp.Desc}, {"Icon", tp.Icon}, {"Script", tp.Script},
			{"Hook", tp.Hook}, {"Body", tp.Body}, {"CTA", tp.CTA},
			{"Style", tp.Style}, {"Aspect", tp.Aspect},
		} {
			if strings.TrimSpace(f.val) == "" {
				t.Errorf("khuôn %q thiếu %s", tp.ID, f.name)
			}
		}
		if tp.Seconds <= 0 || tp.Seconds > 600 {
			t.Errorf("khuôn %q có Seconds=%d, ngoài khoảng hợp lý", tp.ID, tp.Seconds)
		}
		// khuôn trỏ tới nền tảng nào thì nền tảng đó phải có thật
		if tp.Platform != "" {
			if _, ok := FindPlatform(tp.Platform); !ok {
				t.Errorf("khuôn %q trỏ tới nền tảng không tồn tại: %q", tp.ID, tp.Platform)
			}
		}
		// tone nhạc cũng vậy
		if tp.MusicMood != "" {
			if _, ok := FindMood(tp.MusicMood); !ok {
				t.Errorf("khuôn %q trỏ tới tone nhạc không tồn tại: %q", tp.ID, tp.MusicMood)
			}
		}
		if tp.Aspect != "9:16" && tp.Aspect != "16:9" && tp.Aspect != "1:1" {
			t.Errorf("khuôn %q có tỉ lệ lạ: %q", tp.ID, tp.Aspect)
		}
	}
	if len(Categories()) < 5 {
		t.Errorf("chỉ có %d danh mục", len(Categories()))
	}
	// mọi danh mục phải có ít nhất một khuôn, không thì hiện mục trống
	for _, c := range Categories() {
		n := 0
		for _, tp := range ts {
			if tp.Category == c {
				n++
			}
		}
		if n == 0 {
			t.Errorf("danh mục %q không có khuôn nào", c)
		}
	}
}

func TestFind(t *testing.T) {
	if _, ok := Find("khong-co-dau"); ok {
		t.Error("tìm id bịa mà vẫn thấy")
	}
	first := All()[0]
	got, ok := Find(strings.ToUpper(first.ID))
	if !ok || got.ID != first.ID {
		t.Errorf("tìm không phân biệt hoa thường thất bại với %q", first.ID)
	}
}

// Sửa bảng khuôn xong mà quên sao chép là mọi nơi dùng chung một mảng — người
// dùng đổi tên khuôn ở tab này, tab kia đổi theo.
func TestAllTraBanSao(t *testing.T) {
	a := All()
	orig := a[0].Name
	a[0].Name = "đã bị sửa"
	if All()[0].Name != orig {
		t.Error("All() trả thẳng mảng gốc, sửa được từ bên ngoài")
	}
}

func TestPlatforms(t *testing.T) {
	ps := Platforms()
	if len(ps) < 5 {
		t.Fatalf("chỉ có %d nền tảng", len(ps))
	}
	for _, p := range ps {
		if p.Width <= 0 || p.Height <= 0 {
			t.Errorf("nền tảng %q có kích thước %dx%d", p.ID, p.Width, p.Height)
		}
		// khung hình phải chẵn: h264 yuv420p không mã hoá được cạnh lẻ
		if p.Width%2 != 0 || p.Height%2 != 0 {
			t.Errorf("nền tảng %q có cạnh lẻ %dx%d, h264 sẽ không mã hoá được", p.ID, p.Width, p.Height)
		}
		if p.LUFS >= 0 || p.LUFS < -30 {
			t.Errorf("nền tảng %q có mốc độ to lạ: %v LUFS", p.ID, p.LUFS)
		}
		if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.Note) == "" {
			t.Errorf("nền tảng %q thiếu tên hoặc ghi chú", p.ID)
		}
	}
	if _, ok := FindPlatform("myspace"); ok {
		t.Error("tìm nền tảng bịa mà vẫn thấy")
	}
}

func TestMoodForScript(t *testing.T) {
	cases := []struct{ text, want string }{
		{"Câu chuyện kinh dị có thật ở nghĩa địa", "u-am"},
		{"Khuyến mãi lớn, mua ngay kẻo hết", "hao-hung"},
		{"Hướng dẫn dùng phần mềm cho người mới", "nhe-nhang"},
		{"Trận đánh lịch sử của vua Quang Trung", "hung-trang"},
		{"", ""},
		{"abc xyz không có từ khoá nào", ""},
	}
	for _, c := range cases {
		if got := MoodForScript(c.text); got != c.want {
			t.Errorf("MoodForScript(%q) = %q, muốn %q", c.text, got, c.want)
		}
	}
}

func TestMoodsToanVen(t *testing.T) {
	ms := Moods()
	if len(ms) < 5 {
		t.Fatalf("chỉ có %d tone nhạc", len(ms))
	}
	seen := map[string]bool{}
	for _, m := range ms {
		if seen[m.ID] {
			t.Errorf("trùng id tone: %q", m.ID)
		}
		seen[m.ID] = true
		if strings.TrimSpace(m.Name) == "" || strings.TrimSpace(m.Desc) == "" {
			t.Errorf("tone %q thiếu tên hoặc mô tả", m.ID)
		}
	}
	// mọi tone trong bảng từ khoá phải là tone có thật
	for id := range moodKeywords {
		if _, ok := FindMood(id); !ok {
			t.Errorf("bảng từ khoá trỏ tới tone không tồn tại: %q", id)
		}
	}
}

// Trường Style của khuôn là TÊN một bộ Style Kit trong danh sách mồi. Đổi tên
// bên đó mà quên bên này thì khuôn trỏ hụt, mà lỗi không hiện ra ở đâu cả —
// người dùng chỉ thấy gợi ý một bộ style không tồn tại. Ghim lại ở đây.
func TestStyleCuaKhuonCoThat(t *testing.T) {
	co := map[string]bool{}
	for _, name := range store.DefaultStyleKitNames() {
		// tên trong danh sách mồi có emoji đứng trước: "🎬 Điện ảnh chân thực"
		co[strings.TrimSpace(name)] = true
		if _, rest, ok := strings.Cut(name, " "); ok {
			co[strings.TrimSpace(rest)] = true
		}
	}
	for _, tp := range All() {
		if tp.Style == "" {
			continue
		}
		if !co[tp.Style] {
			t.Errorf("khuôn %q trỏ tới Style Kit không tồn tại: %q", tp.ID, tp.Style)
		}
	}
}

// Chữ đã cháy sẵn trong hình co theo khung khi chuẩn hoá. Người dùng burn phụ
// đề lúc render rồi mới chuẩn hoá là đường đi rất tự nhiên, và trước đây không
// có gì báo cho họ biết phụ đề đã teo lại.
//
// Số 0.316 dưới đây KHÔNG phải tính toán suông: đo trên video thật có phụ đề
// cháy sẵn, dải chữ chiếm 3.98% chiều cao khung 16:9, sau khi chuẩn hoá sang
// khung dọc TikTok còn 1.25% — tỉ lệ 0.314, khớp công thức trong sai số đo.
func TestChuChaySanCoTheoKhung(t *testing.T) {
	cases := []struct {
		ten            string
		fw, fh, tw, th int
		muon           float64
		phaiCanhBao    bool
	}{
		{"16:9 → dọc: teo hơn ba lần", 1920, 1080, 1080, 1920, 0.316, true},
		{"1:1 → dọc: teo gần một nửa", 1080, 1080, 1080, 1920, 0.562, true},
		{"dọc → dọc: không đổi", 1080, 1920, 1080, 1920, 1.000, false},
		// Dọc đặt vào khung ngang thì lấp trọn CHIỀU CAO, nên chữ giữ nguyên tỉ
		// lệ so với khung — ngược hẳn với chiều kia. Đây là lý do phải tính theo
		// chiều cao khung chứ không theo hệ số thu ảnh.
		{"dọc → ngang: không đổi", 1080, 1920, 1920, 1080, 1.000, false},
		{"phóng to lên: chữ to theo", 540, 960, 1080, 1920, 1.000, false},
	}
	for _, c := range cases {
		got := burnedTextScale(c.fw, c.fh, c.tw, c.th)
		if diff := got - c.muon; diff > 0.005 || diff < -0.005 {
			t.Errorf("%s: burnedTextScale = %.3f, muốn %.3f", c.ten, got, c.muon)
		}
		if canhBao := got < textReadableFloor; canhBao != c.phaiCanhBao {
			t.Errorf("%s: cảnh báo = %v, muốn %v (hệ số %.3f, ngưỡng %.2f)",
				c.ten, canhBao, c.phaiCanhBao, got, textReadableFloor)
		}
	}
	// đầu vào rác thì trả 0 chứ không chia cho không
	for _, c := range [][4]int{{0, 1080, 1080, 1920}, {1920, 0, 1080, 1920}, {1920, 1080, 0, 1920}, {1920, 1080, 1080, 0}} {
		if got := burnedTextScale(c[0], c[1], c[2], c[3]); got != 0 {
			t.Errorf("kích thước %v phải trả 0, nhận %.3f", c, got)
		}
	}
}
