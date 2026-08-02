// Package qc — QC tự động: đo loudness, phát hiện frame đen, đứng hình, khoảng lặng.
package qc

import (
	"context"
	"fmt"

	"bizstudio/internal/media"
	"bizstudio/internal/util"
)

// Span — một khoảng thời gian bất thường [start, end] tính bằng giây.
type Span struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// Report — kết quả QC tự động của một video.
type Report struct {
	DurationS    float64  `json:"durationS"`
	Width        int      `json:"width"`
	Height       int      `json:"height"`
	LoudnessLUFS float64  `json:"loudnessLufs"`
	BlackSpans   []Span   `json:"blackSpans"`
	FreezeSpans  []Span   `json:"freezeSpans"`
	SilenceSpans []Span   `json:"silenceSpans"`
	Warnings     []string `json:"warnings"`
}

// Run phân tích video bằng 1 lệnh ffmpeg (ebur128 + silencedetect + blackdetect + freezedetect).
func Run(ctx context.Context, videoPath string) (Report, error) {
	rep := Report{
		BlackSpans:   []Span{},
		FreezeSpans:  []Span{},
		SilenceSpans: []Span{},
		Warnings:     []string{},
	}
	info, err := media.Probe(videoPath)
	if err != nil {
		return rep, fmt.Errorf("không probe được video: %w", err)
	}
	rep.DurationS, rep.Width, rep.Height = info.Duration, info.Width, info.Height

	stderr, audioOK, err := analyze(ctx, videoPath)
	if err != nil {
		return rep, err
	}
	rep.BlackSpans = parseBlack(stderr)
	rep.FreezeSpans = parseFreeze(stderr, info.Duration)

	lufsFound := false
	if audioOK {
		for _, s := range media.ParseSilences(stderr, info.Duration) {
			rep.SilenceSpans = append(rep.SilenceSpans, Span{Start: s.Start, End: s.End})
		}
		rep.LoudnessLUFS, lufsFound = parseLUFS(stderr)
	} else {
		rep.Warnings = append(rep.Warnings, "Không phân tích được âm thanh (video có thể không có audio)")
	}
	rep.Warnings = append(rep.Warnings, buildWarnings(&rep, audioOK, lufsFound)...)
	return rep, nil
}

// analyze chạy ffmpeg với đủ 4 filter; nếu lỗi (thường do không có audio) thì
// thử lại chỉ với filter video.
func analyze(ctx context.Context, path string) (stderr string, audioOK bool, err error) {
	const vf = "blackdetect=d=0.5:pix_th=0.10,freezedetect=n=-60dB:d=2"
	const af = "ebur128,silencedetect=noise=-40dB:d=1.5"
	_, se, runErr := util.RunErr(ctx, "ffmpeg", "-hide_banner", "-i", path,
		"-af", af, "-vf", vf, "-f", "null", "-")
	if runErr == nil {
		return se, true, nil
	}
	_, se2, runErr2 := util.RunErr(ctx, "ffmpeg", "-hide_banner", "-i", path,
		"-vf", vf, "-an", "-f", "null", "-")
	if runErr2 != nil {
		return "", false, fmt.Errorf("ffmpeg phân tích QC thất bại: %w — %s", runErr, tail(se, 400))
	}
	return se2, false, nil
}

// buildWarnings sinh cảnh báo tiếng Việt từ số liệu QC.
func buildWarnings(r *Report, audioOK, lufsFound bool) []string {
	var w []string
	if audioOK {
		if !lufsFound {
			w = append(w, "Không đo được âm lượng tổng (LUFS)")
		} else if r.LoudnessLUFS < -18 || r.LoudnessLUFS > -12 {
			w = append(w, fmt.Sprintf(
				"Âm lượng %.1f LUFS nằm ngoài khoảng khuyến nghị [-18, -12] LUFS", r.LoudnessLUFS))
		}
	}
	if n := len(r.BlackSpans); n > 0 {
		w = append(w, fmt.Sprintf("Có %d đoạn frame đen", n))
	}
	if n := len(r.FreezeSpans); n > 0 {
		w = append(w, fmt.Sprintf("Có %d đoạn đứng hình", n))
	}
	long := 0
	for _, s := range r.SilenceSpans {
		if s.End-s.Start > 3 {
			long++
		}
	}
	if long > 0 {
		w = append(w, fmt.Sprintf("Có %d đoạn im lặng dài hơn 3 giây", long))
	}
	return w
}
