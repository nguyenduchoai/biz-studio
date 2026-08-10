package text2video

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"unicode/utf8"

	"bizstudio/internal/gemini"
	"bizstudio/internal/openaiapi"
	"bizstudio/internal/store"
	"bizstudio/internal/util"
	"bizstudio/internal/vtemplate"
)

// maxSourceRunes — cắt bớt nguồn quá dài trước khi gửi LLM (giữ ngân sách token).
const maxSourceRunes = 24000

const scriptSystem = "Bạn là biên tập viên viết kịch bản lời đọc tiếng Việt cho video. " +
	"Luôn trả về đúng JSON mảng chuỗi được yêu cầu, không thêm bất kỳ nội dung nào khác."

// WriteScript nhờ LLM viết kịch bản LỜI ĐỌC từ nội dung nguồn và chia thành các
// đoạn ngắn (mỗi đoạn 1–3 câu, một ý). targetSeconds > 0 thì canh độ dài kịch
// bản theo tốc độ đọc ~15 ký tự/giây.
//
// templateID: khuôn lĩnh vực (rỗng = không dùng khuôn).
//
// engine: "claude" (Claude CLI) | "gemini" | "openai" (API Trực Tiếp);
// rỗng → tự chọn theo Cấu hình & API: API Trực Tiếp → Gemini → Claude CLI.
// model: rỗng thì dùng model mặc định của engine.
func WriteScript(ctx context.Context, st *store.Store, src, engine, model string, targetSeconds int, templateID string) ([]store.T2VSegment, error) {
	source := strings.TrimSpace(src)
	if source == "" {
		return nil, errors.New("chưa có nội dung nguồn — dán văn bản hoặc lấy nội dung từ link trước khi viết kịch bản")
	}
	if r := []rune(source); len(r) > maxSourceRunes {
		source = string(r[:maxSourceRunes])
	}

	// Giới hạn độ dài lời đọc mỗi đoạn lấy từ bộ style đang dùng — đưa vào
	// prompt để LLM viết đúng cỡ, và vẫn cắt hậu kiểm phòng LLM viết quá tay.
	maxChars := VoiceCharLimit(st)

	raw, err := runScriptLLM(ctx, st, engine, model, buildScriptPrompt(source, targetSeconds, maxChars, templateID))
	if err != nil {
		return nil, err
	}
	segs := parseScript(raw, maxChars)
	if len(segs) == 0 {
		return nil, fmt.Errorf("LLM không trả về đoạn kịch bản nào — thử lại hoặc đổi engine viết kịch bản (nội dung nhận được: %s)",
			shortText(raw, 200))
	}
	return segs, nil
}

// buildScriptPrompt dựng prompt tiếng Việt: văn nói, không markdown, không URL,
// chia đoạn, trả JSON mảng chuỗi. maxChars > 0 thì ràng độ dài từng đoạn.
// tplID rỗng = không dùng khuôn.
func buildScriptPrompt(source string, targetSeconds, maxChars int, tplID string) string {
	var b strings.Builder
	b.WriteString(`Viết kịch bản LỜI ĐỌC (voice-over) tiếng Việt cho video, dựa trên nội dung nguồn bên dưới.

Quy tắc bắt buộc:
- Viết văn NÓI: câu ngắn, mạch lạc, đọc lên nghe tự nhiên như người dẫn chương trình. Không viết văn viết, không liệt kê khô khan.
- TUYỆT ĐỐI không dùng markdown, không emoji, không dấu đầu dòng, không tiêu đề, không ký hiệu đặc biệt (dấu sao, thăng, gạch dưới, ngoặc nhọn, dấu backtick).
- Không đọc địa chỉ web, không đọc URL, không đọc tên file, không nhắc tới "bài viết", "đường link" hay "bên dưới".
- Viết số và ký hiệu sao cho đọc lên tự nhiên (ví dụ "hai mươi phần trăm" thay vì dạng ký hiệu).
- Mỗi đoạn 1 đến 3 câu và chỉ chứa MỘT ý; đoạn đầu mở bài cuốn hút, đoạn cuối chốt lại và kêu gọi hành động.
- Chỉ dùng thông tin có trong nguồn, không bịa thêm số liệu hay sự kiện.
`)
	if targetSeconds > 0 {
		chars := targetSeconds * charsPerSecond
		parts := chars / 130
		if parts < 3 {
			parts = 3
		}
		fmt.Fprintf(&b, "- Video dài khoảng %d giây: tổng kịch bản khoảng %d ký tự (tốc độ đọc khoảng %d ký tự mỗi giây), chia thành khoảng %d đoạn.\n",
			targetSeconds, chars, charsPerSecond, parts)
	} else {
		b.WriteString("- Độ dài vừa đủ truyền tải nội dung, thường từ 6 đến 14 đoạn.\n")
	}
	if maxChars > 0 {
		fmt.Fprintf(&b, "- Mỗi đoạn TỐI ĐA %d ký tự (kể cả dấu cách); đoạn nào dài hơn phải tách thành đoạn mới.\n", maxChars)
	}
	// Khuôn đặt SAU các quy tắc bắt buộc và TRƯỚC nguồn: nói rõ đây là cách kể,
	// không phải nội dung — nếu để lẫn, model đem nguyên câu trong khuôn ("Mở
	// đầu bằng nỗi đau cụ thể") vào chính lời đọc.
	if tpl, ok := vtemplate.Find(tplID); ok {
		b.WriteString("\nKhuôn kể chuyện cần theo — đây là HƯỚNG DẪN cách kể, tuyệt đối không đưa nguyên văn mấy dòng này vào lời đọc:\n")
		if tpl.Script != "" {
			fmt.Fprintf(&b, "- %s\n", tpl.Script)
		}
		if tpl.Hook != "" {
			fmt.Fprintf(&b, "- Đoạn mở đầu: %s\n", tpl.Hook)
		}
		if tpl.Body != "" {
			fmt.Fprintf(&b, "- Các đoạn giữa: %s\n", tpl.Body)
		}
		if tpl.CTA != "" {
			fmt.Fprintf(&b, "- Đoạn chốt: %s\n", tpl.CTA)
		}
	}
	b.WriteString(`
Trả về DUY NHẤT một JSON mảng các chuỗi, mỗi phần tử là một đoạn lời đọc:
["đoạn một", "đoạn hai", "đoạn ba"]
Không giải thích, không markdown, không văn bản nào khác ngoài JSON.

Nội dung nguồn:
`)
	b.WriteString(source)
	return b.String()
}

// runScriptLLM gọi engine LLM tương ứng.
func runScriptLLM(ctx context.Context, st *store.Store, engine, model, prompt string) (string, error) {
	set := st.Settings()
	eng := strings.ToLower(strings.TrimSpace(engine))
	if eng == "" {
		switch {
		case strings.TrimSpace(set.OpenAIKey) != "":
			eng = "openai"
		case strings.TrimSpace(set.GeminiAPIKey) != "":
			eng = "gemini"
		default:
			eng = "claude"
		}
	}
	model = strings.TrimSpace(model)

	switch eng {
	case "gemini":
		cli := gemini.NewFromSettings(st)
		if model != "" {
			cli.Model = model
		}
		return cli.GenerateText(ctx, scriptSystem, prompt)
	case "openai":
		cli := openaiapi.NewFromSettings(st)
		if model != "" {
			cli.Model = model
		}
		return cli.ChatText(ctx, scriptSystem, prompt)
	case "claude":
		return runClaudeScript(ctx, set.ClaudeBin, model, scriptSystem+"\n\n"+prompt)
	default:
		return "", fmt.Errorf("engine viết kịch bản không hỗ trợ: %q (hỗ trợ \"claude\", \"gemini\", \"openai\")", engine)
	}
}

// runClaudeScript chạy Claude CLI: <bin> -p --output-format text [--model M],
// prompt đưa qua stdin. Lỗi bọc kèm gợi ý từ util.ClaudeFailReason.
func runClaudeScript(ctx context.Context, bin, model, prompt string) (string, error) {
	bin = strings.TrimSpace(bin)
	if bin == "" {
		bin = "claude"
	}
	args := []string{"-p", "--output-format", "text"}
	if model != "" {
		args = append(args, "--model", model)
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = strings.NewReader(prompt)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("chạy Claude CLI (%s) thất bại: %w — %s",
			bin, err, util.ClaudeFailReason(so.String(), se.String()))
	}
	out := strings.TrimSpace(so.String())
	if out == "" {
		return "", fmt.Errorf("Claude CLI không trả về nội dung nào — %s",
			util.ClaudeFailReason("", se.String()))
	}
	return out, nil
}

// parseScript đọc kết quả LLM thành danh sách đoạn: ưu tiên JSON mảng chuỗi
// (hoặc mảng object có field text), không parse được thì tách theo dòng trống.
// maxChars > 0 thì cắt hậu kiểm từng đoạn theo giới hạn của bộ style.
func parseScript(raw string, maxChars int) []store.T2VSegment {
	body := stripFence(raw)
	lines := parseJSONScript(body)
	if len(lines) == 0 {
		lines = splitParagraphs(body)
	}
	out := make([]store.T2VSegment, 0, len(lines))
	for _, ln := range lines {
		t := trimSpoken(cleanSpoken(ln), maxChars)
		if t == "" {
			continue
		}
		out = append(out, store.T2VSegment{Text: t, Chars: utf8.RuneCountInString(t)})
	}
	return out
}

// VoiceCharLimit trả giới hạn ký tự lời đọc mỗi đoạn của bộ style đang dùng
// (0 = không giới hạn).
func VoiceCharLimit(st *store.Store) int {
	if st == nil {
		return 0
	}
	if k, ok := st.ActiveStyleKit(); ok && k.MaxVoiceChars > 0 {
		return k.MaxVoiceChars
	}
	return 0
}

// trimSpoken cắt một đoạn lời đọc về đúng giới hạn ký tự: ưu tiên dừng ở cuối
// câu gần nhất (nếu không cắt cụt quá nửa đoạn), không có thì dừng ở khoảng
// trắng — cắt giữa từ nghe rất gãy.
func trimSpoken(v string, maxChars int) string {
	if maxChars <= 0 {
		return v
	}
	head := []rune(v)
	if len(head) <= maxChars {
		return v
	}
	head = head[:maxChars]
	keep := maxChars * 6 / 10 // mốc tối thiểu để không cắt mất quá nhiều nội dung
	for i := len(head) - 1; i >= keep; i-- {
		if head[i] == '.' || head[i] == '!' || head[i] == '?' || head[i] == '…' {
			return strings.TrimSpace(string(head[:i+1]))
		}
	}
	for i := len(head) - 1; i > 0; i-- {
		if head[i] == ' ' {
			return strings.TrimSpace(string(head[:i])) + "."
		}
	}
	return strings.TrimSpace(string(head))
}

// parseJSONScript thử đọc mảng JSON: ["…"] hoặc [{"text":"…"}].
func parseJSONScript(body string) []string {
	arr := extractJSONArray(body)
	if arr == "" {
		return nil
	}
	var texts []string
	if err := json.Unmarshal([]byte(arr), &texts); err == nil {
		return texts
	}
	var objs []struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(arr), &objs); err == nil {
		out := make([]string, 0, len(objs))
		for _, o := range objs {
			out = append(out, o.Text)
		}
		return out
	}
	return nil
}

// stripFence bỏ khối ```lang … ``` bao quanh output LLM (nếu có).
func stripFence(v string) string {
	t := strings.TrimSpace(v)
	if !strings.HasPrefix(t, "```") {
		return t
	}
	t = strings.TrimPrefix(t, "```")
	if i := strings.Index(t, "\n"); i >= 0 {
		t = t[i+1:]
	} else {
		t = ""
	}
	if i := strings.LastIndex(t, "```"); i >= 0 {
		t = t[:i]
	}
	return strings.TrimSpace(t)
}

// extractJSONArray cắt từ '[' đầu tiên tới ']' cuối cùng (phòng LLM kèm lời dẫn).
func extractJSONArray(v string) string {
	i := strings.Index(v, "[")
	j := strings.LastIndex(v, "]")
	if i >= 0 && j > i {
		return v[i : j+1]
	}
	return ""
}

// splitParagraphs tách văn bản thường thành đoạn theo dòng trống; không có dòng
// trống thì mỗi dòng là một đoạn.
func splitParagraphs(v string) []string {
	v = strings.ReplaceAll(v, "\r\n", "\n")
	blocks := regexp.MustCompile(`\n\s*\n`).Split(v, -1)
	if len(blocks) <= 1 {
		blocks = strings.Split(v, "\n")
	}
	out := make([]string, 0, len(blocks))
	for _, b := range blocks {
		if t := strings.TrimSpace(strings.ReplaceAll(b, "\n", " ")); t != "" {
			out = append(out, t)
		}
	}
	return out
}

var (
	reBulletPrefix = regexp.MustCompile(`^\s*(?:[-*+•–—]|\d+[.)]|[IVXivx]+[.)])\s+`)
	reMarkdownChar = regexp.MustCompile("[*#_`>|~\\[\\]{}]")
	reSpaceRun     = regexp.MustCompile(`\s+`)
)

// cleanSpoken làm sạch một đoạn lời đọc: bỏ dấu đầu dòng, ký hiệu markdown,
// gộp khoảng trắng (giữ nguyên dấu câu và dấu tiếng Việt).
func cleanSpoken(v string) string {
	t := strings.TrimSpace(v)
	t = reBulletPrefix.ReplaceAllString(t, "")
	t = reMarkdownChar.ReplaceAllString(t, "")
	t = reSpaceRun.ReplaceAllString(t, " ")
	return strings.TrimSpace(t)
}
