package media

import (
	"context"
	"fmt"
	"log"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"bizstudio/internal/util"
	"bizstudio/internal/whisper"
)

// Mặc định cho AutoCutGuarded — đo thực tế trên giọng đọc tiếng Việt:
// âm cuối không hữu thanh (c/t/p/ch) rất nhỏ nên phải chừa đuôi rộng hơn đầu.
const (
	guardMinSilence = 0.6  // khoảng lặng ngắn hơn thì không đáng cắt
	guardPadStart   = 0.12 // giữ trước khi từ kế tiếp bắt đầu
	guardPadEnd     = 0.18 // giữ sau khi từ vừa rồi kết thúc
	guardMinKeep    = 0.25 // bỏ đoạn giữ vụn hơn mức này
	guardOffsetDb   = 18   // ngưỡng = mean_volume - 18dB
	guardMinDb      = -45  // kẹp dưới
	guardMaxDb      = -20  // kẹp trên
)

// ---------- AutoCut v2: ngưỡng tự đo + transcript bảo vệ ----------

// AutoCutOpt — tuỳ chọn cắt khoảng lặng có bảo vệ.
type AutoCutOpt struct {
	SilenceDb  float64 `json:"silenceDb"`  // 0 = TỰ ĐO theo mean_volume của chính file
	MinSilence float64 `json:"minSilence"` // 0 = 0.6s
	PadStart   float64 `json:"padStart"`   // 0 = 0.12s
	PadEnd     float64 `json:"padEnd"`     // 0 = 0.18s
	MinKeep    float64 `json:"minKeep"`    // 0 = 0.25s
}

// Report — kết quả một lần cắt, để UI nói rõ đã cắt bao nhiêu và theo ngưỡng nào.
type Report struct {
	ThresholdDb  float64 `json:"thresholdDb"`
	Guarded      bool    `json:"guarded"`      // có transcript bảo vệ hay không
	TotalSilence float64 `json:"totalSilence"` // tổng thời lượng khoảng lặng dò được
	CutSilence   float64 `json:"cutSilence"`   // thời lượng THỰC SỰ bị cắt
	Cuts         int     `json:"cuts"`         // số khoảng lặng bị cắt
	BeforeS      float64 `json:"beforeS"`
	AfterS       float64 `json:"afterS"`
}

// Summary — mô tả tiếng Việt gọn cho job detail.
func (r Report) Summary() string {
	guard := "KHÔNG có transcript bảo vệ"
	if r.Guarded {
		guard = "có transcript bảo vệ"
	}
	return fmt.Sprintf("Ngưỡng %.1f dB · %s · cắt %d đoạn (%.1fs/%.1fs im lặng) · %.1fs → %.1fs",
		r.ThresholdDb, guard, r.Cuts, r.CutSilence, r.TotalSilence, r.BeforeS, r.AfterS)
}

// AutoCutGuarded cắt khoảng lặng AN TOÀN: ngưỡng đo theo chính file, và khoảng
// lặng nào chạm vào bất kỳ từ nào trong transcript thì KHÔNG cắt.
//
// Vì sao cần transcript: đo độ to không phân biệt được "đang ngừng" với "nói
// nhỏ" — âm cuối tiếng Việt (c/t/p/ch) không hữu thanh nên rơi xuống dưới
// ngưỡng và bị coi là im lặng; cắt theo độ to là cắt vào chữ.
// tr = nil → chỉ cắt khoảng lặng dài ≥ 2× MinSilence và ghi cảnh báo.
func AutoCutGuarded(ctx context.Context, src, dst string, tr *whisper.Transcript,
	opt AutoCutOpt, upd func(float64, string)) (Report, error) {

	if upd == nil {
		upd = func(float64, string) {}
	}
	opt = opt.withDefaults()
	rep := Report{Guarded: tr != nil && tr.WordCount() > 0}

	info, err := Probe(src)
	if err != nil {
		return rep, err
	}
	if info.Duration <= 0 {
		return rep, fmt.Errorf("file không có thời lượng hợp lệ: %s", src)
	}
	rep.BeforeS, rep.AfterS = info.Duration, info.Duration
	if err := ensureDir(dst); err != nil {
		return rep, err
	}
	if !HasAudio(ctx, src) {
		upd(90, "File không có âm thanh — giữ nguyên bản gốc")
		return rep, copyFile(src, dst)
	}

	// 1) Ngưỡng: thuộc tính của FILE, không phải hằng số.
	upd(5, "Bước 1/4: đo độ to trung bình để chọn ngưỡng…")
	rep.ThresholdDb = opt.SilenceDb
	if opt.SilenceDb == 0 {
		th, mean, err := MeasureSilenceDb(ctx, src)
		if err != nil {
			return rep, err
		}
		rep.ThresholdDb = th
		log.Printf("[autocut] %s: mean_volume %.1f dB → ngưỡng im lặng %.1f dB", src, mean, th)
		upd(15, fmt.Sprintf("Độ to trung bình %.1f dB → ngưỡng im lặng %.1f dB", mean, th))
	} else {
		log.Printf("[autocut] %s: dùng ngưỡng im lặng do người dùng đặt %.1f dB", src, rep.ThresholdDb)
	}

	// 2) Dò khoảng lặng.
	upd(25, "Bước 2/4: dò các khoảng lặng…")
	silences, err := detectSilences(ctx, src, rep.ThresholdDb, opt.MinSilence, info.Duration)
	if err != nil {
		return rep, err
	}
	for _, s := range silences {
		rep.TotalSilence += s.End - s.Start
	}
	if len(silences) == 0 {
		upd(90, fmt.Sprintf("Không có khoảng lặng nào ở ngưỡng %.1f dB — giữ nguyên bản gốc", rep.ThresholdDb))
		return rep, copyFile(src, dst)
	}

	// 3) Lọc bằng transcript (hoặc luật an toàn khi không có transcript).
	upd(40, "Bước 3/4: đối chiếu transcript để không cắt vào chữ…")
	var cuts []TimeSpan
	if rep.Guarded {
		cuts = safeCutRanges(silences, tr.Words(), opt)
		log.Printf("[autocut] %s: %d đoạn cắt được từ %d khoảng lặng, không chạm %d từ có mốc",
			src, len(cuts), len(silences), tr.WordCount())
	} else {
		cuts = looseCutRanges(silences, opt)
		log.Printf("[autocut] %s: CẢNH BÁO cắt không có transcript bảo vệ — chỉ cắt %d/%d khoảng lặng dài ≥ %.2fs",
			src, len(cuts), len(silences), 2*opt.MinSilence)
		upd(45, fmt.Sprintf("Cảnh báo: cắt KHÔNG có transcript bảo vệ — chỉ cắt %d khoảng lặng dài ≥ %.2fs (bóc băng trước để cắt sát hơn)",
			len(cuts), 2*opt.MinSilence))
	}

	keeps, cutSil := keepsFromCuts(cuts, info.Duration, opt.MinKeep)
	rep.Cuts, rep.CutSilence = len(cuts), cutSil
	// Chỉ bỏ qua khi KHÔNG còn đoạn nào để giữ. Một đoạn giữ duy nhất vẫn là
	// kết quả hợp lệ và rất hay gặp: bản thu thừa khoảng lặng ở đầu và cuối,
	// cắt xong còn đúng phần giữa. concat=n=1 của ffmpeg xử lý được ca này.
	if len(cuts) == 0 || len(keeps) == 0 {
		upd(90, "Không có khoảng lặng nào cắt được an toàn — giữ nguyên bản gốc")
		rep.Cuts, rep.CutSilence = 0, 0
		return rep, copyFile(src, dst)
	}

	// 4) Cắt và ghép.
	upd(55, fmt.Sprintf("Bước 4/4: cắt %d khoảng lặng, ghép %d đoạn…", len(cuts), len(keeps)))
	cut := cutAndConcat
	if !HasVideo(ctx, src) {
		cut = cutAndConcatAudio // file chỉ có tiếng (giọng đọc, podcast)
	}
	if err := cut(ctx, src, dst, keeps); err != nil {
		return rep, err
	}
	rep.AfterS = 0
	for _, k := range keeps {
		rep.AfterS += k.end - k.start
	}
	if out, err := Probe(dst); err == nil && out.Duration > 0 {
		rep.AfterS = out.Duration
	}
	upd(98, rep.Summary())
	log.Printf("[autocut] %s → %s: %s", src, dst, rep.Summary())
	return rep, nil
}

// withDefaults bù giá trị mặc định (0 = dùng mặc định).
func (o AutoCutOpt) withDefaults() AutoCutOpt {
	if o.MinSilence <= 0 {
		o.MinSilence = guardMinSilence
	}
	if o.PadStart <= 0 {
		o.PadStart = guardPadStart
	}
	if o.PadEnd <= 0 {
		o.PadEnd = guardPadEnd
	}
	if o.MinKeep <= 0 {
		o.MinKeep = guardMinKeep
	}
	return o
}

// HasVideo kiểm tra file có stream hình hay không (file chỉ có tiếng thì cắt
// bằng đường riêng, không dùng filter [0:v]).
func HasVideo(ctx context.Context, path string) bool {
	out, _, err := util.RunErr(ctx, "ffprobe", "-v", "error",
		"-select_streams", "v", "-show_entries", "stream=codec_type", "-of", "csv=p=0", path)
	return err == nil && strings.TrimSpace(out) != ""
}

var reMeanVolume = regexp.MustCompile(`mean_volume:\s*(-?[0-9.]+) dB`)

// MeasureSilenceDb đo mean_volume của file rồi trả (ngưỡng, mean).
// Ngưỡng = mean_volume - 18dB, kẹp trong [-45, -20]: ngưỡng cứng thì hoặc vô
// dụng (không tìm ra khoảng lặng nào) hoặc nuốt tiếng.
func MeasureSilenceDb(ctx context.Context, src string) (float64, float64, error) {
	_, se, err := util.RunErr(ctx, "ffmpeg", "-hide_banner", "-i", src,
		"-af", "volumedetect", "-f", "null", "-")
	if err != nil {
		return 0, 0, fmt.Errorf("đo độ to thất bại: %w — %s", err, tail(se, 400))
	}
	m := reMeanVolume.FindStringSubmatch(se)
	if m == nil {
		return 0, 0, fmt.Errorf("không đọc được mean_volume từ ffmpeg — %s", tail(se, 300))
	}
	mean, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, 0, fmt.Errorf("mean_volume không hợp lệ: %s", m[1])
	}
	th := math.Round((mean-guardOffsetDb)*10) / 10
	if th < guardMinDb {
		th = guardMinDb
	}
	if th > guardMaxDb {
		th = guardMaxDb
	}
	return th, mean, nil
}

// detectSilences chạy silencedetect với ngưỡng + độ dài tối thiểu cho trước.
func detectSilences(ctx context.Context, src string, db, minSil, dur float64) ([]TimeSpan, error) {
	_, se, err := util.RunErr(ctx, "ffmpeg", "-hide_banner", "-i", src,
		"-af", fmt.Sprintf("silencedetect=noise=%.1fdB:d=%.2f", db, minSil),
		"-f", "null", "-")
	if err != nil {
		return nil, fmt.Errorf("phân tích khoảng lặng thất bại: %w — %s", err, tail(se, 400))
	}
	return ParseSilences(se, dur), nil
}

// minRemove — đoạn cắt ngắn hơn mức này thì không đáng cắt (chỉ tạo điểm nối).
const minRemove = 0.15

// WordGuards dựng vùng BẢO VỆ quanh từng từ: [word.Start-padStart, word.End+padEnd].
// Đây là vùng tuyệt đối không được cắt — nơi chứa âm cuối tiếng Việt (c/t/p/ch)
// vốn không hữu thanh nên máy đo độ to tưởng là im lặng.
func WordGuards(words []whisper.Word, padStart, padEnd float64) []TimeSpan {
	guards := make([]TimeSpan, 0, len(words))
	for _, w := range words {
		if w.End <= w.Start {
			continue
		}
		s := w.Start - padStart
		if s < 0 {
			s = 0
		}
		guards = append(guards, TimeSpan{Start: s, End: w.End + padEnd})
	}
	return mergeSpans(guards)
}

// safeCutRanges trả các đoạn ĐƯỢC PHÉP bỏ đi: phần khoảng lặng nằm NGOÀI mọi
// vùng bảo vệ của từ. Phần khoảng lặng chạm vào từ thì giữ lại nguyên vẹn —
// đây chính là thứ chặn việc nuốt âm cuối tiếng Việt.
func safeCutRanges(silences []TimeSpan, words []whisper.Word, opt AutoCutOpt) []TimeSpan {
	guards := WordGuards(words, opt.PadStart, opt.PadEnd)
	var out []TimeSpan
	for _, s := range silences {
		for _, piece := range subtractSpans(s, guards) {
			if piece.End-piece.Start >= minRemove {
				out = append(out, piece)
			}
		}
	}
	return out
}

// looseCutRanges — không có transcript: chỉ dám cắt khoảng lặng dài ≥ 2× MinSilence,
// và vẫn chừa đệm hai đầu (không biết từ nằm đâu nên phải rộng tay).
func looseCutRanges(silences []TimeSpan, opt AutoCutOpt) []TimeSpan {
	minLen := 2 * opt.MinSilence
	var out []TimeSpan
	for _, s := range silences {
		if s.End-s.Start < minLen {
			continue
		}
		piece := TimeSpan{Start: s.Start + opt.PadEnd, End: s.End - opt.PadStart}
		if piece.End-piece.Start >= minRemove {
			out = append(out, piece)
		}
	}
	return out
}

// subtractSpans trả các phần của s không nằm trong bất kỳ khoảng nào của
// guards (guards đã sắp xếp + gộp).
func subtractSpans(s TimeSpan, guards []TimeSpan) []TimeSpan {
	var out []TimeSpan
	cur := s.Start
	i := sort.Search(len(guards), func(i int) bool { return guards[i].End > s.Start })
	for ; i < len(guards) && guards[i].Start < s.End; i++ {
		if guards[i].Start > cur {
			out = append(out, TimeSpan{Start: cur, End: guards[i].Start})
		}
		if guards[i].End > cur {
			cur = guards[i].End
		}
	}
	if cur < s.End {
		out = append(out, TimeSpan{Start: cur, End: s.End})
	}
	return out
}

// mergeSpans sắp xếp + gộp các khoảng chồng nhau (để so trùng nhanh, đúng).
func mergeSpans(in []TimeSpan) []TimeSpan {
	if len(in) == 0 {
		return nil
	}
	cp := append([]TimeSpan(nil), in...)
	sort.Slice(cp, func(i, j int) bool { return cp[i].Start < cp[j].Start })
	out := []TimeSpan{cp[0]}
	for _, s := range cp[1:] {
		last := &out[len(out)-1]
		if s.Start <= last.End {
			if s.End > last.End {
				last.End = s.End
			}
			continue
		}
		out = append(out, s)
	}
	return out
}

// keepsFromCuts lấy phần bù của các đoạn bị cắt → danh sách đoạn GIỮ lại,
// bỏ những mẩu vụn ngắn hơn minKeep. Trả kèm tổng thời lượng đã bỏ đi.
func keepsFromCuts(cuts []TimeSpan, dur, minKeep float64) ([]segment, float64) {
	var keeps []segment
	var removed float64
	cur := 0.0
	for _, c := range cuts {
		start, end := c.Start, c.End
		if start < cur {
			start = cur
		}
		if end > dur {
			end = dur
		}
		if end <= start {
			continue
		}
		if start-cur >= minKeep {
			keeps = append(keeps, segment{cur, start})
		}
		removed += end - start
		cur = end
	}
	if dur-cur >= minKeep {
		keeps = append(keeps, segment{cur, dur})
	}
	return keeps, removed
}

// cutAndConcatAudio — bản chỉ-âm-thanh của cutAndConcat (file không có hình).
func cutAndConcatAudio(ctx context.Context, src, dst string, keeps []segment) error {
	var b strings.Builder
	for i, k := range keeps {
		fmt.Fprintf(&b, "[0:a]atrim=start=%.3f:end=%.3f,asetpts=PTS-STARTPTS[a%d];", k.start, k.end, i)
	}
	for i := range keeps {
		fmt.Fprintf(&b, "[a%d]", i)
	}
	fmt.Fprintf(&b, "concat=n=%d:v=0:a=1[aout]", len(keeps))

	args := []string{"-y", "-hide_banner", "-i", src,
		"-filter_complex", b.String(), "-map", "[aout]"}
	if strings.EqualFold(filepath.Ext(dst), ".wav") {
		args = append(args, "-c:a", "pcm_s16le")
	} else {
		args = append(args, "-c:a", "aac", "-b:a", "192k")
	}
	return run(ctx, append(args, dst)...)
}

// KeepSpans cắt lấy ĐÚNG các đoạn trong spans rồi nối lại theo thứ tự đưa vào.
//
// Ngược với AutoCutGuarded (bỏ khoảng lặng, giữ phần còn lại), hàm này giữ đúng
// những đoạn được chỉ định — dùng cho việc rút gọn video dài thành clip ngắn.
// Dùng chung đúng bộ lọc trim/concat của autocut nên chất lượng và cách xử lý
// tiếng giống hệt, không phải một đường ống thứ hai đi lệch dần.
func KeepSpans(ctx context.Context, src, dst string, spans []TimeSpan) error {
	if len(spans) == 0 {
		return fmt.Errorf("không có đoạn nào để giữ")
	}
	if err := ensureDir(dst); err != nil {
		return err
	}
	keeps := make([]segment, 0, len(spans))
	for _, s := range spans {
		if s.End > s.Start {
			keeps = append(keeps, segment{s.Start, s.End})
		}
	}
	if len(keeps) == 0 {
		return fmt.Errorf("mọi đoạn đều rỗng hoặc lộn đầu đuôi")
	}
	if HasVideo(ctx, src) {
		return cutAndConcat(ctx, src, dst, keeps)
	}
	return cutAndConcatAudio(ctx, src, dst, keeps)
}
