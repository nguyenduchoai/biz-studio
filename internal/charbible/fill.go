package charbible

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"bizstudio/internal/gemini"
	"bizstudio/internal/store"
	"bizstudio/internal/tts"
)

// ---------- AI dựng hồ sơ đầy đủ từ mô tả ngắn ----------
//
// Chia việc theo NGÔN NGỮ, không phải theo trường:
//   - chữ cho NGƯỜI đọc  → tiếng Việt (hồ sơ, tính cách, khí chất…)
//   - prompt cho MÁY ăn  → luôn tiếng Anh (model ảnh và engine giọng ăn tiếng
//     Anh ổn định nhất, không liên quan giao diện hiển thị ngôn ngữ gì)

const fillSystem = `Bạn là biên kịch dựng hồ sơ nhân vật cho một xưởng phim hoạt hình.
Chỉ trả về DUY NHẤT một object JSON, không giải thích, không rào markdown.`

// khuonJSON — khuôn trả lời, liệt kê đúng tên MỌI khoá.
//
// Phải đưa khuôn đầy đủ chứ không mô tả suông: đo thật thấy khi chỉ viết
// {"persona":{...}} thì model tự nghĩ ra tên khoá của nó (age thay ageRange,
// role thay identity) và bỏ trống quá nửa số trường.
const khuonJSON = `{
  "persona": {
    "gender": "Nam hoặc Nữ",
    "ageRange": "khoảng bao nhiêu tuổi",
    "identity": "thân phận, nghề nghiệp",
    "appearance": "ngoại hình đầy đủ, chi tiết hơn mô tả gốc",
    "personality": ["từ 1", "từ 2", "từ 3"],
    "temperament": "khí chất, cách ăn nói",
    "motivation": "điều nhân vật muốn",
    "arc": "tuyến phát triển",
    "region": "Bắc hoặc Trung hoặc Nam"
  },
  "voice": {
    "timbre": "âm sắc",
    "pitch": "cao độ",
    "pace": "nhịp nói",
    "accent": "giọng vùng miền",
    "emotion": "cảm xúc mặc định"
  },
  "imagePrompt": "một câu tiếng Anh tả chân dung"
}`

// LuatPromptAnh — luật viết prompt ảnh, tách riêng để bài kiểm tra soi được.
const LuatPromptAnh = `TUYỆT ĐỐI KHÔNG viết tên nhân vật, biệt danh, tên tác giả hay tên tác phẩm vào imagePrompt.
Model sinh ảnh thiên vị rất nặng với tên riêng: gọi tên là nó vẽ nhân vật nó đã học chứ không vẽ nhân vật này.
Hãy MÔ TẢ con người, đừng gọi tên.`

// Đọc câu trả lời của AI theo kiểu KHOAN DUNG, không dùng struct cứng: đo thật
// thấy model trả "personality" là CHUỖI thay vì mảng, và đặt tên trường khác
// (age thay ageRange, role thay identity). Bắt bẻ đúng schema thì hỏng cả lần
// gọi vì một dấu ngoặc — trong khi dữ liệu vẫn dùng được.

// pickStr lấy giá trị chuỗi đầu tiên tìm được trong các khoá có thể có.
func pickStr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

// pickList lấy danh sách, chấp nhận cả mảng lẫn một chuỗi ngăn bằng dấu phẩy.
func pickList(m map[string]any, keys ...string) []string {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch t := v.(type) {
		case []any:
			out := make([]string, 0, len(t))
			for _, it := range t {
				if s, ok := it.(string); ok && strings.TrimSpace(s) != "" {
					out = append(out, strings.TrimSpace(s))
				}
			}
			if len(out) > 0 {
				return out
			}
		case string:
			var out []string
			for _, part := range strings.Split(t, ",") {
				if p := strings.TrimSpace(part); p != "" {
					out = append(out, p)
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	return nil
}

// sub lấy object con; không có thì trả map rỗng để người gọi khỏi kiểm nil.
func sub(m map[string]any, key string) map[string]any {
	if v, ok := m[key]; ok {
		if o, ok := v.(map[string]any); ok {
			return o
		}
	}
	return map[string]any{}
}

// Fill nhờ AI dựng hồ sơ đầy đủ cho nhân vật từ tên/ngoại hình/vai trò đang có,
// rồi ghép luôn giọng đọc trong máy. Ghi thẳng vào c.
func Fill(ctx context.Context, st *store.Store, c *store.Character) error {
	if c == nil {
		return errors.New("thiếu nhân vật")
	}
	if strings.TrimSpace(c.Look) == "" && strings.TrimSpace(c.Role) == "" {
		return errors.New("nhân vật chưa có mô tả ngoại hình hoặc vai trò — AI không có gì để dựng hồ sơ")
	}
	cli := gemini.NewFromSettings(st)
	if strings.TrimSpace(cli.Key) == "" {
		return errors.New("chưa cấu hình Gemini API key (Cấu hình & API) — bạn vẫn có thể tự điền hồ sơ")
	}

	var b strings.Builder
	b.WriteString("Dựng hồ sơ đầy đủ cho một nhân vật.\n\n")
	fmt.Fprintf(&b, "Tên (chỉ để bạn hiểu ngữ cảnh, KHÔNG được đưa vào imagePrompt): %s\n", strings.TrimSpace(c.Name))
	if s := strings.TrimSpace(c.Look); s != "" {
		fmt.Fprintf(&b, "Ngoại hình đã có: %s\n", s)
	}
	if s := strings.TrimSpace(c.Role); s != "" {
		fmt.Fprintf(&b, "Vai trò / tính cách đã có: %s\n", s)
	}
	b.WriteString("\nQuy tắc:\n")
	b.WriteString("- persona.* và voice.timbre/pitch/pace/accent/emotion: viết TIẾNG VIỆT.\n")
	b.WriteString("- persona.region: chỉ một trong \"Bắc\", \"Trung\", \"Nam\" — chọn theo thân phận và bối cảnh.\n")
	b.WriteString("- persona.personality: 3-5 từ.\n")
	b.WriteString("- imagePrompt: viết TIẾNG ANH, một câu tả chân dung bán thân góc ba phần tư, nền trung tính.\n")
	b.WriteString("  Chân thực đến từ sự KHÔNG hoàn hảo: lỗ chân lông, da không đều màu, mí mắt và lông mày hơi lệch nhau,\n")
	b.WriteString("  tóc con phá đường chân tóc, nếp nhăn đi theo cơ mặt. Vải phải có thớ, có sờn, có nếp đổ nặng.\n")
	b.WriteString("- Phần nào phải suy đoán vì mô tả gốc không nói, hãy ghi \"(suy đoán)\" ngay sau.\n")
	b.WriteString("- " + LuatPromptAnh + "\n")
	// Đưa KHUÔN đầy đủ chứ không mô tả suông: đo thật thấy khi chỉ viết
	// {"persona":{...}} thì model tự nghĩ ra tên khoá của nó (age thay ageRange,
	// role thay identity) và bỏ trống quá nửa số trường.
	b.WriteString("\nĐiền đúng khuôn này, giữ nguyên MỌI tên khoá, không thêm không bớt khoá:\n")
	b.WriteString(khuonJSON)

	out, err := cli.GenerateText(ctx, fillSystem, b.String())
	if err != nil {
		return fmt.Errorf("AI dựng hồ sơ thất bại: %w", err)
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(stripFence(out)), &raw); err != nil {
		return fmt.Errorf("AI trả về không đúng định dạng JSON: %w — nội dung: %.200s", err, out)
	}
	pp, vv := sub(raw, "persona"), sub(raw, "voice")
	if len(pp) == 0 && len(vv) == 0 {
		return fmt.Errorf("AI không trả về hồ sơ nào — nội dung: %.200s", out)
	}

	c.Persona = &store.Persona{
		Gender:      pickStr(pp, "gender", "sex", "gioiTinh"),
		AgeRange:    pickStr(pp, "ageRange", "age", "tuoi"),
		Identity:    pickStr(pp, "identity", "role", "occupation", "thanPhan"),
		Appearance:  pickStr(pp, "appearance", "look", "ngoaiHinh"),
		Personality: pickList(pp, "personality", "traits", "tinhCach"),
		Temperament: pickStr(pp, "temperament", "khiChat"),
		Motivation:  pickStr(pp, "motivation", "goal", "dongCo"),
		Arc:         pickStr(pp, "arc", "development", "tuyenPhatTrien"),
		Region:      pickStr(pp, "region", "accentRegion", "vungMien"),
	}
	c.VoiceSpec = &store.VoiceSpec{
		Timbre:  pickStr(vv, "timbre", "tone", "amSac"),
		Pitch:   pickStr(vv, "pitch", "caoDo"),
		Pace:    pickStr(vv, "pace", "speed", "nhip"),
		Accent:  pickStr(vv, "accent", "giongVung"),
		Emotion: pickStr(vv, "emotion", "mood", "camXuc"),
	}
	c.VoiceSpec.Prompt = VoicePrompt(c.Persona, c.VoiceSpec)
	voiceID, regionOK := MatchVoice(c.Persona, tts.VoicesFor(st))
	c.VoiceSpec.VoiceID = voiceID
	if voiceID != "" && !regionOK {
		c.VoiceSpec.Note = fmt.Sprintf("chưa có giọng miền %s trong máy — đã ghép giọng %s, bạn đổi tay được",
			c.Persona.Region, voiceID)
	}

	// Chốt chặn cuối: AI vẫn có thể quên luật và nhét tên vào prompt ảnh.
	c.ImagePrompt = StripNames(pickStr(raw, "imagePrompt", "image_prompt", "prompt"), c.Name)
	if c.Negative == "" {
		c.Negative = SheetNegative
	}
	return nil
}

// StripNames gỡ tên nhân vật khỏi prompt ảnh. Đây là chốt chặn cuối cùng —
// không tin lời hứa của model, kiểm lại bằng mã.
func StripNames(prompt, name string) string {
	p := strings.TrimSpace(prompt)
	for _, w := range strings.Fields(name) {
		w = strings.TrimSpace(w)
		if len([]rune(w)) < 2 {
			continue
		}
		p = strings.ReplaceAll(p, w+"'s ", "the ")
		p = strings.ReplaceAll(p, w+" ", "")
		p = strings.ReplaceAll(p, w, "")
	}
	return strings.TrimSpace(strings.Join(strings.Fields(p), " "))
}

// stripFence bóc rào ```json quanh câu trả lời và lấy đúng object JSON.
func stripFence(s string) string {
	t := strings.TrimSpace(s)
	t = strings.TrimPrefix(t, "```json")
	t = strings.TrimPrefix(t, "```")
	t = strings.TrimSuffix(t, "```")
	if i := strings.Index(t, "{"); i >= 0 {
		if j := strings.LastIndex(t, "}"); j > i {
			t = t[i : j+1]
		}
	}
	return strings.TrimSpace(t)
}
