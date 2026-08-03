package translate

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"bizstudio/internal/gemini"
	"bizstudio/internal/openaiapi"
	"bizstudio/internal/store"
	"bizstudio/internal/util"
)

const claudeTimeout = 300 * time.Second

// Text dịch một đoạn văn bản. mode: phim | sub | truyen | khoahoc.
// engine: "claude" (mặc định, dùng Claude CLI qua subscription — không cần API key)
// | "gemini" | "openai" (API Trực Tiếp — endpoint OpenAI-compatible).
func Text(ctx context.Context, st *store.Store, text, mode, engine, targetLang string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("nội dung cần dịch trống")
	}
	system := systemPrompt(mode, normLang(targetLang))
	return runEngine(ctx, st, engine, system, text)
}

// normLang: targetLang mặc định "tiếng Việt".
func normLang(l string) string {
	l = strings.TrimSpace(l)
	if l == "" {
		return "tiếng Việt"
	}
	return l
}

// systemPrompt tạo prompt hệ thống theo chế độ dịch.
func systemPrompt(mode, lang string) string {
	base := fmt.Sprintf("Bạn là chuyên gia dịch thuật. Dịch nội dung sau sang %s.", lang)
	switch mode {
	case "phim":
		return base + " Dịch tự nhiên theo văn nói đời thường như lời thoại phim, không dịch máy móc từng chữ. Chỉ trả về bản dịch, không giải thích."
	case "sub":
		return base + " Đây là phụ đề video: dịch ngắn gọn, mỗi dòng tối đa 42 ký tự, giữ trọn ý chính. Chỉ trả về bản dịch, không giải thích."
	case "truyen":
		return base + " Văn phong kể chuyện mượt mà, giàu cảm xúc, tự nhiên như truyện dịch chuyên nghiệp. Chỉ trả về bản dịch, không giải thích."
	case "khoahoc":
		return base + " Nội dung khoa học: dịch chính xác thuật ngữ chuyên ngành, diễn đạt rõ ràng, nhất quán. Chỉ trả về bản dịch, không giải thích."
	default:
		return base + " Dịch chính xác và tự nhiên. Chỉ trả về bản dịch, không giải thích."
	}
}

// runEngine chọn engine dịch: "claude" (mặc định), "gemini" hoặc "openai".
func runEngine(ctx context.Context, st *store.Store, engine, system, text string) (string, error) {
	switch engine {
	case "", "claude":
		return runClaude(ctx, st, system, text)
	case "gemini":
		return gemini.NewFromSettings(st).GenerateText(ctx, system, text)
	case "openai":
		return openaiapi.NewFromSettings(st).ChatText(ctx, system, text)
	default:
		return "", fmt.Errorf("engine dịch không hỗ trợ: %q (chỉ hỗ trợ \"claude\", \"gemini\" hoặc \"openai\")", engine)
	}
}

// runClaude chạy Claude CLI: bin -p --output-format text, prompt qua stdin
// (system prompt gộp vào đầu). Timeout 300s.
func runClaude(ctx context.Context, st *store.Store, system, text string) (string, error) {
	bin := strings.TrimSpace(st.Settings().ClaudeBin)
	if bin == "" {
		bin = "claude"
	}
	cctx, cancel := context.WithTimeout(ctx, claudeTimeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, bin, "-p", "--output-format", "text")
	cmd.Stdin = strings.NewReader(system + "\n\n" + text)
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
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
