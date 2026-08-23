package timeline

import (
	"strings"
	"testing"
)

// Giao diện gửi lên đủ kiểu số lệch: kéo quá mép trái thành số âm, kéo hai mép
// qua nhau, fade dài hơn cả đoạn. Normalize là chỗ DUY NHẤT dọn, nên nó phải
// dọn đủ — rải phép kiểm khắp nơi là kiểu gì cũng sót một chỗ.
func TestNormalizeFixesBadInput(t *testing.T) {
	d := &Doc{
		Video: "v.mp4", VideoDur: 30,
		Tracks: []Track{{
			ID: "a1", Items: []Item{
				{Path: "a.wav", At: -5, In: -2, Out: 4},                // At và In âm
				{Path: "b.wav", At: 1, In: 5, Out: 3},                  // hai mép qua nhau → bỏ
				{Path: "", At: 2, Out: 3},                              // không có file → bỏ
				{Path: "c.wav", At: 10, Out: 2, FadeIn: 9, FadeOut: 9}, // fade dài hơn đoạn
				{Path: "d.wav", At: 3, Out: 1, Gain: 999},              // âm lượng vô lý
			},
		}},
		Subs: []Cue{
			{Start: 5, End: 3, Text: "ngược"}, // hết trước khi bắt đầu → bỏ
			{Start: -1, End: 2, Text: "âm"},   // bắt đầu âm → kéo về 0
			{Start: 8, End: 9, Text: "   "},   // rỗng → bỏ
			{Start: 1, End: 2, Text: "giữ lại"},
		},
	}
	d.Normalize()

	items := d.Tracks[0].Items
	if len(items) != 3 {
		t.Fatalf("giữ %d đoạn, muốn 3 (bỏ đoạn mép ngược và đoạn không có file)", len(items))
	}
	// Phải sắp theo vị trí trên timeline, không theo thứ tự người dùng thêm vào.
	for i := 1; i < len(items); i++ {
		if items[i].At < items[i-1].At {
			t.Errorf("đoạn không sắp theo thời gian: %.1f sau %.1f", items[i].At, items[i-1].At)
		}
	}
	if items[0].At != 0 || items[0].In != 0 {
		t.Errorf("At/In âm chưa được kéo về 0: At=%.1f In=%.1f", items[0].At, items[0].In)
	}
	for _, it := range items {
		if it.Gain > 12 || it.Gain < -60 {
			t.Errorf("âm lượng %.0f dB ngoài khoảng dùng được", it.Gain)
		}
		if half := it.Dur() / 2; it.FadeIn > half || it.FadeOut > half {
			t.Errorf("fade (%.1f/%.1f) dài hơn nửa đoạn %.1fs — ffmpeg cho ra đoạn câm mà không báo lỗi",
				it.FadeIn, it.FadeOut, it.Dur())
		}
	}

	if len(d.Subs) != 2 {
		t.Fatalf("giữ %d dòng phụ đề, muốn 2", len(d.Subs))
	}
	if d.Subs[0].Start < 0 {
		t.Error("mốc phụ đề âm chưa được kéo về 0")
	}
}

// Normalize gọi hai lần phải ra cùng kết quả; nếu không, mỗi lần lưu lại đổi
// dữ liệu một ít và timeline tự trôi.
func TestNormalizeIsIdempotent(t *testing.T) {
	mk := func() *Doc {
		return &Doc{Video: "v.mp4", VideoDur: 10, Tracks: []Track{{ID: "a",
			Items: []Item{{Path: "x.wav", At: -3, In: -1, Out: 5, FadeIn: 99, Gain: 50}}}}}
	}
	a, b := mk(), mk()
	a.Normalize()
	b.Normalize()
	b.Normalize()
	if a.Tracks[0].Items[0] != b.Tracks[0].Items[0] {
		t.Errorf("chuẩn hoá hai lần ra khác nhau:\n  1 lần: %+v\n  2 lần: %+v",
			a.Tracks[0].Items[0], b.Tracks[0].Items[0])
	}
}

func TestValidate(t *testing.T) {
	d := &Doc{}
	if err := d.Validate(); err == nil || !strings.Contains(err.Error(), "video nền") {
		t.Errorf("thiếu video nền phải báo lỗi rõ, nhận: %v", err)
	}

	big := &Doc{Video: "v.mp4", Tracks: []Track{{ID: "a"}}}
	for i := 0; i < maxItems+1; i++ {
		big.Tracks[0].Items = append(big.Tracks[0].Items, Item{Path: "x.wav", Out: 1})
	}
	if err := big.Validate(); err == nil {
		t.Error("quá trần số đoạn mà không báo — dòng lệnh ffmpeg sẽ vượt giới hạn hệ điều hành")
	}
}

// Độ dài timeline phải tính cả đoạn âm thanh vượt quá video, nếu không thanh
// thời gian trên giao diện cắt cụt mất phần cuối.
func TestDurCoversOverhangingLayers(t *testing.T) {
	d := &Doc{Video: "v.mp4", VideoDur: 10,
		Tracks: []Track{{ID: "a", Items: []Item{{Path: "x.wav", At: 8, Out: 6}}}},
		Subs:   []Cue{{Start: 1, End: 20, Text: "dài"}}}
	if got := d.Dur(); got != 20 {
		t.Errorf("Dur() = %.0f, muốn 20 (phụ đề kéo dài nhất)", got)
	}
}

func TestSrtTime(t *testing.T) {
	cases := map[float64]string{
		0:        "00:00:00,000",
		1.5:      "00:00:01,500",
		61.25:    "00:01:01,250",
		3661.007: "01:01:01,007",
		-3:       "00:00:00,000",
	}
	for in, want := range cases {
		if got := srtTime(in); got != want {
			t.Errorf("srtTime(%.3f) = %s, muốn %s", in, got, want)
		}
	}
}

// Đường dẫn người dùng có dấu cách, dấu nháy, dấu hai chấm là chuyện thường —
// mà cú pháp filter của ffmpeg dùng đúng những ký tự đó làm dấu phân cách.
func TestEscapeFilterPath(t *testing.T) {
	got := escapeFilterPath(`/Users/a b/Dự án: "test"/phụ'đề.srt`)
	for _, raw := range []string{`: `, `'`} {
		if strings.Contains(strings.ReplaceAll(got, `\`+raw, ""), raw) {
			t.Errorf("chưa thoát %q: %s", raw, got)
		}
	}
}

// Không có lời đọc thì chẳng có gì để né — nhạc phải trộn thẳng, không được
// dựng sidechaincompress với một đầu vào không tồn tại.
func TestNoDuckingWithoutNarration(t *testing.T) {
	d := &Doc{Video: "v.mp4", VideoDur: 10,
		Tracks: []Track{{ID: "m", Role: RoleMusic, Duck: true,
			Items: []Item{{Path: "m.wav", Out: 10}}}}}
	d.Normalize()
	plan, err := BuildPlan(d, "v.mp4", false, "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plan.Filter, "sidechaincompress") {
		t.Errorf("dựng né giọng dù không có lời đọc: %s", plan.Filter)
	}
}

func TestErrorWhenEverythingMuted(t *testing.T) {
	d := &Doc{Video: "v.mp4", VideoDur: 10,
		Tracks: []Track{{ID: "a", Role: RoleNarration, Mute: true,
			Items: []Item{{Path: "a.wav", Out: 3}}}}}
	d.Normalize()
	if _, err := BuildPlan(d, "v.mp4", false, ""); err == nil {
		t.Error("mọi lớp đều tắt mà vẫn dựng — ffmpeg sẽ lỗi với thông báo khó hiểu")
	}
}

// Đoán vai trò từ tên file. Sai thì người dùng sửa một cú bấm; đúng thì đỡ được
// bước hay bị quên nhất — bật "né lời đọc" cho nhạc nền.
func TestGuessRole(t *testing.T) {
	cases := map[string]string{
		"nhac-nen.mp3":        RoleMusic,
		"BGM_epic.wav":        RoleMusic,
		"background loop.m4a": RoleMusic,
		"Nhạc chill.mp3":      RoleMusic,
		"sfx-whoosh.wav":      RoleSFX,
		"click.wav":           RoleSFX,
		"loi-doc-canh-1.wav":  RoleNarration,
		"voice.mp3":           RoleNarration,
		"":                    RoleNarration,
	}
	for name, want := range cases {
		if got := GuessRole(name); got != want {
			t.Errorf("GuessRole(%q) = %s, muốn %s", name, got, want)
		}
	}
}
