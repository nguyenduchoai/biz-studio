// Package dubbing — engine lồng tiếng video theo phụ đề SRT: đọc từng câu bằng
// TTS, ép nhịp cho khớp mốc thời gian của phụ đề, ghép thành một track liền mạch
// rồi mux vào video gốc.
package dubbing

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bizstudio/internal/store"
	"bizstudio/internal/translate"
)

// Config — cấu hình lồng tiếng.
type Config struct {
	Voice  string `json:"voice"`  // ID giọng đọc (hỗ trợ "Tên@style", "clone:<id>")
	Engine string `json:"engine"` // "" tự chọn | "vieneu" | "say" | "gemini"
	Style  string `json:"style"`  // style VieNeu: tu_nhien | tin_tuc | doc_truyen

	TargetLang      string `json:"targetLang"`      // != "" → dịch phụ đề trước khi đọc
	TranslateEngine string `json:"translateEngine"` // "claude" | "gemini" | "openai"

	KeepOriginal   bool    `json:"keepOriginal"`   // giữ tiếng gốc làm nền
	OriginalVolume float64 `json:"originalVolume"` // 0..1, mặc định 0.12

	FitTiming bool    `json:"fitTiming"` // ép câu đọc vừa slot phụ đề
	MaxSpeed  float64 `json:"maxSpeed"`  // trần tăng tốc, mặc định 1.6
}

// Result — kết quả lồng tiếng (đường dẫn tuyệt đối).
type Result struct {
	VideoPath string `json:"videoPath"` // rỗng nếu không có video đầu vào
	AudioPath string `json:"audioPath"` // track lồng tiếng đã ghép
	SrtPath   string `json:"srtPath"`   // file SRT thực sự dùng (bản dịch nếu có)
}

// Mốc tiến độ của từng giai đoạn (0..100).
const (
	progTransFrom = 10.0
	progTransTo   = 20.0
	progTTSFrom   = 20.0
	progTTSTo     = 82.0
	progTrack     = 84.0
	progMux       = 92.0
)

// Run lồng tiếng cho video theo phụ đề SRT.
//
// Luồng: (1) chưa có SRT → tự bóc băng bằng Gemini; (2) TargetLang != "" → dịch
// phụ đề; (3) mỗi cue → TTS + ép nhịp; (4) ghép silence + đoạn đọc thành track;
// (5) có video → mux (giữ tiếng gốc nếu được yêu cầu).
//
// Thư mục tmp/ chỉ được dọn khi thành công — khi lỗi giữ nguyên để debug.
func Run(ctx context.Context, st *store.Store, videoPath, srtPath string, cfg Config, workDir string, upd func(float64, string)) (Result, error) {
	if upd == nil {
		upd = func(float64, string) {}
	}
	videoPath, srtPath = strings.TrimSpace(videoPath), strings.TrimSpace(srtPath)
	if videoPath == "" && srtPath == "" {
		return Result{}, errors.New("cần ít nhất đường dẫn video hoặc file phụ đề .srt để lồng tiếng")
	}
	if strings.TrimSpace(workDir) == "" {
		return Result{}, errors.New("thiếu thư mục làm việc cho tác vụ lồng tiếng")
	}
	if abs, err := filepath.Abs(workDir); err == nil {
		workDir = abs
	}
	tmpDir := filepath.Join(workDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("không tạo được thư mục làm việc %s: %w", tmpDir, err)
	}

	usedSRT, err := prepareSRT(ctx, st, videoPath, srtPath, cfg, workDir, tmpDir, upd)
	if err != nil {
		return Result{}, err
	}
	cues, err := loadCues(usedSRT)
	if err != nil {
		return Result{}, err
	}
	segs, err := speakCues(ctx, st, cues, cfg, tmpDir, upd)
	if err != nil {
		return Result{}, err
	}

	dubWav := filepath.Join(workDir, "dubbed.wav")
	if err := buildTrack(ctx, segs, tmpDir, dubWav, upd); err != nil {
		return Result{}, err
	}

	res := Result{AudioPath: dubWav, SrtPath: usedSRT}
	if videoPath != "" {
		upd(progMux, "Ghép tiếng vào video…")
		out := filepath.Join(workDir, "dubbed.mp4")
		if err := muxVideo(ctx, st, videoPath, dubWav, out, cfg); err != nil {
			return Result{}, err
		}
		res.VideoPath = out
	}

	upd(98, "Dọn dẹp file tạm…")
	_ = os.RemoveAll(tmpDir)
	upd(100, fmt.Sprintf("Hoàn tất: đã lồng tiếng %d câu", len(segs)))
	return res, nil
}

// prepareSRT trả về file SRT dùng để lồng tiếng: tự bóc băng khi chưa có,
// và dịch sang ngôn ngữ đích nếu cfg.TargetLang != "".
func prepareSRT(ctx context.Context, st *store.Store, videoPath, srtPath string, cfg Config, workDir, tmpDir string, upd func(float64, string)) (string, error) {
	if srtPath == "" {
		upd(2, "Chưa có phụ đề — tự bóc băng từ video…")
		auto, err := transcribe(ctx, st, videoPath, workDir, tmpDir, upd)
		if err != nil {
			return "", err
		}
		srtPath = auto
	} else if fi, err := os.Stat(srtPath); err != nil || fi.IsDir() {
		return "", fmt.Errorf("không tìm thấy file phụ đề: %s", srtPath)
	}

	lang := strings.TrimSpace(cfg.TargetLang)
	if lang == "" {
		return srtPath, nil
	}
	upd(progTransFrom, "Dịch phụ đề sang "+lang+"…")
	// Dịch trên bản sao trong thư mục làm việc để không ghi đè file của người dùng.
	local := filepath.Join(workDir, "source.srt")
	if err := copyFile(srtPath, local); err != nil {
		return "", fmt.Errorf("sao chép phụ đề vào thư mục làm việc: %w", err)
	}
	translated, err := translate.File(ctx, st, local, "sub", cfg.TranslateEngine, lang,
		func(p float64, d string) {
			upd(progTransFrom+p/100*(progTransTo-progTransFrom), d)
		})
	if err != nil {
		return "", fmt.Errorf("dịch phụ đề sang %s thất bại: %w", lang, err)
	}
	return translated, nil
}

// originalVolume — âm lượng tiếng gốc khi giữ làm nền (mặc định 0.12).
func originalVolume(cfg Config) float64 {
	if cfg.OriginalVolume <= 0 || cfg.OriginalVolume > 1 {
		return 0.12
	}
	return cfg.OriginalVolume
}

// maxSpeed — trần tăng tốc câu đọc (mặc định 1.6, chặn trên 3.0 để tiếng còn nghe được).
func maxSpeed(cfg Config) float64 {
	switch {
	case cfg.MaxSpeed <= 1:
		return 1.6
	case cfg.MaxSpeed > 3:
		return 3
	default:
		return cfg.MaxSpeed
	}
}
