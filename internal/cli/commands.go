package cli

import (
	"context"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"bizstudio/internal/broll"
	"bizstudio/internal/media"
	"bizstudio/internal/vtemplate"
	"bizstudio/internal/whisper"
)

const cmdTimeout = 6 * time.Hour

// All trả bảng lệnh.
func All() []Command {
	return []Command{
		{"probe", "Đọc thông tin một file media (không đổi gì)", runProbe},
		{"normalize", "Chuẩn hoá video cho một nền tảng (khung hình + độ to)", runNormalize},
		{"broll", "Ghép clip tư liệu khớp độ dài một file lời đọc", runBroll},
		{"autocut", "Cắt khoảng lặng, có bảo vệ theo mốc từ nếu có bản bóc băng", runAutocut},
		{"platforms", "Liệt kê preset nền tảng", runPlatforms},
		{"templates", "Liệt kê khuôn theo lĩnh vực", runTemplates},
		{"setup", "Cài/cập nhật công cụ ngoài (ffmpeg, yt-dlp, whisper…)", runSetup},
	}
}

// Dispatch chạy lệnh theo tên. Trả mã thoát.
func Dispatch(args []string) int {
	cmds := All()
	name := args[0]
	if name == "help" || name == "--help" || name == "-h" {
		Help(cmds)
		return 0
	}
	for _, c := range cmds {
		if c.Name == name {
			return Emit(c.Run(args[1:]))
		}
	}
	Help(cmds)
	return Emit(Fail(name, Usage("không có lệnh %q", name)))
}

// ---------- tiện ích dùng chung ----------

// fs dựng bộ cờ im lặng: flag tự in lỗi ra stderr theo kiểu riêng của nó, mà ta
// cần mọi lỗi đi qua đúng một đường (Result JSON) để agent đọc được.
func fs(name string) *flag.FlagSet {
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	f.SetOutput(new(strings.Builder))
	return f
}

// parse phân tích cờ KHÔNG phụ thuộc thứ tự.
//
// Gói flag của Go dừng ngay ở đối số đầu tiên không phải cờ, nên
// `normalize video.mp4 --platform tiktok` sẽ bỏ qua hẳn --platform và báo một
// lỗi chẳng liên quan. Mà đó lại là cách viết tự nhiên nhất, cả người lẫn agent
// đều gõ thế. Ở đây dồn cờ lên trước rồi mới phân tích.
func parse(f *flag.FlagSet, args []string) error {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" { // mọi thứ sau "--" là đối số thường
			pos = append(pos, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") {
			pos = append(pos, a)
			continue
		}
		flags = append(flags, a)
		// Cờ dạng "--ten gia-tri" cần nuốt thêm một đối số; cờ luận lý và
		// dạng "--ten=gia-tri" thì không. Hỏi chính FlagSet xem cờ này là gì
		// thay vì đoán — đoán sai là nuốt mất một đối số thường.
		if strings.Contains(a, "=") {
			continue
		}
		name := strings.TrimLeft(a, "-")
		if fl := f.Lookup(name); fl != nil && !isBoolFlag(fl) && i+1 < len(args) {
			flags = append(flags, args[i+1])
			i++
		}
	}
	// Chèn lại "--" trước nhóm đối số thường: mất nó thì tên file bắt đầu bằng
	// dấu gạch ("-ten-la.mp4") lại bị hiểu là cờ.
	if len(pos) > 0 {
		flags = append(flags, "--")
	}
	return f.Parse(append(flags, pos...))
}

func isBoolFlag(fl *flag.Flag) bool {
	b, ok := fl.Value.(interface{ IsBoolFlag() bool })
	return ok && b.IsBoolFlag()
}

// needFile kiểm file có thật và không phải thư mục.
func needFile(label, p string) error {
	if strings.TrimSpace(p) == "" {
		return Usage("thiếu %s", label)
	}
	fi, err := os.Stat(p)
	if err != nil {
		return Usage("không tìm thấy %s: %s", label, p)
	}
	if fi.IsDir() {
		return Usage("%s là thư mục, cần một file: %s", label, p)
	}
	return nil
}

// needTools kiểm các công cụ ngoài có trong PATH chưa. Đây là loại lỗi
// "dependency": agent biết phải đi cài chứ không phải thử lại.
func needTools(names ...string) error {
	var thieu []string
	for _, n := range names {
		if _, err := exec.LookPath(n); err != nil {
			thieu = append(thieu, n)
		}
	}
	if len(thieu) > 0 {
		return Dependency("thiếu công cụ trong PATH: %s — cài rồi chạy lại", strings.Join(thieu, ", "))
	}
	return nil
}

// outPath chọn đường dẫn ra: --out nếu có, không thì đặt cạnh file nguồn.
func outPath(out, src, suffix string) string {
	if strings.TrimSpace(out) != "" {
		return out
	}
	return strings.TrimSuffix(src, filepath.Ext(src)) + suffix
}

// finish ghi manifest rồi dựng Result thành công.
func finish(command, workdir string, outs map[string]string, stats map[string]any, dry bool) Result {
	res := Result{
		OK: true, Command: command, Workdir: workdir,
		Stats: stats, DryRun: dry,
		Outputs: map[string]any{},
	}
	for k, v := range outs {
		res.Outputs[k] = v
	}
	if workdir != "" {
		m := LoadManifest(workdir)
		if err := m.Save(workdir, command, ManifestStage{
			Command: command, At: Now(), Outputs: outs, Stats: stats, DryRun: dry,
		}); err != nil {
			Logf("cảnh báo: không ghi được manifest: %v", err)
		}
	}
	return res
}

// ---------- probe ----------

func runProbe(args []string) Result {
	f := fs("probe")
	if err := parse(f, args); err != nil {
		return Fail("probe", Usage("%s", err))
	}
	if f.NArg() != 1 {
		return Fail("probe", Usage("dùng: bizstudio probe <file>"))
	}
	src := f.Arg(0)
	if err := needFile("file", src); err != nil {
		return Fail("probe", err)
	}
	if err := needTools("ffprobe"); err != nil {
		return Fail("probe", err)
	}
	info, err := media.Probe(src)
	if err != nil {
		return Fail("probe", Failed("đọc file thất bại: %v", err))
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	return Result{
		OK: true, Command: "probe",
		Outputs: map[string]any{"path": src},
		Stats: map[string]any{
			"durationSec": info.Duration, "width": info.Width, "height": info.Height,
			"fps": info.FPS, "sizeBytes": info.Size,
			"hasVideo": media.HasVideo(ctx, src), "hasAudio": media.HasAudio(ctx, src),
		},
	}
}

// ---------- normalize ----------

func runNormalize(args []string) Result {
	f := fs("normalize")
	platform := f.String("platform", "", "id nền tảng (xem: bizstudio platforms)")
	out := f.String("out", "", "file ra (mặc định đặt cạnh file nguồn)")
	workdir := f.String("workdir", "", "thư mục làm việc để ghi manifest")
	dry := f.Bool("dry-run", false, "chỉ kiểm tham số, không chạy")
	if err := parse(f, args); err != nil {
		return Fail("normalize", Usage("%s", err))
	}
	if f.NArg() != 1 {
		return Fail("normalize", Usage("dùng: bizstudio normalize <video> --platform tiktok"))
	}
	src := f.Arg(0)
	if err := needFile("video", src); err != nil {
		return Fail("normalize", err)
	}
	p, ok := vtemplate.FindPlatform(*platform)
	if !ok {
		return Fail("normalize", Usage("nền tảng %q không có — xem danh sách: bizstudio platforms", *platform))
	}
	dst := outPath(*out, src, "."+p.ID+".mp4")

	if *dry {
		return finish("normalize", *workdir,
			map[string]string{"video": dst},
			map[string]any{"platform": p.Name, "toW": p.Width, "toH": p.Height, "lufs": p.LUFS}, true)
	}
	if err := needTools("ffmpeg", "ffprobe"); err != nil {
		return Fail("normalize", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	Logf("Chuẩn hoá cho %s (%dx%d, %.0f LUFS)…", p.Name, p.Width, p.Height, p.LUFS)
	rep, err := vtemplate.NormalizeForPlatform(ctx, src, dst, p.ID)
	if err != nil {
		return Fail("normalize", Failed("%v", err))
	}
	stats := map[string]any{
		"platform": p.Name, "fromW": rep.FromW, "fromH": rep.FromH,
		"toW": rep.ToW, "toH": rep.ToH, "padded": rep.Padded,
		"durationSec": rep.Duration, "overLimit": rep.OverLimit,
		"burnedTextScale": rep.TextScale,
	}
	if rep.TextWarn != "" {
		stats["textWarn"] = rep.TextWarn
		Logf("⚠ %s", rep.TextWarn)
	}
	if rep.OverLimit {
		stats["note"] = rep.Note
		Logf("⚠ %s", rep.Note)
	}
	return finish("normalize", *workdir, map[string]string{"video": dst}, stats, false)
}

// ---------- broll ----------

func runBroll(args []string) Result {
	f := fs("broll")
	clipsDir := f.String("clips", "", "thư mục chứa clip tư liệu")
	audio := f.String("audio", "", "file lời đọc")
	aspect := f.String("aspect", "", "9:16 | 3:4 | 16:9 | 1:1 (mặc định theo clip đầu)")
	maxClip := f.Float64("max-clip", 0, "mỗi mẩu tối đa bao nhiêu giây (mặc định 5)")
	fps := f.Int("fps", 0, "khung hình mỗi giây (mặc định 30)")
	shuffle := f.Bool("shuffle", false, "xáo thứ tự mẩu (vẫn tất định)")
	out := f.String("out", "", "file ra")
	workdir := f.String("workdir", "", "thư mục làm việc để ghi manifest")
	dry := f.Bool("dry-run", false, "chỉ kiểm tham số, không chạy")
	if err := parse(f, args); err != nil {
		return Fail("broll", Usage("%s", err))
	}

	// Thiếu tham số thì lấy từ manifest của lệnh trước trong cùng thư mục —
	// đây là điểm chính của manifest: nối lệnh mà không phải chép đường dẫn.
	if *audio == "" && *workdir != "" {
		*audio = LoadManifest(*workdir).Get("voice")
	}
	if strings.TrimSpace(*clipsDir) == "" {
		return Fail("broll", Usage("thiếu --clips (thư mục clip tư liệu)"))
	}
	fi, err := os.Stat(*clipsDir)
	if err != nil || !fi.IsDir() {
		return Fail("broll", Usage("--clips phải là thư mục có thật: %s", *clipsDir))
	}
	if err := needFile("--audio", *audio); err != nil {
		return Fail("broll", err)
	}
	clips, err := broll.ListClips(*clipsDir)
	if err != nil {
		return Fail("broll", Failed("đọc thư mục clip: %v", err))
	}
	if len(clips) == 0 {
		return Fail("broll", Usage("thư mục %s không có file video nào (.mp4 .mov .mkv .webm .m4v .avi)", *clipsDir))
	}
	dst := outPath(*out, *audio, ".broll.mp4")

	opt := broll.Opt{MaxClipSec: *maxClip, FPS: *fps, Shuffle: *shuffle}
	if a := strings.TrimSpace(*aspect); a != "" {
		opt.Width, opt.Height = vtemplate.AspectSize(a)
	}
	if *dry {
		return finish("broll", *workdir, map[string]string{"video": dst},
			map[string]any{"clipsFound": len(clips), "clipsDir": *clipsDir, "audio": *audio}, true)
	}
	if err := needTools("ffmpeg", "ffprobe"); err != nil {
		return Fail("broll", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	Logf("Ghép %d clip tư liệu khớp lời đọc…", len(clips))
	rep, err := broll.Assemble(ctx, clips, *audio, dst, opt)
	if err != nil {
		return Fail("broll", Failed("%v", err))
	}
	stats := map[string]any{
		"audioSec": rep.AudioSec, "videoSec": rep.VideoSec,
		"pieces": rep.Pieces, "clipsUsed": rep.Clips, "clipsFound": len(clips),
		"width": rep.Width, "height": rep.Height,
	}
	if rep.ShortFall {
		stats["reusedRounds"] = rep.Reused
		Logf("⚠ tư liệu không đủ dài, đã dùng lại %d vòng — thêm clip để hình đỡ lặp", rep.Reused)
	}
	return finish("broll", *workdir, map[string]string{"video": dst}, stats, false)
}

// ---------- autocut ----------

func runAutocut(args []string) Result {
	f := fs("autocut")
	out := f.String("out", "", "file ra")
	srt := f.String("transcript", "", "file transcript.json để bảo vệ theo mốc từ (tuỳ chọn)")
	minSilence := f.Float64("min-silence", 0, "khoảng lặng ngắn hơn mức này thì không cắt (mặc định 0.6s)")
	workdir := f.String("workdir", "", "thư mục làm việc để ghi manifest")
	dry := f.Bool("dry-run", false, "chỉ kiểm tham số, không chạy")
	if err := parse(f, args); err != nil {
		return Fail("autocut", Usage("%s", err))
	}
	if f.NArg() != 1 {
		return Fail("autocut", Usage("dùng: bizstudio autocut <video|audio>"))
	}
	src := f.Arg(0)
	if err := needFile("file nguồn", src); err != nil {
		return Fail("autocut", err)
	}
	dst := outPath(*out, src, ".autocut.mp4")

	var tr *whisper.Transcript
	if p := strings.TrimSpace(*srt); p != "" {
		if err := needFile("--transcript", p); err != nil {
			return Fail("autocut", err)
		}
		got, err := whisper.LoadJSON(p)
		if err != nil {
			return Fail("autocut", Usage("đọc transcript thất bại: %v", err))
		}
		tr = got
	}
	if *dry {
		return finish("autocut", *workdir, map[string]string{"video": dst},
			map[string]any{"guarded": tr != nil}, true)
	}
	if err := needTools("ffmpeg", "ffprobe"); err != nil {
		return Fail("autocut", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	Logf("Cắt khoảng lặng…")
	rep, err := media.AutoCutGuarded(ctx, src, dst, tr,
		media.AutoCutOpt{MinSilence: *minSilence},
		func(p float64, d string) { Logf("  %.0f%% %s", p, d) })
	if err != nil {
		return Fail("autocut", Failed("%v", err))
	}
	return finish("autocut", *workdir, map[string]string{"video": dst},
		map[string]any{
			"beforeSec": rep.BeforeS, "afterSec": rep.AfterS,
			"removedSec": rep.BeforeS - rep.AfterS, "guarded": rep.Guarded,
		}, false)
}

// ---------- tra cứu ----------

func runPlatforms(args []string) Result {
	f := fs("platforms")
	if err := parse(f, args); err != nil {
		return Fail("platforms", Usage("%s", err))
	}
	list := []any{}
	for _, p := range vtemplate.Platforms() {
		list = append(list, map[string]any{
			"id": p.ID, "name": p.Name, "width": p.Width, "height": p.Height,
			"maxSec": p.MaxSec, "lufs": p.LUFS, "note": p.Note,
		})
	}
	return Result{OK: true, Command: "platforms", Outputs: map[string]any{"platforms": list}}
}

func runTemplates(args []string) Result {
	f := fs("templates")
	cat := f.String("category", "", "lọc theo danh mục")
	if err := parse(f, args); err != nil {
		return Fail("templates", Usage("%s", err))
	}
	list := []any{}
	for _, t := range vtemplate.All() {
		if c := strings.TrimSpace(*cat); c != "" && !strings.EqualFold(t.Category, c) {
			continue
		}
		list = append(list, map[string]any{
			"id": t.ID, "name": t.Name, "category": t.Category, "desc": t.Desc,
			"aspect": t.Aspect, "seconds": t.Seconds, "platform": t.Platform,
			"style": t.Style, "musicMood": t.MusicMood,
		})
	}
	return Result{OK: true, Command: "templates",
		Outputs: map[string]any{"templates": list, "categories": vtemplate.Categories()},
		Stats:   map[string]any{"count": len(list)}}
}
