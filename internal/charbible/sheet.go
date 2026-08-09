// Package charbible — dựng hồ sơ nhân vật đầy đủ: bản vẽ ba góc nhìn để giữ
// ngoại hình nhất quán, và ghép giọng đọc theo tính cách.
package charbible

import (
	"fmt"
	"strings"

	"bizstudio/internal/store"
)

// ---------- Bản vẽ ba góc nhìn (turnaround sheet) ----------
//
// Đây là tờ giấy tham chiếu của cả video: mọi cảnh sau đều nhìn vào đây mà vẽ,
// nên bố cục phải KHOÁ CỨNG, không để model tự bố trí.
//
// Ba điều bắt buộc viết vào prompt, mỗi điều chữa một kiểu hỏng đã biết:
//
//  1. Không có tên riêng nào trong prompt. Model thiên vị nặng với tên: gọi tên
//     là nó vẽ nhân vật nó đã học, đè lên mô tả của người dùng.
//  2. Ánh sáng chia theo VÙNG. Cả tờ đánh sáng có hướng thì ba hình chiếu không
//     tách nền và đo tỉ lệ được; cả tờ ánh sáng phẳng thì chân dung mất khối,
//     nhìn như hình minh hoạ.
//  3. Tỉ lệ là chỗ dễ vỡ nhất. Phải nói rõ ba hình chiếu bằng nhau, cùng đường
//     chân, và khi thiếu chỗ thì Ô CHI TIẾT nhường — không bao giờ bóp méo người.

// SheetPrompt dựng prompt vẽ bản ba góc nhìn cho một nhân vật.
// styleLine là câu mô tả phong cách của bộ Style Kit (có thể rỗng).
func SheetPrompt(c store.Character, styleLine string) string {
	look := strings.TrimSpace(c.Look)
	if c.Persona != nil && strings.TrimSpace(c.Persona.Appearance) != "" {
		look = strings.TrimSpace(c.Persona.Appearance)
	}
	if look == "" {
		return ""
	}

	var b strings.Builder
	b.WriteString("Character model sheet on ONE 16:9 landscape canvas, pure white background, even margins. ")
	b.WriteString("The subject: ")
	b.WriteString(look)
	if c.Persona != nil {
		if extra := personaTraits(c.Persona); extra != "" {
			b.WriteString(". ")
			b.WriteString(extra)
		}
	}
	b.WriteString(". ")

	b.WriteString("LAYOUT — three zones separated by thin hairline rules. ")
	b.WriteString("LEFT ZONE, about 34% of the canvas width: one bust portrait, head and shoulders, front facing, centred, both shoulders complete, bottom edge cut flat. This is the reference for the face. ")
	b.WriteString("RIGHT-TOP ZONE: three FULL-BODY views side by side — front, side, back — sharing one ground line. ")
	b.WriteString("RIGHT-BOTTOM ZONE: four to five small isolated close-up studies of key details, evenly spaced in a row, clearly smaller than the full-body views. ")

	b.WriteString("LIGHTING IN THE LEFT ZONE ONLY: soft directional key light from upper left with natural falloff, ambient occlusion under the chin, in the eye sockets and where collar meets neck, so the face has volume. ")
	b.WriteString("LIGHTING IN THE RIGHT ZONES: flat even orthographic lighting, no directional key, no cast shadows. ")

	b.WriteString("PROPORTIONS ARE CRITICAL: the three full-body views are equal in height, identical head-to-body ratio, correct limb length, both feet on the same ground line, clear space above the head and below the feet. ")
	b.WriteString("Never stretch or squash the figures to fit anything in — if space runs short the detail studies give way, not the figures. ")
	b.WriteString("ONE face only across the whole sheet: the three views match the bust portrait exactly — same features, same hair, same expression. ")
	b.WriteString("No text, no labels, no watermark, no arrows.")

	if s := strings.TrimSpace(styleLine); s != "" {
		b.WriteString(" Rendering style: ")
		b.WriteString(s)
		b.WriteString(".")
	}
	return b.String()
}

// SheetNegative — thứ cần tránh trên bản vẽ.
//
// Cố ý KHÔNG cấm "photorealistic": vừa muốn chân thực vừa cấm chân thực là tự
// mâu thuẫn, model sẽ ra kết quả lưng chừng. Thứ cần cấm là cái GIẢ.
const SheetNegative = "multiple different faces, inconsistent character, text, labels, watermark, " +
	"arrows, callout lines, cropped limbs, feet cut off, figures at different heights, " +
	"stretched or squashed anatomy, plastic waxy skin, over-smoothed airbrushed face, " +
	"perfectly symmetrical face, dead eyes without highlights, helmet-like hair, cluttered background"

// personaTraits rút các nét trong hồ sơ thành câu mô tả thêm cho model.
// KHÔNG lấy tên, không lấy động cơ/tuyến phát triển — model vẽ không dùng được.
func personaTraits(p *store.Persona) string {
	var parts []string
	if g := strings.TrimSpace(p.Gender); g != "" {
		parts = append(parts, g)
	}
	if a := strings.TrimSpace(p.AgeRange); a != "" {
		parts = append(parts, a)
	}
	if id := strings.TrimSpace(p.Identity); id != "" {
		parts = append(parts, id)
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf("Subject reference: %s", strings.Join(parts, ", "))
}
