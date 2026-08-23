package timeline

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Bộ test này chạy ffmpeg THẬT. Dựng đúng filtergraph là chuyện dễ tưởng bở:
// chuỗi nhìn hợp lý mà ffmpeg vẫn từ chối, hoặc chạy được nhưng cho ra một file
// câm. So chuỗi với chuỗi không bắt được cả hai.

func requireFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("máy không có ffmpeg")
	}
}

// gen dựng file media thử bằng chính ffmpeg.
func gen(t *testing.T, dir, name, lavfi string, extra ...string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	args := append([]string{"-v", "error", "-y", "-f", "lavfi", "-i", lavfi}, extra...)
	args = append(args, p)
	out, err := exec.Command("ffmpeg", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("dựng %s: %v — %s", name, err, out)
	}
	return p
}

func fixture(t *testing.T) (dir, base, narr, music, sfx string) {
	t.Helper()
	dir = t.TempDir()
	base = filepath.Join(dir, "base.mp4")
	cmd := exec.Command("ffmpeg", "-v", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=25:duration=20",
		"-f", "lavfi", "-i", "sine=frequency=200:duration=20",
		"-shortest", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", base)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("dựng base.mp4: %v — %s", err, out)
	}
	narr = gen(t, dir, "narr.wav", "sine=frequency=800:duration=4")
	music = gen(t, dir, "music.wav", "sine=frequency=440:duration=20")
	sfx = gen(t, dir, "sfx.wav", "sine=frequency=1200:duration=1")
	return
}

func run(t *testing.T, plan *Plan, dst string) {
	t.Helper()
	args := append(append([]string{}, plan.Args...), dst)
	out, err := exec.CommandContext(context.Background(), "ffmpeg", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("ffmpeg từ chối filtergraph: %v\n--- filter ---\n%s\n--- ffmpeg nói ---\n%s",
			err, plan.Filter, tail(string(out), 1500))
	}
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}

// probeFloat đọc một trường số từ ffprobe.
func probeFloat(t *testing.T, path string, entry string) float64 {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", entry,
		"-of", "default=nw=1:nk=1", path).Output()
	if err != nil {
		t.Fatalf("ffprobe %s: %v", entry, err)
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(strings.Split(string(out), "\n")[0]), 64)
	if err != nil {
		t.Fatalf("ffprobe %s trả %q, không đọc được số", entry, out)
	}
	return f
}

// meanVolume đo âm lượng trung bình của một KHOẢNG thời gian, dùng volumedetect.
// Đây là cách duy nhất chứng minh né giọng có thật sự xảy ra hay không.
func meanVolume(t *testing.T, path string, from, to float64) float64 {
	t.Helper()
	out, _ := exec.Command("ffmpeg", "-v", "info", "-ss", fmt.Sprintf("%.2f", from),
		"-to", fmt.Sprintf("%.2f", to), "-i", path,
		"-af", "volumedetect", "-f", "null", "-").CombinedOutput()
	for _, line := range strings.Split(string(out), "\n") {
		if i := strings.Index(line, "mean_volume:"); i >= 0 {
			f := strings.Fields(line[i+len("mean_volume:"):])
			if len(f) > 0 {
				v, err := strconv.ParseFloat(f[0], 64)
				if err == nil {
					return v
				}
			}
		}
	}
	t.Fatalf("không đọc được mean_volume từ %s [%.1f-%.1f]", path, from, to)
	return 0
}

// Trường hợp cơ bản: video nền + lời đọc đặt ở giây 5 + nhạc + tiếng động.
func TestRenderMixesEveryLayer(t *testing.T) {
	requireFFmpeg(t)
	dir, base, narr, music, sfx := fixture(t)

	d := &Doc{
		Video: base, VideoDur: 20,
		Tracks: []Track{
			{ID: "src", Role: RoleSource, Gain: -6},
			{ID: "a1", Role: RoleNarration, Items: []Item{
				{Path: narr, At: 5, In: 0, Out: 4},
			}},
			{ID: "a2", Role: RoleMusic, Gain: -12, Duck: true, Items: []Item{
				{Path: music, At: 0, In: 0, Out: 20, FadeIn: 1, FadeOut: 2},
			}},
			{ID: "a3", Role: RoleSFX, Items: []Item{
				{Path: sfx, At: 2, In: 0, Out: 1},
				{Path: sfx, At: 15, In: 0, Out: 1},
			}},
		},
	}
	d.Normalize()

	plan, err := BuildPlan(d, base, true, "")
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	dst := filepath.Join(dir, "out.mp4")
	run(t, plan, dst)

	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("không sinh ra file: %v", err)
	}
	if got := probeFloat(t, dst, "format=duration"); got < 19 || got > 21 {
		t.Errorf("thời lượng %.1fs, muốn ~20s", got)
	}
	// File câm là kiểu hỏng hay gặp nhất và im lặng nhất.
	if v := meanVolume(t, dst, 0, 20); v < -60 {
		t.Errorf("file gần như câm (mean %.1f dB) — các lớp không vào được đường ra", v)
	}
	t.Logf("mean volume toàn bài: %.1f dB", meanVolume(t, dst, 0, 20))
}

// Né giọng phải ĐO ĐƯỢC: nhạc lúc đang nói phải nhỏ hơn hẳn lúc không nói.
func TestDuckingActuallyLowersMusic(t *testing.T) {
	requireFFmpeg(t)
	dir, base, narr, music, _ := fixture(t)

	build := func(duck bool) string {
		d := &Doc{
			Video: base, VideoDur: 20,
			Tracks: []Track{
				{ID: "src", Role: RoleSource, Mute: true}, // tắt tiếng gốc cho phép đo sạch
				{ID: "a1", Role: RoleNarration, Items: []Item{{Path: narr, At: 8, In: 0, Out: 4}}},
				{ID: "a2", Role: RoleMusic, Duck: duck, Items: []Item{{Path: music, At: 0, In: 0, Out: 20}}},
			},
		}
		d.Normalize()
		plan, err := BuildPlan(d, base, true, "")
		if err != nil {
			t.Fatalf("BuildPlan(duck=%v): %v", duck, err)
		}
		dst := filepath.Join(dir, fmt.Sprintf("duck-%v.mp4", duck))
		run(t, plan, dst)
		return dst
	}

	// Đo ở khoảng KHÔNG nói (0-6s) và khoảng ĐANG nói (9-11s).
	off := build(false)
	on := build(true)

	quietOff := meanVolume(t, off, 0, 6)
	loudOff := meanVolume(t, off, 9, 11)
	quietOn := meanVolume(t, on, 0, 6)
	loudOn := meanVolume(t, on, 9, 11)

	t.Logf("KHÔNG né: yên tĩnh %.1f dB → đang nói %.1f dB (chênh %+.1f)",
		quietOff, loudOff, loudOff-quietOff)
	t.Logf("CÓ né:    yên tĩnh %.1f dB → đang nói %.1f dB (chênh %+.1f)",
		quietOn, loudOn, loudOn-quietOn)

	// Khi bật né, phần đang nói phải NHỎ HƠN so với khi không né — nhạc đã lùi.
	if loudOn >= loudOff-1.0 {
		t.Errorf("bật né mà đoạn đang nói không nhỏ đi (%.1f dB so với %.1f dB) — sidechaincompress không có tác dụng",
			loudOn, loudOff)
	}
}

// Không có phụ đề thì phải COPY stream hình, không mã hoá lại.
func TestNoSubtitlesCopiesVideoStream(t *testing.T) {
	requireFFmpeg(t)
	_, base, narr, _, _ := fixture(t)
	d := &Doc{Video: base, VideoDur: 20,
		Tracks: []Track{{ID: "a1", Role: RoleNarration, Items: []Item{{Path: narr, Out: 4}}}}}
	d.Normalize()
	plan, err := BuildPlan(d, base, true, "")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(plan.Args, " ")
	if !strings.Contains(joined, "-c:v copy") {
		t.Errorf("không copy stream hình dù không có phụ đề — mã hoá lại vô ích, chậm và mất chất lượng:\n%s", joined)
	}
}

// Có phụ đề thì ghi lên hình thật, và chữ tiếng Việt phải qua được.
func TestBurnsSubtitles(t *testing.T) {
	requireFFmpeg(t)
	dir, base, narr, _, _ := fixture(t)
	d := &Doc{Video: base, VideoDur: 20,
		Tracks: []Track{{ID: "a1", Role: RoleNarration, Items: []Item{{Path: narr, Out: 4}}}},
		Subs: []Cue{
			{Start: 1, End: 3, Text: "Chào bạn, đây là dòng phụ đề tiếng Việt"},
			{Start: 5, End: 8, Text: "Dòng thứ hai: có dấu hai chấm, dấu phẩy"},
		}}
	d.Normalize()

	srt := filepath.Join(dir, "subs.srt")
	if err := WriteSRT(d, srt); err != nil {
		t.Fatalf("WriteSRT: %v", err)
	}
	raw, _ := os.ReadFile(srt)
	if !strings.Contains(string(raw), "00:00:01,000 --> 00:00:03,000") {
		t.Errorf("mốc thời gian SRT sai:\n%s", raw)
	}

	plan, err := BuildPlan(d, base, true, srt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(plan.Args, " "), "libx264") {
		t.Error("ghi phụ đề lên hình thì phải mã hoá lại, không copy được")
	}
	run(t, plan, filepath.Join(dir, "subbed.mp4"))
}

// Video không có tiếng gốc: tham chiếu [0:a] là lỗi cả lệnh, mà thông báo của
// ffmpeg không hề gợi ý nguyên nhân.
func TestVideoWithoutAudioStream(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()
	silent := filepath.Join(dir, "silent.mp4")
	out, err := exec.Command("ffmpeg", "-v", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=size=320x240:rate=25:duration=10",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", silent).CombinedOutput()
	if err != nil {
		t.Fatalf("dựng video câm: %v — %s", err, out)
	}
	narr := gen(t, dir, "n.wav", "sine=frequency=800:duration=3")

	d := &Doc{Video: silent, VideoDur: 10,
		Tracks: []Track{{ID: "a1", Role: RoleNarration, Items: []Item{{Path: narr, At: 1, Out: 3}}}}}
	d.Normalize()

	plan, err := BuildPlan(d, silent, false, "") // srcAudio = false
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plan.Filter, "[0:a]") {
		t.Fatalf("vẫn tham chiếu [0:a] dù video không có tiếng: %s", plan.Filter)
	}
	run(t, plan, filepath.Join(dir, "out.mp4"))
}

// Đoạn đặt ở giây thứ N phải NẰM ở giây thứ N. Quên asetpts sau atrim là đoạn
// rơi vào chỗ khác hẳn mà ffmpeg không báo gì.
func TestItemLandsAtRequestedTime(t *testing.T) {
	requireFFmpeg(t)
	dir, base, _, _, sfx := fixture(t)

	d := &Doc{Video: base, VideoDur: 20,
		Tracks: []Track{
			{ID: "src", Role: RoleSource, Mute: true},
			{ID: "a1", Role: RoleSFX, Items: []Item{{Path: sfx, At: 12, In: 0, Out: 1}}},
		}}
	d.Normalize()
	plan, err := BuildPlan(d, base, true, "")
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "placed.mp4")
	run(t, plan, dst)

	before := meanVolume(t, dst, 2, 8)   // phải im
	at := meanVolume(t, dst, 12.1, 12.8) // phải có tiếng
	t.Logf("giây 2-8: %.1f dB · giây 12.1-12.8: %.1f dB", before, at)
	if at <= before+10 {
		t.Errorf("tiếng động không nằm ở giây 12 (trước %.1f dB, tại chỗ %.1f dB)", before, at)
	}
}
