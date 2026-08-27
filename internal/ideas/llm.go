package ideas

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"bizstudio/internal/gemini"
	"bizstudio/internal/openaiapi"
	"bizstudio/internal/store"
	"bizstudio/internal/util"
)

// ideaSystem — chỉ dẫn hệ thống chung cho mọi engine.
const ideaSystem = "Bạn là chuyên gia nội dung video tiếng Việt. " +
	"Luôn trả về đúng JSON được yêu cầu, không thêm bất kỳ nội dung nào khác."

// runLLM gọi engine LLM theo Cấu hình & API, thứ tự ưu tiên giống bước viết kịch
// bản của Text → Video: API Trực Tiếp (OpenAI-compatible) → Gemini → Claude CLI.
func runLLM(ctx context.Context, st *store.Store, prompt string) (string, error) {
	set := st.Settings()
	switch {
	case strings.TrimSpace(set.OpenAIKey) != "":
		return openaiapi.NewFromSettings(st).ChatText(ctx, ideaSystem, prompt)
	case strings.TrimSpace(set.GeminiAPIKey) != "":
		return gemini.NewFromSettings(st).GenerateText(ctx, ideaSystem, prompt)
	default:
		return runClaude(ctx, set.ClaudeBin, ideaSystem+"\n\n"+prompt)
	}
}

// runClaude chạy Claude CLI với model mặc định của chính Claude CLI.
// đưa qua stdin. Lỗi bọc kèm gợi ý dễ hiểu từ util.ClaudeFailReason (Claude CLI
// in nhiều lỗi quan trọng ra stdout, không phải stderr).
func runClaude(ctx context.Context, bin, prompt string) (string, error) {
	bin = strings.TrimSpace(bin)
	if bin == "" {
		bin = "claude"
	}
	args := []string{"-p", "--output-format", "text"}
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

// ---------- làm sạch văn bản ----------

var (
	reBulletPrefix = regexp.MustCompile(`^\s*(?:[-*+•–—]|\d+[.)]|[IVXivx]+[.)])\s+`)
	reMarkdownChar = regexp.MustCompile("[*#_`>|~\\[\\]{}]")
	reSpaceRun     = regexp.MustCompile(`\s+`)
)

// cleanLine làm sạch một dòng do LLM sinh ra: bỏ đánh số / gạch đầu dòng, ký
// hiệu markdown, dấu nháy bao ngoài và gộp mọi khoảng trắng thành một dấu cách.
func cleanLine(v string) string {
	t := strings.TrimSpace(v)
	t = reBulletPrefix.ReplaceAllString(t, "")
	t = reMarkdownChar.ReplaceAllString(t, "")
	t = reSpaceRun.ReplaceAllString(t, " ")
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(t), "\"'"))
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

// shortText rút gọn chuỗi theo rune (an toàn tiếng Việt).
func shortText(v string, n int) string {
	v = strings.TrimSpace(v)
	r := []rune(v)
	if len(r) > n {
		return strings.TrimSpace(string(r[:n])) + "…"
	}
	return v
}
