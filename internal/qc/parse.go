package qc

import (
	"bufio"
	"regexp"
	"strconv"
	"strings"
)

var (
	reLUFS        = regexp.MustCompile(`I:\s*(-?[0-9.]+)\s*LUFS`)
	reBlack       = regexp.MustCompile(`black_start:\s*(-?[0-9.]+)\s+black_end:\s*(-?[0-9.]+)`)
	reFreezeStart = regexp.MustCompile(`lavfi\.freezedetect\.freeze_start:\s*(-?[0-9.]+)`)
	reFreezeEnd   = regexp.MustCompile(`lavfi\.freezedetect\.freeze_end:\s*(-?[0-9.]+)`)
)

// parseLUFS lấy Integrated loudness — match cuối cùng chính là dòng Summary của ebur128.
func parseLUFS(stderr string) (float64, bool) {
	ms := reLUFS.FindAllStringSubmatch(stderr, -1)
	if len(ms) == 0 {
		return 0, false
	}
	v, err := strconv.ParseFloat(ms[len(ms)-1][1], 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// parseBlack đọc các cặp black_start/black_end của blackdetect.
func parseBlack(stderr string) []Span {
	out := []Span{}
	for _, m := range reBlack.FindAllStringSubmatch(stderr, -1) {
		s, err1 := strconv.ParseFloat(m[1], 64)
		e, err2 := strconv.ParseFloat(m[2], 64)
		if err1 == nil && err2 == nil && e > s {
			out = append(out, Span{Start: s, End: e})
		}
	}
	return out
}

// parseFreeze ghép cặp freeze_start/freeze_end của freezedetect.
// Đoạn đứng hình kéo tới hết file (không có freeze_end) được chốt tại dur.
func parseFreeze(stderr string, dur float64) []Span {
	out := []Span{}
	pending := -1.0
	sc := bufio.NewScanner(strings.NewReader(stderr))
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if m := reFreezeStart.FindStringSubmatch(line); m != nil {
			if v, err := strconv.ParseFloat(m[1], 64); err == nil {
				if v < 0 {
					v = 0
				}
				pending = v
			}
			continue
		}
		if m := reFreezeEnd.FindStringSubmatch(line); m != nil && pending >= 0 {
			if v, err := strconv.ParseFloat(m[1], 64); err == nil && v > pending {
				out = append(out, Span{Start: pending, End: v})
			}
			pending = -1
		}
	}
	if pending >= 0 && dur > pending {
		out = append(out, Span{Start: pending, End: dur})
	}
	return out
}

// tail lấy tối đa n ký tự cuối chuỗi (thông tin lỗi ffmpeg nằm ở cuối stderr).
func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return "…" + s[len(s)-n:]
	}
	return s
}
