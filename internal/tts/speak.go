package tts

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"bizstudio/internal/gemini"
	"bizstudio/internal/store"
	"bizstudio/internal/util"
)

const defaultSayRate = 175

// Speak tổng hợp giọng nói ra file dst.
// engine "vieneu": VieNeu-TTS on-device (48 kHz, tiếng Việt tự nhiên — mặc định nếu đã cài);
// "clone": giọng nhân bản (cũng chạy bằng VieNeu, voiceID dạng "clone:<id>[@style]");
// "say": macOS say → ffmpeg convert; "gemini": Gemini TTS (dst nên là .wav);
// engine rỗng: tự chọn theo thứ tự vieneu → say → gemini (voiceID clone → luôn vieneu).
func Speak(ctx context.Context, st *store.Store, text, voiceID string, rate float64, engine, dst string) error {
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("nội dung TTS trống")
	}
	if strings.TrimSpace(dst) == "" {
		return fmt.Errorf("thiếu đường dẫn file audio đầu ra")
	}
	if dir := filepath.Dir(dst); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("tạo thư mục đầu ra %s: %w", dir, err)
		}
	}

	if engine == "" {
		switch {
		case IsCloneVoice(voiceID):
			engine = "vieneu" // giọng nhân bản chỉ chạy được bằng VieNeu-TTS
		case VieNeuAvailable(st):
			engine = "vieneu"
		case util.Exists("say"):
			engine = "say"
		default:
			engine = "gemini"
		}
		// Giọng đang chọn thuộc engine khác → tôn trọng engine của giọng.
		if voiceID != "" && !IsCloneVoice(voiceID) {
			if e := engineOfVoice(st, voiceID); e != "" {
				engine = e
			}
		}
	}
	switch engine {
	case "vieneu", "clone":
		refAudio, err := cloneRefFor(st, voiceID)
		if err != nil {
			return err
		}
		return speakVieNeu(ctx, st, text, voiceID, refAudio, dst)
	case "say":
		return speakSay(ctx, text, voiceID, rate, dst)
	case "gemini":
		return gemini.NewFromSettings(st).TTS(ctx, text, voiceID, dst)
	default:
		return fmt.Errorf("engine TTS không hỗ trợ: %q (hỗ trợ \"vieneu\", \"clone\", \"say\", \"gemini\")", engine)
	}
}

// engineOfVoice tìm engine sở hữu voiceID (bỏ hậu tố @style nếu có).
func engineOfVoice(st *store.Store, voiceID string) string {
	base := voiceID
	if i := strings.LastIndex(base, "@"); i >= 0 {
		base = base[:i]
	}
	for _, v := range VoicesFor(st) {
		if v.ID == base || v.Name == base {
			return v.Engine
		}
	}
	return ""
}

// speakSay: ghi text ra file tạm → say -o tmp.aiff → ffmpeg convert sang dst.
func speakSay(ctx context.Context, text, voiceID string, rate float64, dst string) error {
	tmpTxt, err := os.CreateTemp("", "bizstudio-tts-*.txt")
	if err != nil {
		return fmt.Errorf("tạo file text tạm: %w", err)
	}
	tmpTxtPath := tmpTxt.Name()
	defer os.Remove(tmpTxtPath)
	if _, err := tmpTxt.WriteString(text); err != nil {
		tmpTxt.Close()
		return fmt.Errorf("ghi file text tạm: %w", err)
	}
	if err := tmpTxt.Close(); err != nil {
		return fmt.Errorf("đóng file text tạm: %w", err)
	}

	tmpAiff := tmpTxtPath + ".aiff"
	defer os.Remove(tmpAiff)

	r := defaultSayRate
	if rate > 0 {
		r = int(rate)
	}
	args := []string{}
	if voiceID != "" {
		args = append(args, "-v", voiceID)
	}
	args = append(args, "-r", strconv.Itoa(r), "-o", tmpAiff, "--input-file="+tmpTxtPath)
	if _, err := util.Run(ctx, "say", args...); err != nil {
		return fmt.Errorf("chạy say thất bại: %w", err)
	}

	ff := []string{"-y", "-i", tmpAiff}
	switch strings.ToLower(filepath.Ext(dst)) {
	case ".wav":
		ff = append(ff, "-ar", "24000")
	case ".m4a":
		ff = append(ff, "-c:a", "aac")
	case ".mp3":
		ff = append(ff, "-c:a", "libmp3lame")
	}
	ff = append(ff, dst)
	if _, err := util.Run(ctx, "ffmpeg", ff...); err != nil {
		return fmt.Errorf("chuyển đổi audio bằng ffmpeg thất bại: %w", err)
	}
	return nil
}
