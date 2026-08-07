package recap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bizstudio/internal/media"
	"bizstudio/internal/store"
	"bizstudio/internal/tts"
	"bizstudio/internal/util"
)

// ---------- Dựng video kể chuyện ----------
//
// Mỗi cảnh render thành một clip riêng rồi ghép — vì mỗi cảnh có một bài toán
// đồng bộ riêng: lời dài hơn cảnh thì (1) tăng tốc giọng tới trần 1.6x, (2) vẫn
// chưa đủ thì ĐÓNG BĂNG khung hình cuối kéo dài cảnh cho tròn lời. Tuyệt đối
// không để lời bị cắt giữa chừng.

const (
	// maxVoiceSpeed — trần tăng tốc giọng, cùng mức với module lồng tiếng:
	// quá 1.6x người nghe bắt đầu nhận ra giọng bị ép.
	maxVoiceSpeed = 1.6
	// origDuckVol — âm lượng tiếng gốc SAU khi đã né lời (0..1).
	origDuckVol = 0.65
)

// RenderOpts — tham số dựng.
type RenderOpts struct {
	Voice        string  // giọng đọc; rỗng = giọng mặc định của engine
	Engine       string  // vieneu | say | gemini | clone:<id>; rỗng = tự chọn
	KeepOriginal bool    // giữ tiếng gốc của phim (tự né lời)
	OrigVolume   float64 // 0 = origDuckVol
	BurnSub      bool    // gắn cứng phụ đề lời dẫn
}

// RenderResult — sản phẩm của một lần dựng.
type RenderResult struct {
	VideoPath string  `json:"videoPath"` // tuyệt đối
	SRTPath   string  `json:"srtPath"`
	AudioDir  string  `json:"audioDir"` // các file giọng từng cảnh — nguồn cho bước xuất trình dựng ngoài
	Duration  float64 `json:"duration"`
	Voiced    int     `json:"voiced"` // số cảnh có lời
}

// Render dựng video kể chuyện từ manifest. srcAbs = video nguồn tuyệt đối.
func Render(ctx context.Context, st *store.Store, m *Manifest, srcAbs string,
	opt RenderOpts, upd func(float64, string)) (*RenderResult, error) {

	if upd == nil {
		upd = func(float64, string) {}
	}
	if opt.OrigVolume <= 0 {
		opt.OrigVolume = origDuckVol
	}
	base := Dir(st.DataDir, m.ID)
	audioDir := filepath.Join(base, "audio")
	clipDir := filepath.Join(base, "clips")
	for _, d := range []string{audioDir, clipDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("tạo thư mục làm việc: %w", err)
		}
	}

	// Phim không có track tiếng (video quay màn hình, timelapse...) vẫn phải
	// dựng được: mọi clip đều cần track audio để concat, nên thiếu thì chèn im
	// lặng, và bước "tiếng gốc né lời" tự tắt.
	hasAudio := media.HasAudio(ctx, srcAbs)

	res := &RenderResult{AudioDir: audioDir}
	var clips []string
	var srt strings.Builder
	cursor := 0.0 // mốc bắt đầu của cảnh hiện tại trên timeline THÀNH PHẨM
	nSub := 0

	for i, sc := range m.Scenes {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("dựng bị hủy: %w", err)
		}
		upd(5+float64(i)/float64(len(m.Scenes))*85,
			fmt.Sprintf("Cảnh %d/%d…", i+1, len(m.Scenes)))

		clip := filepath.Join(clipDir, fmt.Sprintf("clip-%03d.mp4", sc.Index))
		text := strings.TrimSpace(sc.Text)

		if text == "" {
			// Cảnh không lời: giữ nguyên đoạn phim với tiếng gốc.
			if err := cutPlain(ctx, srcAbs, sc, hasAudio, clip); err != nil {
				return nil, fmt.Errorf("cảnh %d: %w", sc.Index, err)
			}
			clips = append(clips, clip)
			cursor += sc.End - sc.Start
			continue
		}

		// 1) Đọc lời và ĐO thời lượng thật. Lời không đổi thì dùng lại file
		// giọng cũ — người dùng sửa một cảnh rồi dựng lại không phải chờ đọc
		// lại cả phim (VieNeu trên máy đọc đoạn dài mất hàng phút).
		wav := filepath.Join(audioDir, fmt.Sprintf("seg-%03d.wav", sc.Index))
		sidecar := wav + ".txt"
		cached := false
		if old, err := os.ReadFile(sidecar); err == nil && string(old) == voiceCacheKey(text, opt) {
			if st2, err := os.Stat(wav); err == nil && st2.Size() > 0 {
				cached = true
			}
		}
		if !cached {
			if err := tts.Speak(ctx, st, text, opt.Voice, 0, opt.Engine, wav); err != nil {
				return nil, fmt.Errorf("cảnh %d: đọc lời thất bại: %w", sc.Index, err)
			}
			_ = os.WriteFile(sidecar, []byte(voiceCacheKey(text, opt)), 0o644)
		}
		vInfo, err := media.Probe(wav)
		if err != nil || vInfo.Duration <= 0 {
			return nil, fmt.Errorf("cảnh %d: không đo được thời lượng giọng", sc.Index)
		}
		voiceDur := vInfo.Duration
		sceneDur := sc.End - sc.Start

		// 2) Khớp lời vào cảnh: tăng tốc có trần, quá trần thì kéo dài cảnh.
		speed := 1.0
		if voiceDur > sceneDur {
			speed = voiceDur / sceneDur
			if speed > maxVoiceSpeed {
				speed = maxVoiceSpeed
			}
		}
		fitted := voiceDur / speed // thời lượng lời sau tăng tốc
		finalDur := sceneDur       // thời lượng cảnh trên thành phẩm
		if fitted > sceneDur {
			finalDur = fitted // đóng băng khung cuối phần dôi — không cắt lời
		}

		if err := buildVoicedClip(ctx, srcAbs, wav, sc, speed, finalDur, opt, hasAudio, clip); err != nil {
			return nil, fmt.Errorf("cảnh %d: %w", sc.Index, err)
		}
		clips = append(clips, clip)

		nSub++
		subEnd := cursor + fitted
		fmt.Fprintf(&srt, "%d\n%s --> %s\n%s\n\n", nSub, srtTime(cursor), srtTime(subEnd), text)
		cursor += finalDur
		res.Voiced++
	}

	upd(92, "Ghép các cảnh…")
	merged := filepath.Join(base, "recap.mp4")
	if err := media.Concat(ctx, clips, merged); err != nil {
		return nil, fmt.Errorf("ghép cảnh thất bại: %w", err)
	}

	srtPath := filepath.Join(base, "recap.srt")
	if err := os.WriteFile(srtPath, []byte(srt.String()), 0o644); err != nil {
		return nil, fmt.Errorf("ghi phụ đề: %w", err)
	}
	res.SRTPath = srtPath

	final := merged
	if opt.BurnSub && res.Voiced > 0 {
		upd(96, "Gắn phụ đề vào video…")
		burned := filepath.Join(base, "recap-sub.mp4")
		if err := media.BurnSubs(ctx, merged, srtPath, burned); err != nil {
			return nil, fmt.Errorf("gắn phụ đề thất bại: %w", err)
		}
		final = burned
	}
	res.VideoPath = final
	if info, err := media.Probe(final); err == nil {
		res.Duration = info.Duration
	}
	upd(99, fmt.Sprintf("Xong — %d cảnh, %d cảnh có lời, dài %.1fs", len(m.Scenes), res.Voiced, res.Duration))
	return res, nil
}

// cutPlain cắt nguyên đoạn phim, tiếng gốc giữ 100%. Re-encode về cùng một bộ
// codec/thông số với các clip khác để concat không vấp.
func cutPlain(ctx context.Context, src string, sc Scene, hasAudio bool, dst string) error {
	args := []string{"-y", "-hide_banner",
		"-ss", f3(sc.Start), "-to", f3(sc.End), "-i", src,
	}
	if hasAudio {
		args = append(args,
			"-vf", "fps=30,setpts=PTS-STARTPTS",
			"-af", "aresample=48000,asetpts=PTS-STARTPTS")
	} else {
		// nguồn câm: chèn track im lặng để mọi clip cùng cấu trúc stream
		args = append(args,
			"-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo",
			"-vf", "fps=30,setpts=PTS-STARTPTS",
			"-map", "0:v", "-map", "1:a", "-shortest")
	}
	args = append(args, encodeArgs(dst)...)
	if _, err := util.Run(ctx, "ffmpeg", args...); err != nil {
		return fmt.Errorf("cắt cảnh thất bại: %w", err)
	}
	return nil
}

// buildVoicedClip dựng một cảnh có lời:
//   - video: đoạn phim gốc; nếu finalDur > thời lượng cảnh thì tpad ĐÓNG BĂNG
//     khung hình cuối cho đủ.
//   - audio: lời (đã tăng tốc nếu cần) + tiếng gốc tự NÉ lời qua nén sidechain
//     (đang nói thì tiếng gốc lùi xuống, hết câu nâng lại) — hoặc bỏ tiếng gốc.
func buildVoicedClip(ctx context.Context, src, wav string, sc Scene,
	speed, finalDur float64, opt RenderOpts, hasAudio bool, dst string) error {

	sceneDur := sc.End - sc.Start
	pad := finalDur - sceneDur
	vchain := "fps=30,setpts=PTS-STARTPTS"
	if pad > 0.001 {
		vchain += fmt.Sprintf(",tpad=stop_mode=clone:stop_duration=%s", f3(pad))
	}

	// Hai điều bắt buộc, cả hai đều đo ra bằng treo/lệch thật:
	//
	//  1. apad đặt TRƯỚC asplit và đệm tới ĐÚNG finalDur: sidechaincompress
	//     dừng khi tín hiệu điều khiển hết — lời 2 giây trong cảnh 7 giây mà
	//     không đệm thì track tiếng cụt ở giây 2 (đo: hình 7.00s, tiếng 1.54s),
	//     ghép nhiều cảnh là tiếng/hình lệch dồn dần.
	//  2. Nhánh HÌNH phải nằm CHUNG filter_complex với nhánh tiếng: để hình ở
	//     -vf riêng bên ngoài thì ffmpeg kẹt cứng giữa hai đồ thị lọc khi đồ
	//     thị tiếng có apad (đo: treo vô hạn, wait4 18 phút, 0% CPU).
	apad := fmt.Sprintf("apad=whole_dur=%s", f3(finalDur+0.1))
	var fc string
	if opt.KeepOriginal && hasAudio {
		fc = fmt.Sprintf(
			"[0:v]%s[v];"+
				"[1:a]%s,aresample=48000,%s[vp];[vp]asplit=2[voice][key];"+
				"[0:a]aresample=48000[bgsrc];"+
				"[bgsrc][key]sidechaincompress=threshold=0.03:ratio=4:attack=20:release=400[bgduck];"+
				"[bgduck]volume=%.3f[bg];"+
				"[voice][bg]amix=inputs=2:duration=longest:dropout_transition=2:normalize=0[aout]",
			vchain, media.AtempoChain(speed), apad, opt.OrigVolume)
	} else {
		fc = fmt.Sprintf("[0:v]%s[v];[1:a]%s,aresample=48000,%s[aout]",
			vchain, media.AtempoChain(speed), apad)
	}

	args := []string{"-y", "-hide_banner",
		"-ss", f3(sc.Start), "-to", f3(sc.End), "-i", src,
		"-i", wav,
		"-filter_complex", fc,
		"-map", "[v]", "-map", "[aout]",
		"-t", f3(finalDur),
	}
	args = append(args, encodeArgs(dst)...)
	if _, err := util.Run(ctx, "ffmpeg", args...); err != nil {
		return fmt.Errorf("dựng cảnh có lời thất bại: %w", err)
	}
	return nil
}

// encodeArgs — bộ thông số chung để mọi clip concat được với nhau.
func encodeArgs(dst string) []string {
	return []string{
		"-c:v", "libx264", "-crf", "20", "-preset", "veryfast", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "192k", "-ar", "48000", "-ac", "2",
		dst,
	}
}

func f3(v float64) string { return fmt.Sprintf("%.3f", v) }

// voiceCacheKey — file giọng chỉ được dùng lại khi lời VÀ giọng/engine y hệt.
func voiceCacheKey(text string, opt RenderOpts) string {
	return opt.Engine + "|" + opt.Voice + "|" + text
}

// srtTime — 00:00:00,000.
func srtTime(sec float64) string {
	ms := int(sec*1000 + 0.5)
	h := ms / 3600000
	ms %= 3600000
	mi := ms / 60000
	ms %= 60000
	s := ms / 1000
	ms %= 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, mi, s, ms)
}
