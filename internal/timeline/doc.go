// Package timeline mô tả cách xếp nhiều lớp âm thanh và phụ đề lên trên MỘT
// lớp video nền, và dựng lệnh ffmpeg từ mô tả đó.
//
// Vì sao chỉ một lớp video: xem trước phải đúng thì người ta mới tin mà dựng.
// Nhiều lớp âm thanh và phụ đề thì trình duyệt tự trộn và tự hiện được — chính
// xác 100%, không độ trễ. Lồng video trên video thì bắt buộc phải ghép hình,
// hoặc trong trình duyệt (đổi cả kiến trúc giao diện) hoặc render nháp ở máy
// chủ (kéo một cái chờ một hai giây). Cả hai đều là chuyện khác, để sau.
//
// Thời gian tính bằng GIÂY chứ không theo khung hình: video nguồn có fps bất
// kỳ, mà ở đây không cắt hình nên không cần chính xác tới từng khung.
package timeline

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Vai trò của một lớp âm thanh. Vai trò quyết định cách trộn, không chỉ là nhãn.
const (
	RoleSource    = "source"    // tiếng gốc của video nền
	RoleNarration = "narration" // lời đọc — cũng là tín hiệu để nhạc né
	RoleMusic     = "music"     // nhạc nền — né được
	RoleSFX       = "sfx"       // tiếng động
)

// maxItems — trần số đoạn âm thanh trong một timeline.
//
// Mỗi đoạn thành một input của ffmpeg. Vài chục thì không sao, vài nghìn thì
// dòng lệnh vượt giới hạn của hệ điều hành và lỗi báo ra chẳng nói gì về
// nguyên nhân.
const maxItems = 200

// Doc — timeline của một dự án.
type Doc struct {
	ProjectID string    `json:"projectId"`
	Video     string    `json:"video"` // đường dẫn tương đối data dir; rỗng = chưa chọn
	VideoDur  float64   `json:"videoDur"`
	Tracks    []Track   `json:"tracks"`
	Subs      []Cue     `json:"subs"`
	SubStyle  string    `json:"subStyle"` // force_style của ffmpeg; rỗng = mặc định
	UpdatedAt time.Time `json:"updatedAt"`
}

// Track — một lớp âm thanh.
type Track struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Role  string  `json:"role"`
	Mute  bool    `json:"mute"`
	Gain  float64 `json:"gain"` // dB
	Duck  bool    `json:"duck"` // chỉ có nghĩa với role=music
	Items []Item  `json:"items"`
}

// Item — một đoạn âm thanh đặt trên lớp.
type Item struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Path    string  `json:"path"`
	At      float64 `json:"at"`  // đặt ở giây nào trên timeline
	In      float64 `json:"in"`  // cắt từ giây nào của file nguồn
	Out     float64 `json:"out"` // tới giây nào; <=0 nghĩa là hết file
	Gain    float64 `json:"gain"`
	FadeIn  float64 `json:"fadeIn"`
	FadeOut float64 `json:"fadeOut"`
}

// Dur — độ dài đoạn sau khi cắt.
func (it Item) Dur() float64 {
	if it.Out > it.In {
		return it.Out - it.In
	}
	return 0
}

// End — thời điểm kết thúc trên timeline.
func (it Item) End() float64 { return it.At + it.Dur() }

// Cue — một dòng phụ đề.
type Cue struct {
	ID    string  `json:"id"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// Normalize dọn tài liệu về trạng thái hợp lệ và ổn định.
//
// Gọi trước mỗi lần lưu VÀ mỗi lần dựng. Giao diện gửi lên đủ kiểu số lệch —
// kéo quá mép trái thành số âm, kéo mép phải quá độ dài file, fade dài hơn cả
// đoạn. Sửa ở đúng một chỗ thay vì rải phép kiểm khắp nơi.
func (d *Doc) Normalize() {
	for ti := range d.Tracks {
		t := &d.Tracks[ti]
		if t.Role == "" {
			t.Role = RoleSFX
		}
		t.Gain = clampDB(t.Gain)

		kept := t.Items[:0]
		for _, it := range t.Items {
			if strings.TrimSpace(it.Path) == "" {
				continue
			}
			if it.At < 0 {
				it.At = 0
			}
			if it.In < 0 {
				it.In = 0
			}
			if it.Out > 0 && it.Out <= it.In {
				continue // kéo mép qua nhau — đoạn rỗng, bỏ
			}
			it.Gain = clampDB(it.Gain)
			// Fade không được dài hơn chính đoạn, nếu không ffmpeg cho ra một
			// đoạn câm mà không báo lỗi gì.
			if dur := it.Dur(); dur > 0 {
				it.FadeIn = clampFade(it.FadeIn, dur)
				it.FadeOut = clampFade(it.FadeOut, dur)
			} else {
				it.FadeIn, it.FadeOut = 0, 0
			}
			kept = append(kept, it)
		}
		t.Items = kept
		sort.SliceStable(t.Items, func(i, j int) bool { return t.Items[i].At < t.Items[j].At })
	}

	subs := d.Subs[:0]
	for _, c := range d.Subs {
		if strings.TrimSpace(c.Text) == "" || c.End <= c.Start {
			continue
		}
		if c.Start < 0 {
			c.Start = 0
		}
		subs = append(subs, c)
	}
	d.Subs = subs
	sort.SliceStable(d.Subs, func(i, j int) bool { return d.Subs[i].Start < d.Subs[j].Start })
}

// Validate báo lỗi những thứ Normalize không tự sửa được.
func (d *Doc) Validate() error {
	if strings.TrimSpace(d.Video) == "" {
		return fmt.Errorf("chưa chọn video nền cho timeline")
	}
	n := 0
	for _, t := range d.Tracks {
		n += len(t.Items)
	}
	if n > maxItems {
		return fmt.Errorf("timeline có %d đoạn âm thanh, quá trần %d — gộp bớt lại", n, maxItems)
	}
	return nil
}

// Dur — độ dài timeline: video nền, hoặc đoạn âm thanh cuối nếu nó dài hơn.
func (d *Doc) Dur() float64 {
	end := d.VideoDur
	for _, t := range d.Tracks {
		for _, it := range t.Items {
			if e := it.End(); e > end {
				end = e
			}
		}
	}
	for _, c := range d.Subs {
		if c.End > end {
			end = c.End
		}
	}
	return end
}

// audible trả các lớp có tiếng: bỏ lớp tắt và lớp rỗng.
func (d *Doc) audible() []Track {
	var out []Track
	for _, t := range d.Tracks {
		if t.Mute || len(t.Items) == 0 {
			continue
		}
		out = append(out, t)
	}
	return out
}

// clampDB giữ âm lượng trong khoảng dùng được. Dưới -60 dB coi như câm; trên
// +12 dB là vỡ tiếng chứ không to thêm.
func clampDB(v float64) float64 {
	if v < -60 {
		return -60
	}
	if v > 12 {
		return 12
	}
	return v
}

func clampFade(v, dur float64) float64 {
	if v < 0 {
		return 0
	}
	if half := dur / 2; v > half {
		return half
	}
	return v
}

// GuessRole đoán vai trò của một file âm thanh từ TÊN của nó.
//
// Mặc định mọi lớp thành "lời đọc" thì nhạc nền cũng bị chấm như lời đọc: không
// có ô "né lời đọc", và nếu người dùng không để ý thì nhạc đè lên tiếng nói suốt
// video. Đoán sai vẫn sửa được bằng một cú bấm, còn đoán đúng thì đỡ hẳn một
// bước cho trường hợp phổ biến nhất.
func GuessRole(name string) string {
	n := strings.ToLower(name)
	for _, k := range []string{"music", "nhac", "nhạc", "bgm", "beat", "track", "song", "background"} {
		if strings.Contains(n, k) {
			return RoleMusic
		}
	}
	for _, k := range []string{"sfx", "sound", "effect", "tieng-dong", "tiếng động", "whoosh", "swoosh", "pop", "click"} {
		if strings.Contains(n, k) {
			return RoleSFX
		}
	}
	return RoleNarration
}
