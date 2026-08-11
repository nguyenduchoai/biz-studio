// Package highlight rút một video dài thành clip ngắn: chọn những đoạn đáng
// giữ nhất rồi cắt ghép lại theo đúng thứ tự thời gian.
package highlight

import (
	"strings"

	"bizstudio/internal/whisper"
)

// Candidate — một đoạn ứng viên, ranh giới lấy theo câu trong bản bóc băng.
type Candidate struct {
	Index int     `json:"index"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
	Score float64 `json:"score"` // 0..10, do AI chấm; 0 = chưa chấm
	Why   string  `json:"why"`   // lý do AI chọn, hiện cho người dùng đọc
}

// Dur trả thời lượng đoạn.
func (c Candidate) Dur() float64 { return c.End - c.Start }

const (
	// minCandidate — đoạn ngắn hơn mức này ghép vào clip chỉ thành một mẩu cụt
	// nghe hụt hơi, không thành câu.
	minCandidate = 1.2
	// maxCandidate — đoạn dài hơn thì gộp tiếp là chiếm hết chỗ của clip ngắn.
	maxCandidate = 18.0
	// joinGap — hai câu cách nhau dưới mức này thì gộp làm một ý, vì cắt giữa
	// hai câu liền hơi là nghe như bị hụt.
	joinGap = 0.45
)

// Candidates chia bản bóc băng thành các đoạn ứng viên.
//
// Ranh giới lấy theo CÂU chứ không theo giây đều: cắt ở giây thứ 15 chẵn thì
// rất dễ rơi vào giữa một câu, ghép xong người xem nghe câu cụt đầu cụt đuôi.
// Câu quá ngắn được gộp với câu liền sau cho đủ một ý trọn vẹn.
func Candidates(tr *whisper.Transcript) []Candidate {
	if tr == nil {
		return nil
	}
	var out []Candidate
	var cur *Candidate
	for _, seg := range tr.Segments {
		txt := strings.TrimSpace(seg.Text)
		if txt == "" || seg.End <= seg.Start {
			continue
		}
		// gộp tiếp vào đoạn đang dựng nếu còn ngắn và nằm liền kề
		if cur != nil && seg.Start-cur.End <= joinGap && cur.Dur() < minCandidate {
			cur.End = seg.End
			cur.Text = strings.TrimSpace(cur.Text + " " + txt)
			continue
		}
		if cur != nil {
			out = append(out, *cur)
		}
		cur = &Candidate{Start: seg.Start, End: seg.End, Text: txt}
	}
	if cur != nil {
		out = append(out, *cur)
	}

	// Bỏ đoạn quá vụn còn sót và cắt bớt đoạn quá dài. Không gộp thêm nữa: gộp
	// tham lam sẽ nuốt cả đoạn dài vào một ứng viên khổng lồ, làm AI mất khả
	// năng chọn lọc.
	kept := out[:0]
	for i := range out {
		c := out[i]
		if c.Dur() < minCandidate*0.5 {
			continue
		}
		if c.Dur() > maxCandidate {
			c.End = c.Start + maxCandidate
		}
		c.Index = len(kept)
		kept = append(kept, c)
	}
	return kept
}

// TotalDur cộng thời lượng của một danh sách đoạn.
func TotalDur(cs []Candidate) float64 {
	var t float64
	for _, c := range cs {
		t += c.Dur()
	}
	return t
}
