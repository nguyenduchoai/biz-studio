package dubbing

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"bizstudio/internal/media"
	"bizstudio/internal/store"
	"bizstudio/internal/tts"
)

const (
	segRate       = 44100 // sample rate chuẩn hoá mọi đoạn audio trước khi ghép
	minFitSlot    = 0.25  // slot ngắn hơn mức này thì không ép tốc độ (dễ méo tiếng)
	tempoLayerMax = 2.0   // mỗi tầng atempo của ffmpeg chỉ nhận 0.5–2.0
)

// segment — đoạn audio đã lồng tiếng cho một cue (wav 44100 mono).
type segment struct {
	Path  string
	Dur   float64
	Start float64
}

// speakCues đọc từng cue thành file wav 44100 mono; câu dài hơn slot phụ đề sẽ
// được tăng tốc bằng atempo (nếu cfg.FitTiming), câu ngắn hơn giữ nguyên.
func speakCues(ctx context.Context, st *store.Store, cues []cue, cfg Config, tmpDir string, upd func(float64, string)) ([]segment, error) {
	vid := voiceID(cfg)
	n := len(cues)
	segs := make([]segment, 0, n)
	for i, c := range cues {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("lồng tiếng bị hủy ở câu %d/%d: %w", i+1, n, err)
		}
		upd(progTTSFrom+float64(i)/float64(n)*(progTTSTo-progTTSFrom),
			fmt.Sprintf("Lồng tiếng câu %d/%d", i+1, n))

		seg, err := speakCue(ctx, st, c, i, n, vid, cfg, tmpDir)
		if err != nil {
			return nil, err
		}
		segs = append(segs, seg)
	}
	return segs, nil
}

// speakCue tổng hợp giọng cho 1 cue rồi chuẩn hoá về wav 44100 mono.
func speakCue(ctx context.Context, st *store.Store, c cue, i, n int, vid string, cfg Config, tmpDir string) (segment, error) {
	raw := filepath.Join(tmpDir, fmt.Sprintf("cue-%04d.wav", i+1))
	if err := tts.Speak(ctx, st, c.Text, vid, 0, cfg.Engine, raw); err != nil {
		return segment{}, fmt.Errorf("đọc câu %d/%d (%s): %w", i+1, n, shortText(c.Text, 40), err)
	}
	info, err := media.Probe(raw)
	if err != nil {
		return segment{}, fmt.Errorf("đo thời lượng câu %d/%d: %w", i+1, n, err)
	}
	if info.Duration <= 0 {
		return segment{}, fmt.Errorf("câu %d/%d (%s): giọng đọc tạo ra file rỗng", i+1, n, shortText(c.Text, 40))
	}

	seg := filepath.Join(tmpDir, fmt.Sprintf("seg-%04d.wav", i+1))
	if err := normalize(ctx, raw, seg, fitSpeed(info.Duration, c.Slot(), cfg)); err != nil {
		return segment{}, fmt.Errorf("chuẩn hoá audio câu %d/%d: %w", i+1, n, err)
	}
	out, err := media.Probe(seg)
	if err != nil {
		return segment{}, fmt.Errorf("đo thời lượng câu %d/%d sau khi ép nhịp: %w", i+1, n, err)
	}
	return segment{Path: seg, Dur: out.Duration, Start: c.Start}, nil
}

// voiceID ghép style vào giọng theo quy ước VieNeu ("Tên@style", "clone:<id>@style").
func voiceID(cfg Config) string {
	v := strings.TrimSpace(cfg.Voice)
	style := strings.TrimSpace(cfg.Style)
	if v == "" || style == "" || strings.Contains(v, "@") {
		return v
	}
	return v + "@" + style
}

// fitSpeed tính hệ số tăng tốc để câu đọc vừa slot phụ đề (1 = giữ nguyên).
func fitSpeed(dur, slot float64, cfg Config) float64 {
	if !cfg.FitTiming || slot < minFitSlot || dur <= slot {
		return 1
	}
	speed := dur / slot
	if m := maxSpeed(cfg); speed > m {
		speed = m
	}
	if speed < 1.01 {
		return 1
	}
	return speed
}

// normalize chuyển đoạn audio về wav 44100 mono (chuẩn chung để concat),
// kèm chuỗi atempo khi cần tăng tốc.
func normalize(ctx context.Context, src, dst string, speed float64) error {
	args := []string{"-y", "-i", src}
	if speed > 1.001 {
		args = append(args, "-filter:a", tempoChain(speed))
	}
	args = append(args, "-vn", "-ar", strconv.Itoa(segRate), "-ac", "1", "-c:a", "pcm_s16le", dst)
	return runFFmpeg(ctx, args...)
}

// tempoChain sinh chuỗi atempo nhiều tầng vì mỗi tầng ffmpeg chỉ nhận tối đa 2.0.
func tempoChain(speed float64) string {
	var parts []string
	remain := speed
	for remain > tempoLayerMax+1e-9 {
		parts = append(parts, fmt.Sprintf("atempo=%.4f", tempoLayerMax))
		remain /= tempoLayerMax
	}
	if remain > 1.0001 {
		parts = append(parts, fmt.Sprintf("atempo=%.4f", remain))
	}
	if len(parts) == 0 {
		return "atempo=1.0"
	}
	return strings.Join(parts, ",")
}
