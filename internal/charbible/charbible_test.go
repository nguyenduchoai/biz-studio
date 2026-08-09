package charbible

import (
	"strings"
	"testing"

	"bizstudio/internal/store"
	"bizstudio/internal/tts"
)

// Danh sách giọng giống thật: 14 giọng VieNeu 3 miền + giọng hệ điều hành +
// giọng nhân bản (hai loại sau KHÔNG được ghép tự động).
func voicesMau() []tts.Voice {
	return []tts.Voice{
		{ID: "Minh Đức", Gender: "Nam", Lang: "vi-VN · Bắc", Engine: "vieneu"},
		{ID: "Thái Sơn", Gender: "Nam", Lang: "vi-VN · Trung", Engine: "vieneu"},
		{ID: "Quang Sơn", Gender: "Nam", Lang: "vi-VN · Nam", Engine: "vieneu"},
		{ID: "Trúc Ly", Gender: "Nữ", Lang: "vi-VN · Bắc", Engine: "vieneu"},
		{ID: "Đoan Trang", Gender: "Nữ", Lang: "vi-VN · Nam", Engine: "vieneu"},
		{ID: "Alex", Gender: "Nam", Lang: "en-US", Engine: "say"},
		{ID: "clone:clv_1", Gender: "", Lang: "", Engine: "clone"},
	}
}

// Ghép đúng giới tính VÀ vùng miền.
func TestMatchVoiceGioiTinhVaVungMien(t *testing.T) {
	cases := []struct {
		gender, region, want string
	}{
		{"Nam", "Bắc", "Minh Đức"},
		{"Nam", "Nam", "Quang Sơn"},
		{"Nữ", "Nam", "Đoan Trang"},
		{"Nữ", "Bắc", "Trúc Ly"},
		{"male", "north", "Minh Đức"},
		{"female", "miền Nam", "Đoan Trang"},
		{"Nam", "Huế", "Thái Sơn"},
	}
	for _, c := range cases {
		got := MatchVoice(&store.Persona{Gender: c.gender, Region: c.region}, voicesMau())
		if got != c.want {
			t.Errorf("giới %q vùng %q → %q, cần %q", c.gender, c.region, got, c.want)
		}
	}
}

// "female" chứa chuỗi "male" — nếu xét nhánh nam trước thì nữ bị ghép thành nam,
// nghe là biết ngay. Bài này khoá đúng thứ tự xét.
func TestMatchVoiceFemaleKhongThanhMale(t *testing.T) {
	got := MatchVoice(&store.Persona{Gender: "female", Region: "Bắc"}, voicesMau())
	if got != "Trúc Ly" {
		t.Errorf("giới \"female\" phải ra giọng nữ, nhận %q", got)
	}
}

// Không rõ giới tính thì THÀ KHÔNG GHÉP còn hơn gán bừa giọng sai giới.
func TestMatchVoiceKhongDoanBua(t *testing.T) {
	if got := MatchVoice(&store.Persona{Region: "Bắc"}, voicesMau()); got != "" {
		t.Errorf("thiếu giới tính phải trả rỗng, nhận %q", got)
	}
	if got := MatchVoice(&store.Persona{Gender: "không rõ"}, voicesMau()); got != "" {
		t.Errorf("giới tính không đọc được phải trả rỗng, nhận %q", got)
	}
	if got := MatchVoice(nil, voicesMau()); got != "" {
		t.Errorf("hồ sơ nil phải trả rỗng, nhận %q", got)
	}
}

// Chỉ ghép giọng Việt on-device; giọng hệ điều hành và giọng nhân bản bị loại.
func TestMatchVoiceChiGhepVieNeu(t *testing.T) {
	chiSay := []tts.Voice{
		{ID: "Alex", Gender: "Nam", Lang: "en-US", Engine: "say"},
		{ID: "clone:x", Gender: "Nam", Lang: "", Engine: "clone"},
	}
	if got := MatchVoice(&store.Persona{Gender: "Nam", Region: "Bắc"}, chiSay); got != "" {
		t.Errorf("chỉ có giọng ngoài VieNeu thì phải trả rỗng, nhận %q", got)
	}
}

// Không khai vùng miền vẫn ghép được theo giới tính.
func TestMatchVoiceThieuVungMien(t *testing.T) {
	got := MatchVoice(&store.Persona{Gender: "Nữ"}, voicesMau())
	if got == "" {
		t.Fatal("có giới tính thì phải ghép được")
	}
	if got != "Trúc Ly" && got != "Đoan Trang" {
		t.Errorf("phải ra một giọng nữ, nhận %q", got)
	}
}

// ---------- bản vẽ ba góc nhìn ----------

// Prompt phải có đủ mọi mệnh lệnh khoá bố cục, và TUYỆT ĐỐI không có tên riêng.
func TestSheetPromptDuLenhVaKhongLoTen(t *testing.T) {
	c := store.Character{
		Name: "Elsa",
		Look: "an elderly ferryman with a hunched back and a white film over the left eye",
	}
	got := SheetPrompt(c, "semi-realistic painterly rendering")

	if strings.Contains(got, "Elsa") {
		t.Errorf("tên nhân vật lọt vào prompt bản vẽ: %q", got)
	}
	for _, must := range []string{
		"ONE 16:9 landscape canvas",
		"LEFT ZONE",
		"about 34%",
		"RIGHT-TOP ZONE",
		"RIGHT-BOTTOM ZONE",
		"thin hairline rules",
		"front, side, back",
		"LIGHTING IN THE LEFT ZONE ONLY",
		"LIGHTING IN THE RIGHT ZONES",
		"PROPORTIONS ARE CRITICAL",
		"the detail studies give way, not the figures",
		"ONE face only",
	} {
		if !strings.Contains(got, must) {
			t.Errorf("prompt thiếu mệnh lệnh %q", must)
		}
	}
	if !strings.Contains(got, c.Look) {
		t.Error("prompt mất mô tả ngoại hình")
	}
	if !strings.Contains(got, "semi-realistic painterly rendering") {
		t.Error("prompt mất phong cách của bộ Style Kit")
	}
}

// Có hồ sơ đầy đủ thì dùng Appearance (dài hơn) thay cho Look, và ghép thêm
// giới tính / tuổi / thân phận — nhưng KHÔNG ghép động cơ, tuyến phát triển.
func TestSheetPromptUuTienHoSoDayDu(t *testing.T) {
	c := store.Character{
		Name: "Lan",
		Look: "ngắn gọn",
		Persona: &store.Persona{
			Gender: "Nữ", AgeRange: "khoảng 30", Identity: "cô giáo làng",
			Appearance: "a woman with a long braid and a faded ao dai",
			Motivation: "tìm lại em trai thất lạc",
			Arc:        "từ rụt rè thành quyết đoán",
		},
	}
	got := SheetPrompt(c, "")
	if !strings.Contains(got, "a woman with a long braid") {
		t.Error("phải dùng Appearance khi có hồ sơ đầy đủ")
	}
	if strings.Contains(got, "ngắn gọn") {
		t.Error("không được dùng Look khi đã có Appearance")
	}
	if !strings.Contains(got, "cô giáo làng") {
		t.Error("thiếu thân phận trong prompt")
	}
	for _, khong := range []string{"tìm lại em trai", "rụt rè", "Lan"} {
		if strings.Contains(got, khong) {
			t.Errorf("prompt vẽ không được chứa %q", khong)
		}
	}
}

// Chưa tả ngoại hình thì không dựng được bản vẽ — trả rỗng để người gọi báo lỗi
// rõ ràng, thay vì gửi prompt trống cho model.
func TestSheetPromptChuaTaNgoaiHinh(t *testing.T) {
	if got := SheetPrompt(store.Character{Name: "X"}, "style"); got != "" {
		t.Errorf("chưa tả ngoại hình phải trả rỗng, nhận %q", got)
	}
}

// Negative KHÔNG được cấm photorealistic: vừa đòi chân thực vừa cấm chân thực
// là tự mâu thuẫn. Thứ cần cấm là cái GIẢ.
func TestSheetNegativeKhongTuMauThuan(t *testing.T) {
	low := strings.ToLower(SheetNegative)
	for _, cam := range []string{"photorealistic", "photo-realistic", "3d render"} {
		if strings.Contains(low, cam) {
			t.Errorf("negative không được chứa %q — tự mâu thuẫn với yêu cầu chân thực", cam)
		}
	}
	for _, can := range []string{"multiple different faces", "plastic waxy skin", "stretched or squashed"} {
		if !strings.Contains(low, strings.ToLower(can)) {
			t.Errorf("negative thiếu %q", can)
		}
	}
}

// ---------- prompt giọng ----------

func TestVoicePrompt(t *testing.T) {
	p := &store.Persona{Gender: "Nam", AgeRange: "khoảng bảy mươi"}
	v := &store.VoiceSpec{
		Timbre: "hoarse low bass-baritone", Pitch: "low",
		Pace: "slow with long pauses", Accent: "southern delta", Emotion: "weary but calm",
	}
	got := VoicePrompt(p, v)
	for _, must := range []string{"A male voice", "khoảng bảy mươi", "hoarse low bass-baritone", "southern delta", "weary but calm"} {
		if !strings.Contains(got, must) {
			t.Errorf("prompt giọng thiếu %q: %q", must, got)
		}
	}
	if VoicePrompt(p, nil) != "" {
		t.Error("không có thiết kế giọng phải trả rỗng")
	}
}

// Chốt chặn cuối: dù AI có quên luật mà nhét tên vào prompt ảnh, mã vẫn phải gỡ.
func TestStripNames(t *testing.T) {
	cases := []struct{ prompt, name, khong string }{
		{"Portrait of Elsa, a woman in a blue dress", "Elsa", "Elsa"},
		{"Elsa's face lit from the left", "Elsa", "Elsa"},
		{"A portrait of Chị Lan wearing ao dai", "Chị Lan", "Lan"},
		{"Son Tung stands in the rain", "Son Tung", "Tung"},
	}
	for _, c := range cases {
		got := StripNames(c.prompt, c.name)
		if strings.Contains(got, c.khong) {
			t.Errorf("còn sót %q sau khi gỡ: %q", c.khong, got)
		}
		if strings.TrimSpace(got) == "" {
			t.Errorf("gỡ tên xong không được rỗng: %q → %q", c.prompt, got)
		}
	}
}

// Luật cấm tên phải nằm nguyên trong prompt gửi cho AI.
func TestLuatPromptAnhCoDuY(t *testing.T) {
	for _, must := range []string{"KHÔNG viết tên nhân vật", "MÔ TẢ con người"} {
		if !strings.Contains(LuatPromptAnh, must) {
			t.Errorf("luật thiếu ý %q", must)
		}
	}
}
