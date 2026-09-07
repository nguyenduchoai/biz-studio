// Package qc — QC tự động: đo loudness, phát hiện frame đen, đứng hình, khoảng lặng.
package qc

import (
	"context"
	"fmt"
	"time"

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
	SourceSHA256 string    `json:"sourceSha256"`
	CheckedAt    time.Time `json:"checkedAt"`
	DurationS    float64   `json:"durationS"`
	Width        int       `json:"width"`
	Height       int       `json:"height"`
	LoudnessLUFS float64   `json:"loudnessLufs"`
	BlackSpans   []Span    `json:"blackSpans"`
	FreezeSpans  []Span    `json:"freezeSpans"`
	SilenceSpans []Span    `json:"silenceSpans"`
	Warnings     []string  `json:"warnings"`
}

// Run always checks the current file. Decode/analysis failures are blocking;
// black, frozen or silent content remains a warning because it may be intended.
// The fingerprint binds the report to exact bytes, not a reusable output path.
func Run(ctx context.Context, videoPath string) (Report, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	rep := Report{
		BlackSpans:   []Span{},
		FreezeSpans:  []Span{},
		SilenceSpans: []Span{},
		Warnings:     []string{},
	}
	videoPath, err := media.LocalVideoPath(videoPath)
	if err != nil {
		return rep, err
	}
	before, err := Fingerprint(ctx, videoPath)
	if err != nil {
		return rep, err
	}
	if err := media.ValidateVideo(ctx, videoPath); err != nil {
		return rep, fmt.Errorf("video chưa đạt kiểm tra tính toàn vẹn: %w", err)
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
	after, err := Fingerprint(ctx, videoPath)
	if err != nil {
		return rep, err
	}
	if before != after {
		return rep, fmt.Errorf("video đã thay đổi trong lúc kiểm tra; chờ render xong rồi chạy QC lại")
	}
	rep.SourceSHA256, rep.CheckedAt = after, time.Now().UTC()
	return rep, nil
}

// Do not retry a failed audio analysis as video-only: that used to hide damaged
// audio or missing filters. Files intentionally without audio use video-only QC.
func analyze(ctx context.Context, path string) (stderr string, audioOK bool, err error) {
	const vf = "blackdetect=d=0.5:pix_th=0.10,freezedetect=n=-60dB:d=2"
	// Per-frame EBU logs are verbose; keep only its final summary at info level.
	const af = "ebur128=framelog=verbose,silencedetect=noise=-40dB:d=1.5"
	audioOK = media.HasAudio(ctx, path)
	args := []string{"-nostdin", "-hide_banner", "-nostats", "-xerror", "-err_detect", "explode",
		"-protocol_whitelist", "file", "-i", path, "-map", "0:V:0", "-vf", vf}
	if audioOK {
		args = append(args, "-map", "0:a:0", "-af", af)
	} else {
		args = append(args, "-an")
	}
	_, se, runErr := util.RunErr(ctx, "ffmpeg", append(args, "-f", "null", "-")...)
	if runErr != nil {
		return "", false, fmt.Errorf("ffmpeg phân tích QC thất bại: %w — %s", runErr, tail(se, 400))
	}
	return se, audioOK, nil
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
