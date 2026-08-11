package highlight

import (
	"context"
	"fmt"
	"sort"

	"bizstudio/internal/media"
	"bizstudio/internal/whisper"
)

const (
	// padStart / padEnd — chừa quanh mép cắt. Lấy đúng số của autocut vì cùng
	// một lý do đo được: âm cuối không hữu thanh của tiếng Việt (c/t/p/ch) rất
	// nhỏ, cắt sát là mất luôn phụ âm cuối, nghe như nuốt chữ.
	padStart = 0.12
	padEnd   = 0.18
	// mergeGap — hai đoạn được chọn cách nhau dưới mức này thì nối làm một, vì
	// một mối cắt chỉ dài hơn nửa giây nghe như tiếng nấc chứ không ra chuyển ý.
	mergeGap = 0.5
)

// Report — kết quả rút gọn, để báo cho người dùng biết máy đã làm gì.
type Report struct {
	SourceSec float64     `json:"sourceSec"`
	OutputSec float64     `json:"outputSec"`
	Kept      int         `json:"kept"`      // số đoạn giữ lại
	Total     int         `json:"total"`     // tổng số đoạn ứng viên
	Segments  []Candidate `json:"segments"`  // các đoạn đã giữ, theo thứ tự thời gian
	Truncated bool        `json:"truncated"` // đã phải cắt bớt đoạn cuối cho vừa trần
}

// Build cắt các đoạn đã chọn ra khỏi video gốc rồi nối lại.
//
// KHÔNG động vào file gốc: mọi thứ ghi ra dst. Video nguồn là thứ người dùng
// quay mất công nhất và thường không có bản sao.
func Build(ctx context.Context, src, dst string, tr *whisper.Transcript, chosen []Candidate, maxSec int) (*Report, error) {
	if len(chosen) == 0 {
		return nil, fmt.Errorf("chưa chọn được đoạn nào — thử hạ ngưỡng điểm hoặc tăng thời lượng đích")
	}
	info, err := media.Probe(src)
	if err != nil {
		return nil, err
	}
	if info.Duration <= 0 {
		return nil, fmt.Errorf("file không có thời lượng hợp lệ: %s", src)
	}

	spans := snapToWords(chosen, tr, info.Duration)
	spans = mergeClose(spans)

	rep := &Report{SourceSec: info.Duration, Total: len(chosen)}
	if maxSec > 0 {
		spans, rep.Truncated = capTotal(spans, float64(maxSec))
	}
	if len(spans) == 0 {
		return nil, fmt.Errorf("sau khi canh theo mốc từ thì không còn đoạn nào hợp lệ")
	}
	if err := media.KeepSpans(ctx, src, dst, spans); err != nil {
		return nil, fmt.Errorf("cắt ghép đoạn đã chọn thất bại: %w", err)
	}

	rep.Kept = len(spans)
	for _, s := range spans {
		rep.Segments = append(rep.Segments, Candidate{Start: s.Start, End: s.End, Text: textIn(tr, s)})
	}
	if out, err := media.Probe(dst); err == nil && out.Duration > 0 {
		rep.OutputSec = out.Duration
	}
	return rep, nil
}

// snapToWords nắn mép mỗi đoạn ra tới ranh giới TỪ gần nhất rồi chừa thêm đệm.
//
// Mốc câu của bản bóc băng không khớp chính xác mốc từ: cắt đúng theo mốc câu
// vẫn xén mất nửa âm đầu hoặc nửa âm cuối. Nới ra tới từ trọn vẹn rồi mới đệm.
func snapToWords(cs []Candidate, tr *whisper.Transcript, dur float64) []media.TimeSpan {
	words := tr.Words()
	out := make([]media.TimeSpan, 0, len(cs))
	for _, c := range cs {
		s, e := c.Start, c.End
		for _, w := range words {
			// từ nào chạm vào đoạn thì đoạn phải bao trọn từ đó
			if w.End > s && w.Start < e {
				if w.Start < s {
					s = w.Start
				}
				if w.End > e {
					e = w.End
				}
			}
		}
		s -= padStart
		e += padEnd
		if s < 0 {
			s = 0
		}
		if e > dur {
			e = dur
		}
		if e > s {
			out = append(out, media.TimeSpan{Start: s, End: e})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out
}

// mergeClose nối các đoạn dính nhau hoặc cách nhau rất gần.
func mergeClose(in []media.TimeSpan) []media.TimeSpan {
	if len(in) == 0 {
		return nil
	}
	out := []media.TimeSpan{in[0]}
	for _, s := range in[1:] {
		last := &out[len(out)-1]
		if s.Start-last.End <= mergeGap {
			if s.End > last.End {
				last.End = s.End
			}
			continue
		}
		out = append(out, s)
	}
	return out
}

// capTotal cắt bớt cho tổng thời lượng không vượt trần.
//
// Cắt từ ĐOẠN CUỐI trở đi, và cắt cụt đoạn cuối cùng thay vì bỏ hẳn nó: bỏ hẳn
// thì mất luôn cả phần đầu đoạn vốn vẫn vừa chỗ. Trần nền tảng là ranh giới
// cứng — quá một giây là video bị xếp sang loại khác, nên ở đây phải cắt.
func capTotal(spans []media.TimeSpan, maxSec float64) ([]media.TimeSpan, bool) {
	var total float64
	for i, s := range spans {
		d := s.End - s.Start
		if total+d <= maxSec {
			total += d
			continue
		}
		room := maxSec - total
		if room < minCandidate {
			return spans[:i], true // chỗ còn lại quá ngắn, không thành câu
		}
		cut := spans[:i+1]
		cut[i].End = cut[i].Start + room
		return cut, true
	}
	return spans, false
}

// textIn gom lời thoại rơi vào một khoảng thời gian, để báo cáo đọc được.
func textIn(tr *whisper.Transcript, s media.TimeSpan) string {
	if tr == nil {
		return ""
	}
	var parts []string
	for _, seg := range tr.Segments {
		if seg.End > s.Start && seg.Start < s.End {
			parts = append(parts, seg.Text)
		}
	}
	return oneLine(joinNonEmpty(parts))
}

func joinNonEmpty(ss []string) string {
	out := ""
	for _, s := range ss {
		if s == "" {
			continue
		}
		if out != "" {
			out += " "
		}
		out += s
	}
	return out
}
