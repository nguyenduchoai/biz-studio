package dubbing

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bizstudio/internal/gemini"
	"bizstudio/internal/media"
	"bizstudio/internal/store"
)

const asrPrompt = "Bóc băng audio này thành phụ đề SRT chuẩn (số thứ tự, " +
	"HH:MM:SS,mmm --> HH:MM:SS,mmm, mỗi cue tối đa 2 dòng, mốc thời gian bám sát " +
	"thời điểm nói thực tế). Chỉ trả về nội dung SRT, không giải thích gì thêm."

// transcribe tự bóc băng audio của video thành file SRT trong workDir.
// Cần Gemini API key trong Cấu hình & API.
func transcribe(ctx context.Context, st *store.Store, videoPath, workDir, tmpDir string, upd func(float64, string)) (string, error) {
	if strings.TrimSpace(st.Settings().GeminiAPIKey) == "" {
		return "", fmt.Errorf("cần file phụ đề .srt hoặc cấu hình Gemini API key để tự bóc băng")
	}
	upd(3, "Tách audio 16kHz để bóc băng…")
	wav := filepath.Join(tmpDir, "asr-16k.wav")
	if err := media.ExtractAudioWav16k(ctx, videoPath, wav); err != nil {
		return "", fmt.Errorf("tách audio từ video để bóc băng thất bại: %w", err)
	}

	upd(6, "Gửi Gemini bóc băng…")
	text, err := gemini.NewFromSettings(st).GenerateWithFiles(ctx, asrPrompt, []string{wav})
	if err != nil {
		return "", fmt.Errorf("tự bóc băng thất bại: %w", err)
	}
	srt := stripCodeFence(text)
	if strings.TrimSpace(srt) == "" {
		return "", fmt.Errorf("Gemini không trả về nội dung SRT nào khi bóc băng video")
	}

	dst := filepath.Join(workDir, "auto.srt")
	if err := os.WriteFile(dst, []byte(srt+"\n"), 0o644); err != nil {
		return "", fmt.Errorf("ghi file phụ đề tự bóc băng %s: %w", dst, err)
	}
	upd(9, "Đã bóc băng xong phụ đề")
	return dst, nil
}

// stripCodeFence bỏ khối ```lang … ``` bao quanh output của LLM (nếu có).
func stripCodeFence(v string) string {
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
