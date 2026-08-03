package text2video

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"bizstudio/internal/media"
	"bizstudio/internal/store"
	"bizstudio/internal/tts"
)

// transcriptSegment — một đoạn lời đọc kèm mốc thời gian thật trong voice.wav.
type transcriptSegment struct {
	Index int     `json:"index"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// transcriptFile — nội dung transcript.json.
type transcriptFile struct {
	Language string              `json:"language"`
	Duration float64             `json:"duration"`
	Segments []transcriptSegment `json:"segments"`
}

// BuildVoice đọc từng đoạn kịch bản bằng TTS, ĐO thời lượng thật của mỗi đoạn
// (ffprobe) rồi ghép tất cả thành voice.wav và ghi transcript.json.
//
// Kết quả cập nhật thẳng vào sess: Segments[i].Seconds/AudioPath, VoicePath,
// TranscriptPath, VoiceSeconds. Hàm KHÔNG tự lưu store — người gọi lưu.
func BuildVoice(ctx context.Context, st *store.Store, sess *store.T2VSession, workDir string, upd func(float64, string)) error {
	if upd == nil {
		upd = func(float64, string) {}
	}
	if sess == nil {
		return errors.New("thiếu phiên Text → Video")
	}
	n := len(sess.Segments)
	if n == 0 {
		return errors.New("chưa có kịch bản — hãy viết kịch bản trước khi tạo giọng đọc")
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return fmt.Errorf("không tạo được thư mục phiên %s: %w", workDir, err)
	}
	tmpDir := filepath.Join(workDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return fmt.Errorf("không tạo được thư mục tạm %s: %w", tmpDir, err)
	}

	voiceID := voiceWithStyle(sess.VoiceID, sess.VoiceStyle)
	segs := make([]store.T2VSegment, n)
	parts := make([]string, 0, n)
	total := 0.0

	for i, seg := range sess.Segments {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("tạo giọng đọc bị hủy ở đoạn %d/%d: %w", i+1, n, err)
		}
		text := cleanSpoken(seg.Text)
		if text == "" {
			return fmt.Errorf("đoạn %d đang rỗng — sửa hoặc xoá đoạn này rồi chạy lại", i+1)
		}
		upd(float64(i)/float64(n)*88, fmt.Sprintf("Đọc đoạn %d/%d", i+1, n))

		name := fmt.Sprintf("seg-%d.wav", i+1)
		wav := filepath.Join(workDir, name)
		if err := tts.Speak(ctx, st, text, voiceID, 0, sess.VoiceEngine, wav); err != nil {
			return fmt.Errorf("đọc đoạn %d/%d thất bại: %w", i+1, n, err)
		}
		info, err := media.Probe(wav)
		if err != nil {
			return fmt.Errorf("không đo được thời lượng đoạn %d/%d: %w", i+1, n, err)
		}
		if info.Duration <= 0 {
			return fmt.Errorf("đoạn %d/%d đọc ra file audio rỗng — kiểm tra lại giọng đọc và nội dung đoạn", i+1, n)
		}

		norm := filepath.Join(tmpDir, fmt.Sprintf("norm-%d.wav", i+1))
		if err := normalizeAudio(ctx, wav, norm); err != nil {
			return fmt.Errorf("chuẩn hoá audio đoạn %d/%d: %w", i+1, n, err)
		}
		parts = append(parts, norm)

		segs[i] = store.T2VSegment{
			Text:      text,
			Chars:     utf8.RuneCountInString(text),
			Seconds:   info.Duration,
			AudioPath: dataRelPath(workDir, sess.ID, name),
			// Giữ nguyên storyboard: đọc lại giọng không được làm mất ảnh cảnh
			// (nhất là ảnh người dùng tự tải lên) và mô tả cảnh đã sửa tay.
			ImagePath:   seg.ImagePath,
			ImagePrompt: seg.ImagePrompt,
			ImageSource: seg.ImageSource,
		}
		total += info.Duration
	}

	upd(90, fmt.Sprintf("Ghép %d đoạn thành một file giọng đọc…", n))
	voicePath := filepath.Join(workDir, VoiceFile)
	if err := concatAudio(ctx, parts, tmpDir, voicePath); err != nil {
		return err
	}
	if info, err := media.Probe(voicePath); err == nil && info.Duration > 0 {
		total = info.Duration
	}

	upd(95, "Ghi mốc thời gian (transcript.json)…")
	if err := writeTranscript(filepath.Join(workDir, TranscriptFile), segs, total); err != nil {
		return err
	}

	sess.Segments = segs
	sess.VoicePath = dataRelPath(workDir, sess.ID, VoiceFile)
	sess.TranscriptPath = dataRelPath(workDir, sess.ID, TranscriptFile)
	sess.VoiceSeconds = round3(total)

	_ = os.RemoveAll(tmpDir) // chỉ dọn khi thành công, lỗi thì giữ lại để debug
	upd(99, fmt.Sprintf("Xong: %d đoạn, tổng %.1f giây", n, total))
	return nil
}

// writeTranscript ghi transcript.json với mốc start/end cộng dồn theo thời lượng THẬT.
func writeTranscript(dst string, segs []store.T2VSegment, total float64) error {
	items := make([]transcriptSegment, 0, len(segs))
	cursor := 0.0
	for i, s := range segs {
		start := cursor
		cursor += s.Seconds
		items = append(items, transcriptSegment{
			Index: i + 1,
			Start: round3(start),
			End:   round3(cursor),
			Text:  s.Text,
		})
	}
	data, err := json.MarshalIndent(transcriptFile{
		Language: "vi",
		Duration: round3(total),
		Segments: items,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("mã hoá transcript: %w", err)
	}
	if err := os.WriteFile(dst, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("ghi file %s: %w", dst, err)
	}
	return nil
}

// voiceWithStyle ghép style vào giọng theo quy ước VieNeu ("Tên@style", "clone:<id>@style").
func voiceWithStyle(voice, style string) string {
	v := strings.TrimSpace(voice)
	s := strings.TrimSpace(style)
	if v == "" || s == "" || strings.Contains(v, "@") {
		return v
	}
	return v + "@" + s
}

// round3 làm tròn 3 chữ số thập phân (mốc thời gian đọc cho dễ).
func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}
