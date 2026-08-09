package charbible

import (
	"strings"

	"bizstudio/internal/store"
	"bizstudio/internal/tts"
)

// ---------- Ghép giọng theo hồ sơ nhân vật ----------
//
// Người dùng đã tả nhân vật là "ông lão bảy mươi, giọng khàn, người miền Nam"
// thì không có lý gì bắt họ tự dò trong hàng chục giọng. Ghép ở đây là phép
// tính THUẦN, không gọi AI: giới tính và vùng miền là hai thứ nghe ra ngay,
// sai là hỏng cả video — nên để luật rõ ràng, kiểm chứng được.

// MatchVoice chọn giọng hợp nhất trong danh sách cho một hồ sơ nhân vật.
// Trả rỗng khi không có giọng nào đủ tin cậy — thà để người dùng tự chọn còn
// hơn gán bừa một giọng sai giới tính.
//
// regionOK cho biết có ghép ĐÚNG vùng miền không. Kho giọng thật hiện không có
// giọng miền Nam nào, nên nhân vật miền Nam sẽ nhận giọng miền khác — người gọi
// PHẢI nói ra điều đó, không được lặng lẽ thay giọng rồi coi như xong.
func MatchVoice(p *store.Persona, voices []tts.Voice) (id string, regionOK bool) {
	if p == nil || len(voices) == 0 {
		return "", false
	}
	wantMale, knowGender := genderOf(p.Gender)
	if !knowGender {
		return "", false
	}
	wantRegion := regionOf(p.Region)

	best, bestScore, bestRegion := "", -1, false
	for _, v := range voices {
		// Chỉ ghép giọng Việt on-device: giọng hệ điều hành và giọng nhân bản
		// không có thuộc tính đủ tin để ghép tự động.
		if !strings.EqualFold(v.Engine, "vieneu") {
			continue
		}
		isMale, ok := genderOf(v.Gender)
		if !ok || isMale != wantMale {
			continue
		}
		score, hitRegion := 1, false
		if wantRegion != "" && strings.Contains(v.Lang, wantRegion) {
			score, hitRegion = score+2, true
		}
		if score > bestScore {
			best, bestScore, bestRegion = v.ID, score, hitRegion
		}
	}
	// Không khai vùng miền thì coi như không có gì để lệch.
	if wantRegion == "" {
		bestRegion = true
	}
	return best, bestRegion
}

// genderOf đọc giới tính từ chữ tiếng Việt hoặc tiếng Anh.
// Trả (làNam, đọcĐược) — không đoán được thì đừng ghép.
func genderOf(v string) (bool, bool) {
	s := strings.ToLower(strings.TrimSpace(v))
	switch {
	case s == "":
		return false, false
	case strings.Contains(s, "nữ"), strings.Contains(s, "female"), strings.Contains(s, "woman"), strings.Contains(s, "girl"):
		return false, true
	case strings.Contains(s, "nam"), strings.Contains(s, "male"), strings.Contains(s, "man"), strings.Contains(s, "boy"):
		// "nam" nằm trong "nữ"? không. Nhưng "female" chứa "male" nên nhánh nữ
		// phải xét TRƯỚC nhánh nam — thứ tự case ở đây là cố ý.
		return true, true
	}
	return false, false
}

// regionOf chuẩn hoá vùng miền về đúng chữ dùng trong danh sách giọng.
func regionOf(v string) string {
	s := strings.ToLower(strings.TrimSpace(v))
	switch {
	case strings.Contains(s, "bắc"), strings.Contains(s, "bac"), strings.Contains(s, "north"), strings.Contains(s, "hà nội"):
		return "Bắc"
	case strings.Contains(s, "trung"), strings.Contains(s, "huế"), strings.Contains(s, "central"):
		return "Trung"
	case strings.Contains(s, "nam"), strings.Contains(s, "sài gòn"), strings.Contains(s, "south"):
		return "Nam"
	}
	return ""
}

// VoicePrompt dựng câu mô tả giọng bằng TIẾNG ANH cho engine thiết kế giọng.
// Mô tả CÂY ĐÀN chứ không phải một câu thoại cụ thể: giới tính, tuổi nghe ra,
// âm sắc, cao độ, nhịp, giọng vùng, cảm xúc mặc định.
func VoicePrompt(p *store.Persona, v *store.VoiceSpec) string {
	if v == nil {
		return ""
	}
	var parts []string
	if p != nil {
		if g := strings.TrimSpace(p.Gender); g != "" {
			if male, ok := genderOf(g); ok {
				if male {
					parts = append(parts, "A male voice")
				} else {
					parts = append(parts, "A female voice")
				}
			}
		}
		if a := strings.TrimSpace(p.AgeRange); a != "" {
			parts = append(parts, "sounding around "+a)
		}
	}
	add := func(label, val string) {
		if s := strings.TrimSpace(val); s != "" {
			parts = append(parts, label+" "+s)
		}
	}
	add("timbre:", v.Timbre)
	add("pitch range:", v.Pitch)
	add("pace:", v.Pace)
	add("accent:", v.Accent)
	add("default emotional colour:", v.Emotion)
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ") + "."
}
