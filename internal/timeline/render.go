package timeline

import (
	"fmt"
	"strings"
)

// Thông số né giọng, giống internal/media/ducking.go — nhạc lùi nhanh khi bắt
// đầu nói, nâng lại chậm để không giật cục giữa các từ.
const (
	duckThresh  = 0.03
	duckRatio   = 4.0
	duckAttack  = 20
	duckRelease = 400
)

// Plan — lệnh ffmpeg dựng từ một timeline. Tách khỏi lúc chạy để test được
// filtergraph mà không cần đụng tới ffmpeg hay file thật.
type Plan struct {
	Args   []string // đối số ffmpeg, chưa gồm đường dẫn đích
	Filter string   // filter_complex, để dễ đọc khi debug
	Note   string   // tóm tắt cho người dùng
}

// BuildPlan dựng lệnh ffmpeg trộn mọi lớp âm thanh và ghi phụ đề lên video nền.
//
// srcAudio cho biết video nền có tiếng gốc không: ffmpeg tham chiếu [0:a] cho
// một file không có stream âm thanh là lỗi cả lệnh, mà thông báo của nó
// ("Invalid file index") không hề gợi ý nguyên nhân.
//
// srtPath rỗng nghĩa là không ghi phụ đề — khi đó video được COPY thẳng, không
// mã hoá lại: nhanh hơn nhiều lần và không mất chất lượng.
func BuildPlan(d *Doc, srcAbs string, srcAudio bool, srtPath string) (*Plan, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	tracks := d.audible()

	args := []string{"-y", "-i", srcAbs}
	var fc []string   // các mệnh đề filter
	var buses []busIn // các bus chờ trộn ở bước cuối

	if srcAudio {
		// Tiếng gốc luôn là input 0; người dùng chỉnh nó qua lớp role=source.
		gain, muted := sourceTrackGain(d)
		if !muted {
			fc = append(fc, fmt.Sprintf("[0:a]volume=%.2fdB[src]", gain))
			buses = append(buses, busIn{label: "src", role: RoleSource})
		}
	}

	in := 1 // input 0 là video nền
	for ti, t := range tracks {
		if t.Role == RoleSource {
			continue // lớp source chỉ để chỉnh tiếng gốc, không có file riêng
		}
		var itemLabels []string
		for ii, it := range t.Items {
			lbl := fmt.Sprintf("i%d_%d", ti, ii)
			args = append(args, "-i", it.Path)
			fc = append(fc, itemFilter(in, lbl, it))
			itemLabels = append(itemLabels, lbl)
			in++
		}
		if len(itemLabels) == 0 {
			continue
		}
		bus := fmt.Sprintf("t%d", ti)
		fc = append(fc, mixInto(itemLabels, bus, t.Gain))
		buses = append(buses, busIn{label: bus, role: t.Role, duck: t.Duck})
	}

	if len(buses) == 0 {
		return nil, fmt.Errorf("timeline không có lớp âm thanh nào đang bật — bật một lớp hoặc bỏ tắt tiếng")
	}

	outLabel, more := duckAndMix(buses, &fc)
	fc = append(fc, more...)

	filter := strings.Join(fc, ";")
	args = append(args, "-filter_complex", filter)

	// Phụ đề ghi thẳng lên hình thì buộc phải mã hoá lại; không có phụ đề thì
	// copy stream hình, nhanh hơn nhiều lần và không mất chất lượng.
	if srtPath != "" {
		args = append(args, "-vf", subtitleFilter(srtPath, d.SubStyle),
			"-c:v", "libx264", "-preset", "veryfast", "-crf", "20")
	} else {
		args = append(args, "-c:v", "copy")
	}
	args = append(args, "-map", "0:v", "-map", "["+outLabel+"]",
		"-c:a", "aac", "-b:a", "192k",
		// Dừng theo video nền: đoạn nhạc kéo dài quá phim thì cắt, chứ không
		// để ra một file có mấy giây màn hình đen ở cuối.
		"-shortest")

	return &Plan{Args: args, Filter: filter, Note: summary(d, tracks, srtPath)}, nil
}

type busIn struct {
	label string
	role  string
	duck  bool
}

// itemFilter dựng chuỗi lọc cho một đoạn: cắt → về mốc 0 → âm lượng → fade →
// đẩy tới đúng vị trí trên timeline.
func itemFilter(input int, label string, it Item) string {
	var p []string
	if it.Out > it.In {
		p = append(p, fmt.Sprintf("atrim=start=%.3f:end=%.3f", it.In, it.Out))
	} else if it.In > 0 {
		p = append(p, fmt.Sprintf("atrim=start=%.3f", it.In))
	}
	// Sau atrim, mốc thời gian vẫn là mốc CŨ của file nguồn. Không đặt lại thì
	// adelay phía sau tính sai và đoạn rơi vào chỗ khác hẳn.
	p = append(p, "asetpts=PTS-STARTPTS")
	if it.Gain != 0 {
		p = append(p, fmt.Sprintf("volume=%.2fdB", it.Gain))
	}
	if it.FadeIn > 0 {
		p = append(p, fmt.Sprintf("afade=t=in:st=0:d=%.3f", it.FadeIn))
	}
	if it.FadeOut > 0 && it.Dur() > 0 {
		p = append(p, fmt.Sprintf("afade=t=out:st=%.3f:d=%.3f", it.Dur()-it.FadeOut, it.FadeOut))
	}
	if it.At > 0 {
		// all=1: không có thì adelay chỉ đẩy kênh trái, nghe thành lệch tiếng.
		p = append(p, fmt.Sprintf("adelay=%d:all=1", int(it.At*1000+0.5)))
	}
	return fmt.Sprintf("[%d:a]%s[%s]", input, strings.Join(p, ","), label)
}

// mixInto trộn các đoạn của một lớp thành một bus.
func mixInto(labels []string, bus string, gainDB float64) string {
	var head string
	for _, l := range labels {
		head += "[" + l + "]"
	}
	var body string
	if len(labels) == 1 {
		body = "anull"
	} else {
		// normalize=0: mặc định amix chia đều âm lượng theo số nguồn, nên thêm
		// một tiếng động là cả lớp tự nhỏ đi — người dùng không hiểu vì sao.
		body = fmt.Sprintf("amix=inputs=%d:normalize=0:dropout_transition=0", len(labels))
	}
	if gainDB != 0 {
		body += fmt.Sprintf(",volume=%.2fdB", gainDB)
	}
	return fmt.Sprintf("%s%s[%s]", head, body, bus)
}

// duckAndMix cho nhạc né lời đọc rồi trộn tất cả thành một đường ra.
//
// Nhạc để một mức cố định suốt video thì hoặc át lời, hoặc nhỏ tới mức vô
// nghĩa. Lấy lời đọc làm tín hiệu điều khiển thì nhạc to được mà lời vẫn rõ.
func duckAndMix(buses []busIn, fc *[]string) (string, []string) {
	var extra []string

	var narr, music, plain []string
	for _, b := range buses {
		switch {
		case b.role == RoleNarration:
			narr = append(narr, b.label)
		case b.role == RoleMusic && b.duck:
			music = append(music, b.label)
		default:
			plain = append(plain, b.label)
		}
	}

	// Không có lời đọc thì chẳng có gì để né — nhạc trộn thẳng như lớp thường.
	if len(narr) == 0 {
		plain = append(plain, music...)
		music = nil
	}

	var final []string
	if len(narr) > 0 {
		key := narr[0]
		if len(narr) > 1 {
			extra = append(extra, mixInto(narr, "narrbus", 0))
			key = "narrbus"
		}
		if len(music) > 0 {
			// asplit vì lời đọc vừa nghe được vừa làm tín hiệu điều khiển.
			extra = append(extra, fmt.Sprintf("[%s]asplit=2[narrout][narrkey]", key))
			mbus := music[0]
			if len(music) > 1 {
				extra = append(extra, mixInto(music, "musicbus", 0))
				mbus = "musicbus"
			}
			extra = append(extra, fmt.Sprintf(
				"[%s][narrkey]sidechaincompress=threshold=%.3f:ratio=%.1f:attack=%d:release=%d[ducked]",
				mbus, duckThresh, duckRatio, duckAttack, duckRelease))
			final = append(final, "narrout", "ducked")
		} else {
			final = append(final, key)
		}
	}
	final = append(final, plain...)

	if len(final) == 1 {
		extra = append(extra, fmt.Sprintf("[%s]anull[aout]", final[0]))
	} else {
		extra = append(extra, mixInto(final, "aout", 0))
	}
	return "aout", extra
}

// subtitleFilter dựng bộ lọc ghi phụ đề lên hình.
func subtitleFilter(srt, style string) string {
	if strings.TrimSpace(style) == "" {
		style = "FontName=Be Vietnam Pro,FontSize=18,PrimaryColour=&H00FFFFFF," +
			"OutlineColour=&H80000000,BorderStyle=3,Outline=1,Shadow=0,MarginV=40"
	}
	return fmt.Sprintf("subtitles=%s:force_style='%s'", escapeFilterPath(srt), style)
}

// escapeFilterPath thoát đường dẫn cho bộ lọc của ffmpeg.
//
// Cú pháp filter dùng ':' để ngăn tham số và '\' để thoát, nên một đường dẫn
// bình thường trên máy người dùng — có dấu cách, có dấu nháy — làm vỡ cả chuỗi
// lọc và ffmpeg báo lỗi ở chỗ chẳng liên quan.
func escapeFilterPath(p string) string {
	r := strings.NewReplacer(`\`, `\\`, `:`, `\:`, `'`, `\'`, `[`, `\[`, `]`, `\]`, `,`, `\,`)
	return r.Replace(p)
}

// sourceTrackGain đọc chỉnh sửa người dùng đặt cho tiếng gốc của video.
func sourceTrackGain(d *Doc) (gain float64, muted bool) {
	for _, t := range d.Tracks {
		if t.Role == RoleSource {
			return t.Gain, t.Mute
		}
	}
	return 0, false
}

func summary(d *Doc, tracks []Track, srt string) string {
	n := 0
	for _, t := range tracks {
		n += len(t.Items)
	}
	s := fmt.Sprintf("%d lớp · %d đoạn âm thanh", len(tracks), n)
	if srt != "" {
		s += fmt.Sprintf(" · %d dòng phụ đề ghi lên hình", len(d.Subs))
	}
	return s
}
