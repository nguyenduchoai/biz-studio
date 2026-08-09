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

// LuatPromptAnh — luật viết prompt ảnh, tách riêng để bài kiểm tra soi được.
const LuatPromptAnh = `TUYỆT ĐỐI KHÔNG viết tên nhân vật, biệt danh, tên tác giả hay tên tác phẩm vào imagePrompt.
Model sinh ảnh thiên vị rất nặng với tên riêng: gọi tên là nó vẽ nhân vật nó đã học chứ không vẽ nhân vật này.
Hãy MÔ TẢ con người, đừng gọi tên.`

type fillResult struct {
	Persona struct {
		Gender      string   `json:"gender"`
		AgeRange    string   `json:"ageRange"`
		Identity    string   `json:"identity"`
		Appearance  string   `json:"appearance"`
		Personality []string `json:"personality"`
		Temperament string   `json:"temperament"`
		Motivation  string   `json:"motivation"`
		Arc         string   `json:"arc"`
		Region      string   `json:"region"`
	} `json:"persona"`
	Voice struct {
		Timbre  string `json:"timbre"`
		Pitch   string `json:"pitch"`
		Pace    string `json:"pace"`
		Accent  string `json:"accent"`
		Emotion string `json:"emotion"`
	} `json:"voice"`
	ImagePrompt string `json:"imagePrompt"`
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
	b.WriteString("\nTrả về JSON: {\"persona\":{...},\"voice\":{...},\"imagePrompt\":\"...\"}")

	out, err := cli.GenerateText(ctx, fillSystem, b.String())
	if err != nil {
		return fmt.Errorf("AI dựng hồ sơ thất bại: %w", err)
	}
	var r fillResult
	if err := json.Unmarshal([]byte(stripFence(out)), &r); err != nil {
		return fmt.Errorf("AI trả về không đúng định dạng JSON: %w — nội dung: %.200s", err, out)
	}

	c.Persona = &store.Persona{
		Gender: r.Persona.Gender, AgeRange: r.Persona.AgeRange, Identity: r.Persona.Identity,
		Appearance: r.Persona.Appearance, Personality: r.Persona.Personality,
		Temperament: r.Persona.Temperament, Motivation: r.Persona.Motivation,
		Arc: r.Persona.Arc, Region: r.Persona.Region,
	}
	c.VoiceSpec = &store.VoiceSpec{
		Timbre: r.Voice.Timbre, Pitch: r.Voice.Pitch, Pace: r.Voice.Pace,
		Accent: r.Voice.Accent, Emotion: r.Voice.Emotion,
	}
	c.VoiceSpec.Prompt = VoicePrompt(c.Persona, c.VoiceSpec)
	c.VoiceSpec.VoiceID = MatchVoice(c.Persona, tts.VoicesFor(st))

	// Chốt chặn cuối: AI vẫn có thể quên luật và nhét tên vào prompt ảnh.
	c.ImagePrompt = StripNames(r.ImagePrompt, c.Name)
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
