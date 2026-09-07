package media

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"bizstudio/internal/util"
)

// ValidateVideo checks the file actually delivered to the user, not the exit
// status of its producer. Metadata alone misses damaged/truncated media, so a
// successful full decode with at least one video frame is required as well.
// The caller's cancellation/deadline always wins over this safety ceiling.
func ValidateVideo(ctx context.Context, path string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	abs, err := LocalVideoPath(path)
	if err != nil {
		return err
	}
	var probe struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			Index       int    `json:"index"`
			CodecType   string `json:"codec_type"`
			Width       int    `json:"width"`
			Height      int    `json:"height"`
			Disposition struct {
				AttachedPic int `json:"attached_pic"`
			} `json:"disposition"`
		} `json:"streams"`
	}
	probeCtx, probeCancel := context.WithTimeout(ctx, 30*time.Second)
	out, stderr, err := util.RunErr(probeCtx, "ffprobe", "-v", "error", "-protocol_whitelist", "file",
		"-show_format", "-show_streams", "-of", "json", abs)
	probeErr := probeCtx.Err()
	probeCancel()
	if probeErr != nil {
		return fmt.Errorf("kiểm tra video hết thời gian hoặc bị huỷ: %w", probeErr)
	}
	if err != nil {
		return fmt.Errorf("không đọc được video: %w — %s", err, validationTail(stderr))
	}
	if err := json.Unmarshal([]byte(out), &probe); err != nil {
		return fmt.Errorf("thông tin video không hợp lệ: %w", err)
	}
	duration, err := strconv.ParseFloat(probe.Format.Duration, 64)
	if err != nil || math.IsNaN(duration) || math.IsInf(duration, 0) || duration <= 0 {
		return fmt.Errorf("video không có thời lượng hữu hạn lớn hơn 0")
	}
	videoIndex := -1
	for _, stream := range probe.Streams {
		if stream.CodecType == "video" && stream.Disposition.AttachedPic == 0 && stream.Width > 0 && stream.Height > 0 {
			videoIndex = stream.Index
			break
		}
	}
	if videoIndex < 0 {
		return fmt.Errorf("file không có luồng video với kích thước hợp lệ (ảnh bìa audio không phải video)")
	}
	out, stderr, err = util.RunErr(ctx, "ffmpeg", "-nostdin", "-hide_banner", "-v", "error", "-xerror",
		"-err_detect", "explode", "-protocol_whitelist", "file", "-i", abs,
		"-map", fmt.Sprintf("0:%d", videoIndex), "-map", "0:a?", "-progress", "pipe:1", "-nostats", "-f", "null", "-")
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("giải mã kiểm tra video hết thời gian hoặc bị huỷ: %w", err)
	}
	if err != nil {
		return fmt.Errorf("video hỏng hoặc không giải mã được toàn bộ: %w — %s", err, validationTail(stderr))
	}
	frames := 0
	for _, line := range strings.Split(out, "\n") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), "frame="); ok {
			if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && n > frames {
				frames = n
			}
		}
	}
	if frames == 0 {
		return fmt.Errorf("video không giải mã được khung hình nào")
	}
	return nil
}

// LocalVideoPath rejects protocols, devices and empty/non-regular files before
// invoking media tools. Absolute paths preserve literal colons in Unix names
// and Windows drive letters without interpreting them as FFmpeg protocols.
func LocalVideoPath(path string) (string, error) {
	if path == "" || strings.IndexByte(path, 0) >= 0 || strings.Contains(path, "://") ||
		(!filepath.IsAbs(path) && strings.Contains(strings.Split(filepath.ToSlash(path), "/")[0], ":")) {
		return "", fmt.Errorf("video phải là đường dẫn file cục bộ, không phải URL hay giao thức")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("đường dẫn video không hợp lệ: %w", err)
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("không mở được video: %w", err)
	}
	if !fi.Mode().IsRegular() || fi.Size() <= 0 {
		return "", fmt.Errorf("video phải là file thường và không rỗng")
	}
	return abs, nil
}

func validationTail(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 600 {
		return s[len(s)-600:]
	}
	return s
}
