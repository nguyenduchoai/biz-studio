// Package broll ghép các clip tư liệu thành một dải hình khớp đúng độ dài của
// một bản thu tiếng — kiểu video "đọc trên nền tư liệu" hay gặp ở kênh tin tức,
// kiến thức, review.
//
// Khác với dựng theo cảnh (mỗi cảnh một hình), ở đây tiếng là thứ dẫn: hình cắt
// và nối cho vừa tiếng, không phải ngược lại. Tiếng đã thu rồi, kéo dài hay cắt
// bớt tiếng là hỏng lời đọc.
package broll

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"bizstudio/internal/media"
)

const (
	// defaultMaxClip — mỗi mẩu hình tối đa bao nhiêu giây. Để một clip chạy suốt
	// cả video thì người xem chán ngay; cắt vụn quá thì nhức mắt. 5 giây là mức
	// hay gặp ở video ngắn.
	defaultMaxClip = 5.0
	// minPiece — mẩu ngắn hơn mức này chỉ loé lên rồi mất, đọc ra là lỗi dựng.
	minPiece = 1.2
	// tailMargin — dựng dư ra một chút rồi cắt đúng bằng tiếng. Thiếu dù một
	// khung là cuối video đen sì trong lúc vẫn còn tiếng nói.
	tailMargin = 0.25
)

// Opt — tuỳ chọn ghép.
type Opt struct {
	MaxClipSec float64 // 0 = 5s
	Width      int     // 0 = giữ theo clip đầu tiên
	Height     int
	FPS        int  // 0 = 30
	Shuffle    bool // xáo thứ tự mẩu (vẫn tất định, xem seedShuffle)
}

func (o Opt) withDefaults() Opt {
	if o.MaxClipSec <= 0 {
		o.MaxClipSec = defaultMaxClip
	}
	if o.FPS <= 0 {
		o.FPS = 30
	}
	return o
}

// Piece — một mẩu cắt ra từ một clip tư liệu.
type Piece struct {
	Src   string  `json:"src"`
	Start float64 `json:"start"`
	Dur   float64 `json:"dur"`
}

// Report — kết quả ghép, để nói cho người dùng biết máy đã làm gì.
type Report struct {
	AudioSec  float64 `json:"audioSec"`
	VideoSec  float64 `json:"videoSec"`
	Pieces    int     `json:"pieces"`
	Clips     int     `json:"clips"`  // số clip nguồn thật sự dùng tới
	Reused    int     `json:"reused"` // số vòng phải quay lại dùng lại tư liệu
	Width     int     `json:"width"`
	Height    int     `json:"height"`
	ShortFall bool    `json:"shortFall"` // tư liệu quá ít, đã phải lặp lại
}

// Assemble cắt các clip tư liệu thành mẩu, nối cho đủ dài rồi lồng tiếng vào.
//
// KHÔNG đụng tới file nguồn. Tiếng được giữ nguyên, hình cắt đúng bằng tiếng.
func Assemble(ctx context.Context, clips []string, audioPath, dst string, opt Opt) (*Report, error) {
	opt = opt.withDefaults()
	if len(clips) == 0 {
		return nil, fmt.Errorf("chưa có clip tư liệu nào")
	}
	ainfo, err := media.Probe(audioPath)
	if err != nil {
		return nil, fmt.Errorf("đọc file tiếng thất bại: %w", err)
	}
	if ainfo.Duration <= 0 {
		return nil, fmt.Errorf("file tiếng không có thời lượng hợp lệ: %s", audioPath)
	}
	need := ainfo.Duration + tailMargin

	pieces, w, h, err := cutPieces(clips, opt)
	if err != nil {
		return nil, err
	}
	if len(pieces) == 0 {
		return nil, fmt.Errorf("không cắt được mẩu nào từ %d clip — file có phải video không?", len(clips))
	}
	if opt.Width > 0 && opt.Height > 0 {
		w, h = opt.Width, opt.Height
	}
	if opt.Shuffle {
		seedShuffle(pieces)
	}

	seq, reused := fill(pieces, need)
	rep := &Report{
		AudioSec: ainfo.Duration, Pieces: len(seq), Reused: reused,
		Width: w, Height: h, ShortFall: reused > 0,
	}
	seen := map[string]bool{}
	for _, p := range seq {
		seen[p.Src] = true
	}
	rep.Clips = len(seen)

	tmp, err := os.MkdirTemp("", "broll")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	if err := renderPieces(ctx, seq, tmp, w, h, opt.FPS); err != nil {
		return nil, err
	}
	silent := filepath.Join(tmp, "noimg.mp4")
	if err := concatFiles(ctx, len(seq), tmp, silent); err != nil {
		return nil, err
	}
	if err := muxAudio(ctx, silent, audioPath, dst, ainfo.Duration); err != nil {
		return nil, err
	}
	if out, err := media.Probe(dst); err == nil && out.Duration > 0 {
		rep.VideoSec = out.Duration
	}
	return rep, nil
}

// cutPieces chia mỗi clip thành các mẩu ≤ MaxClipSec, bỏ phần đuôi quá vụn.
// Trả kèm kích thước khung của clip ĐẦU TIÊN đọc được, làm mặc định khi người
// dùng không chỉ định.
func cutPieces(clips []string, opt Opt) ([]Piece, int, int, error) {
	var out []Piece
	var w, h int
	var loi []string
	for _, c := range clips {
		info, err := media.Probe(c)
		if err != nil || info.Duration <= 0 {
			loi = append(loi, filepath.Base(c))
			continue
		}
		if w == 0 && info.Width > 0 {
			w, h = info.Width, info.Height
		}
		for s := 0.0; s < info.Duration; s += opt.MaxClipSec {
			d := math.Min(opt.MaxClipSec, info.Duration-s)
			if d < minPiece {
				break // đuôi vụn: thà bỏ còn hơn để một mẩu loé lên rồi mất
			}
			out = append(out, Piece{Src: c, Start: s, Dur: d})
		}
	}
	if len(out) == 0 && len(loi) > 0 {
		return nil, 0, 0, fmt.Errorf("không đọc được clip nào (%s)", strings.Join(loi, ", "))
	}
	if w <= 0 {
		w, h = 1080, 1920
	}
	return out, w, h, nil
}

// fill lấy mẩu cho tới khi đủ dài, LẦN LƯỢT XOAY VÒNG QUA TỪNG CLIP NGUỒN.
//
// Lấy tuần tự hết clip này mới sang clip kia là sai: đo thật với 4 clip cho một
// video 17 giây thì chỉ 2 clip đầu được dùng, hai clip sau không bao giờ xuất
// hiện — người dùng đưa 4 file mà chỉ thấy 2. Xoay vòng thì mọi clip đều lên
// hình trước khi phải dùng lại bất cứ mẩu nào.
//
// Hết tư liệu thì quay lại từ đầu và đếm số vòng lặp lại — để báo cho người
// dùng biết tư liệu đang quá ít, chứ không lặng lẽ cho họ một video lặp đi lặp
// lại mà không hiểu vì sao.
func fill(pieces []Piece, need float64) ([]Piece, int) {
	// gom theo clip nguồn, giữ nguyên thứ tự gặp
	var order []string
	byClip := map[string][]Piece{}
	for _, p := range pieces {
		if _, ok := byClip[p.Src]; !ok {
			order = append(order, p.Src)
		}
		byClip[p.Src] = append(byClip[p.Src], p)
	}

	var seq []Piece
	var total float64
	idx := make([]int, len(order)) // đã lấy tới mẩu thứ mấy của từng clip
	reused, round := 0, 0
	for total < need {
		lay := false
		for c := range order {
			if total >= need {
				break
			}
			ps := byClip[order[c]]
			if idx[c] >= len(ps) {
				continue // clip này hết mẩu, chờ vòng lặp lại
			}
			seq = append(seq, ps[idx[c]])
			total += ps[idx[c]].Dur
			idx[c]++
			lay = true
		}
		if !lay {
			// mọi clip đã cạn mẩu → quay lại từ đầu
			for i := range idx {
				idx[i] = 0
			}
			reused++
			if reused > 50 { // chặn vòng vô hạn khi tư liệu quá ngắn
				break
			}
		}
		round++
		if round > 5000 {
			break
		}
	}
	return seq, reused
}

// seedShuffle xáo mẩu bằng một dãy tất định (không dùng random thật): cùng bộ
// tư liệu thì dựng lại ra đúng video cũ. Dựng lại ra khác nhau mỗi lần là thứ
// không thể sửa lỗi được — người dùng báo "video bị lỗi ở giây 12" mà lần dựng
// sau giây 12 đã là cảnh khác.
func seedShuffle(p []Piece) {
	n := len(p)
	if n < 2 {
		return
	}
	// bước nhảy nguyên tố cùng nhau với n → đi hết mọi vị trí, không lặp
	step := 7
	for step%n == 0 || gcd(step, n) != 1 {
		step++
	}
	out := make([]Piece, 0, n)
	for i, k := 0, 0; i < n; i, k = i+1, (k+step)%n {
		out = append(out, p[k])
	}
	copy(p, out)
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// renderPieces cắt từng mẩu và đưa về đúng khung hình. Thêm viền chứ không bóp
// méo — tư liệu tải về đủ mọi tỉ lệ, bóp cho vừa là mặt người méo hết.
func renderPieces(ctx context.Context, seq []Piece, dir string, w, h, fps int) error {
	vf := fmt.Sprintf(
		"scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,setsar=1,fps=%d",
		w, h, w, h, fps)
	for i, p := range seq {
		out := filepath.Join(dir, fmt.Sprintf("p-%05d.mp4", i))
		// -ss TRƯỚC -i để nhảy nhanh, -t sau -i để cắt đúng độ dài
		if err := media.RunFFmpeg(ctx, "-y", "-hide_banner",
			"-ss", f3(p.Start), "-i", p.Src, "-t", f3(p.Dur),
			"-an", "-vf", vf,
			"-c:v", "libx264", "-crf", "20", "-preset", "veryfast", "-pix_fmt", "yuv420p",
			out); err != nil {
			return fmt.Errorf("cắt mẩu %d từ %s thất bại: %w", i+1, filepath.Base(p.Src), err)
		}
	}
	return nil
}

// concatFiles nối các mẩu bằng demuxer concat (copy, không mã hoá lại) — mọi
// mẩu đã cùng khung hình, cùng fps, cùng bộ mã nên nối thẳng được.
func concatFiles(ctx context.Context, n int, dir, dst string) error {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "file '%s'\n", filepath.Join(dir, fmt.Sprintf("p-%05d.mp4", i)))
	}
	list := filepath.Join(dir, "list.txt")
	if err := os.WriteFile(list, []byte(b.String()), 0o644); err != nil {
		return err
	}
	if err := media.RunFFmpeg(ctx, "-y", "-hide_banner", "-f", "concat", "-safe", "0",
		"-i", list, "-c", "copy", dst); err != nil {
		return fmt.Errorf("nối các mẩu thất bại: %w", err)
	}
	return nil
}

// muxAudio lồng tiếng vào và cắt hình đúng bằng tiếng.
func muxAudio(ctx context.Context, video, audio, dst string, dur float64) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return media.RunFFmpeg(ctx, "-y", "-hide_banner",
		"-i", video, "-i", audio,
		"-map", "0:v", "-map", "1:a",
		"-t", f3(dur), // cắt đúng bằng tiếng: hình dựng dư ra một chút ở trên
		"-c:v", "copy", "-c:a", "aac", "-b:a", "192k", "-ar", "48000",
		"-movflags", "+faststart", dst)
}

func f3(v float64) string { return fmt.Sprintf("%.3f", v) }

// ListClips liệt kê file video trong một thư mục, sắp theo tên để thứ tự dựng
// tất định (thứ tự đọc thư mục của hệ điều hành không bảo đảm).
func ListClips(dir string) ([]string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".mp4", ".mov", ".mkv", ".webm", ".m4v", ".avi":
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(out)
	return out, nil
}
