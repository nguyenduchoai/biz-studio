package media

import (
	"bufio"
	"regexp"
	"strconv"
	"strings"
)

// TimeSpan — một khoảng thời gian [Start, End] tính bằng giây.
type TimeSpan struct {
	Start float64
	End   float64
}

var (
	reSilStart = regexp.MustCompile(`silence_start:\s*(-?[0-9.]+)`)
	reSilEnd   = regexp.MustCompile(`silence_end:\s*(-?[0-9.]+)`)
)

// ParseSilences parse stderr của ffmpeg silencedetect thành danh sách khoảng lặng.
// Khoảng lặng chưa đóng (kéo tới hết file) được chốt tại totalDur.
func ParseSilences(stderr string, totalDur float64) []TimeSpan {
	var out []TimeSpan
	pending := -1.0
	sc := bufio.NewScanner(strings.NewReader(stderr))
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if m := reSilStart.FindStringSubmatch(line); m != nil {
			if v, err := strconv.ParseFloat(m[1], 64); err == nil {
				if v < 0 {
					v = 0
				}
				pending = v
			}
			continue
		}
		if m := reSilEnd.FindStringSubmatch(line); m != nil && pending >= 0 {
			if v, err := strconv.ParseFloat(m[1], 64); err == nil && v > pending {
				out = append(out, TimeSpan{Start: pending, End: v})
			}
			pending = -1
		}
	}
	if pending >= 0 && totalDur > pending {
		out = append(out, TimeSpan{Start: pending, End: totalDur})
	}
	return out
}
