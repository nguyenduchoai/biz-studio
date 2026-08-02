// Package media — wrapper ffmpeg/ffprobe cho Biz Studio.
package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"bizstudio/internal/util"
)

// Info — thông tin cơ bản của file media (đọc bằng ffprobe).
type Info struct {
	Duration float64 `json:"duration"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	FPS      float64 `json:"fps"`
	Size     int64   `json:"size"`
}

type probeFormat struct {
	Duration string `json:"duration"`
	Size     string `json:"size"`
}

type probeStream struct {
	CodecType  string `json:"codec_type"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	RFrameRate string `json:"r_frame_rate"`
}

type probeResult struct {
	Format  probeFormat   `json:"format"`
	Streams []probeStream `json:"streams"`
}

// Probe đọc thông tin media (duration, kích thước, fps, dung lượng).
func Probe(path string) (Info, error) {
	if _, err := os.Stat(path); err != nil {
		return Info{}, fmt.Errorf("không tìm thấy file: %s", path)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, se, err := util.RunErr(ctx, "ffprobe",
		"-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", path)
	if err != nil {
		return Info{}, fmt.Errorf("ffprobe lỗi với %s: %w — %s", path, err, tail(se, 300))
	}
	var pr probeResult
	if err := json.Unmarshal([]byte(out), &pr); err != nil {
		return Info{}, fmt.Errorf("không đọc được output ffprobe: %w", err)
	}
	info := Info{}
	info.Duration, _ = strconv.ParseFloat(pr.Format.Duration, 64)
	info.Size, _ = strconv.ParseInt(pr.Format.Size, 10, 64)
	if info.Size == 0 {
		if st, statErr := os.Stat(path); statErr == nil {
			info.Size = st.Size()
		}
	}
	for _, s := range pr.Streams {
		if s.CodecType == "video" && info.Width == 0 {
			info.Width, info.Height = s.Width, s.Height
			info.FPS = parseFPS(s.RFrameRate)
		}
	}
	return info, nil
}

// HasAudio kiểm tra file có stream audio hay không.
func HasAudio(ctx context.Context, path string) bool {
	out, _, err := util.RunErr(ctx, "ffprobe", "-v", "error",
		"-select_streams", "a", "-show_entries", "stream=codec_type", "-of", "csv=p=0", path)
	return err == nil && strings.TrimSpace(out) != ""
}

// parseFPS đọc chuỗi r_frame_rate dạng "30000/1001" → 29.97.
func parseFPS(r string) float64 {
	r = strings.TrimSpace(r)
	if r == "" {
		return 0
	}
	if num, den, ok := strings.Cut(r, "/"); ok {
		n, err1 := strconv.ParseFloat(num, 64)
		d, err2 := strconv.ParseFloat(den, 64)
		if err1 != nil || err2 != nil || d == 0 {
			return 0
		}
		return n / d
	}
	v, err := strconv.ParseFloat(r, 64)
	if err != nil {
		return 0
	}
	return v
}
