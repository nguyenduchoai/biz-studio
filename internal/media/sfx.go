package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"bizstudio/internal/util"
)

// ---------- Thư viện hiệu ứng âm thanh ----------
//
// Toàn bộ tiếng động dưới đây được TỔNG HỢP tại chỗ bằng ffmpeg, không kèm file
// thu sẵn. Nhờ vậy phần mềm không phải mang theo thư viện âm thanh của bên thứ
// ba (kèm ràng buộc bản quyền), và người dùng vẫn thêm được file riêng.

// SfxPreset — một tiếng động dựng sẵn.
type SfxPreset struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Desc  string  `json:"desc"`
	Secs  float64 `json:"secs"`
	lavfi string  // nguồn + chuỗi lọc để tổng hợp
}

const sfxRate = 48000

var sfxList = []SfxPreset{
	{"whoosh", "Vút qua", "Chuyển cảnh — quét ngang nhanh", 0.55,
		"anoisesrc=r=48000:c=pink:a=0.9:d=0.55,highpass=f=300,lowpass=f=6000," +
			"afade=t=in:st=0:d=0.08:curve=exp,afade=t=out:st=0.18:d=0.37:curve=exp,volume=0.8"},
	{"pop", "Bụp", "Nhấn một từ khoá, một con số hiện ra", 0.16,
		"sine=f=520:d=0.16:r=48000,afade=t=out:st=0.01:d=0.15:curve=exp,volume=0.7"},
	{"ding", "Ting", "Báo hiệu, tick xanh, ý đúng", 1.10,
		"sine=f=1320:d=1.1:r=48000,afade=t=out:st=0.02:d=1.06:curve=exp,volume=0.55"},
	{"click", "Tách", "Bấm nút, chuyển mục nhỏ", 0.09,
		"anoisesrc=r=48000:c=white:a=0.6:d=0.09,highpass=f=1800,afade=t=out:st=0:d=0.09:curve=exp,volume=0.5"},
	{"swoosh-up", "Vút lên", "Nội dung trồi lên, chuyển sang phần mới", 0.70,
		"sine=f=180:d=0.7:r=48000,vibrato=f=3:d=0.6,afade=t=in:st=0:d=0.1," +
			"afade=t=out:st=0.4:d=0.3,highpass=f=200,volume=0.6"},
	{"impact", "Dộng", "Nhấn mạnh, chốt luận điểm", 0.80,
		"sine=f=70:d=0.8:r=48000,afade=t=out:st=0.03:d=0.75:curve=exp,volume=0.9"},
	{"riser", "Dựng cao trào", "Kéo căng trước khi tiết lộ điều bất ngờ", 1.80,
		"anoisesrc=r=48000:c=pink:a=0.7:d=1.8,highpass=f=400," +
			"afade=t=in:st=0:d=1.5:curve=qua,afade=t=out:st=1.65:d=0.15,volume=0.7"},
	{"typewriter", "Gõ chữ", "Chữ hiện từng ký tự", 0.07,
		"anoisesrc=r=48000:c=white:a=0.5:d=0.07,bandpass=f=2400:width_type=h:w=1200," +
			"afade=t=out:st=0:d=0.07:curve=exp,volume=0.45"},
	{"sparkle", "Lấp lánh", "Ngôi sao, điểm nhấn vui", 0.60,
		"sine=f=2100:d=0.6:r=48000,tremolo=f=18:d=0.8,afade=t=out:st=0.05:d=0.55:curve=exp,volume=0.4"},
	// asetrate hạ tần số lấy mẫu còn một nửa → cao độ tụt xuống VÀ thời lượng
	// dài gấp đôi, nên nguồn 1s cho ra tiếng 2s.
	{"sub-drop", "Hụt xuống", "Kết thúc, hạ nhiệt, chuyển tông trầm", 2.00,
		"sine=f=220:d=1:r=48000,asetrate=48000*0.5,aresample=48000," +
			"afade=t=out:st=0.1:d=0.9:curve=exp,volume=0.8"},
}

// SfxPresets trả danh sách tiếng động dựng sẵn.
func SfxPresets() []SfxPreset {
	out := make([]SfxPreset, len(sfxList))
	copy(out, sfxList)
	return out
}

// FindSfx tìm tiếng động theo id.
func FindSfx(id string) (SfxPreset, bool) {
	for _, s := range sfxList {
		if strings.EqualFold(s.ID, id) {
			return s, true
		}
	}
	return SfxPreset{}, false
}

// EnsureSfx bảo đảm file wav của tiếng động đã có trong sfxDir, tổng hợp nếu
// chưa. Trả đường dẫn tuyệt đối tới file.
func EnsureSfx(ctx context.Context, sfxDir, id string) (string, error) {
	p, ok := FindSfx(id)
	if !ok {
		return "", fmt.Errorf("không có tiếng động %q", id)
	}
	dst := filepath.Join(sfxDir, p.ID+".wav")
	if st, err := os.Stat(dst); err == nil && st.Size() > 0 {
		return dst, nil
	}
	if err := ensureDir(dst); err != nil {
		return "", err
	}
	raw := dst + ".raw.wav"
	defer os.Remove(raw)
	if err := run(ctx, "-y", "-hide_banner", "-f", "lavfi", "-i", p.lavfi,
		"-ac", "1", "-ar", fmt.Sprint(sfxRate), "-c:a", "pcm_s16le", raw); err != nil {
		return "", fmt.Errorf("tổng hợp tiếng động %q thất bại: %w", p.Name, err)
	}
	// Cân độ to: mỗi cách tổng hợp cho ra mức khác nhau (tiếng ồn lọc băng hẹp
	// thì rất nhỏ, sóng sin thì to). Không cân thì người dùng phải chỉnh tay
	// từng hiệu ứng. Kéo đỉnh của mọi hiệu ứng về cùng một mức.
	gain, err := peakGainDb(ctx, raw, sfxPeakDb)
	if err != nil {
		return "", err
	}
	if err := run(ctx, "-y", "-hide_banner", "-i", raw,
		"-af", fmt.Sprintf("volume=%.2fdB", gain),
		"-ac", "1", "-ar", fmt.Sprint(sfxRate), "-c:a", "pcm_s16le", dst); err != nil {
		return "", fmt.Errorf("cân độ to tiếng động %q thất bại: %w", p.Name, err)
	}
	return dst, nil
}

// sfxPeakDb — mức đỉnh chung của mọi hiệu ứng. Để dưới 0 khá nhiều vì hiệu ứng
// chồng lên lời đọc: đủ nghe mà không át tiếng người.
const sfxPeakDb = -12.0

var reMaxVolume = regexp.MustCompile(`max_volume:\s*(-?[0-9.]+) dB`)

// PeakGainDb — bản công khai của peakGainDb, cho gói khác cân độ to dùng chung.
func PeakGainDb(ctx context.Context, path string, targetDb float64) (float64, error) {
	return peakGainDb(ctx, path, targetDb)
}

// peakGainDb đo đỉnh hiện tại của file rồi trả số dB cần cộng để đạt targetDb.
func peakGainDb(ctx context.Context, path string, targetDb float64) (float64, error) {
	_, se, err := util.RunErr(ctx, "ffmpeg", "-hide_banner", "-i", path,
		"-af", "volumedetect", "-f", "null", "-")
	if err != nil {
		return 0, fmt.Errorf("đo đỉnh âm thất bại: %w", err)
	}
	m := reMaxVolume.FindStringSubmatch(se)
	if m == nil {
		return 0, fmt.Errorf("không đọc được max_volume từ ffmpeg")
	}
	peak, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, err
	}
	return targetDb - peak, nil
}

// EnsureAllSfx tổng hợp toàn bộ thư viện (gọi một lần khi mở trang SFX).
func EnsureAllSfx(ctx context.Context, sfxDir string) error {
	for _, p := range sfxList {
		if _, err := EnsureSfx(ctx, sfxDir, p.ID); err != nil {
			return err
		}
	}
	return nil
}

// SfxCue — một lần chèn tiếng động vào mốc thời gian cụ thể.
type SfxCue struct {
	Path  string  `json:"path"`  // file wav/mp3 (tuyệt đối)
	AtSec float64 `json:"atSec"` // chèn tại giây thứ mấy
	Gain  float64 `json:"gain"`  // 0 = 1.0
}

// MixSfx chèn các tiếng động vào đúng mốc thời gian của video/audio nguồn.
//
// Mỗi tiếng động được đẩy lùi bằng adelay rồi trộn chồng lên tiếng gốc. Tiếng
// gốc KHÔNG bị hạ (normalize=0) nên lời đọc giữ nguyên độ to.
func MixSfx(ctx context.Context, src, dst string, cues []SfxCue) error {
	if len(cues) == 0 {
		return copyFile(src, dst)
	}
	if !HasAudio(ctx, src) {
		return fmt.Errorf("file nguồn không có tiếng, không chèn được hiệu ứng: %s", src)
	}
	if err := ensureDir(dst); err != nil {
		return err
	}
	cs := append([]SfxCue(nil), cues...)
	sort.Slice(cs, func(i, j int) bool { return cs[i].AtSec < cs[j].AtSec })

	args := []string{"-y", "-hide_banner", "-i", src}
	var fc strings.Builder
	for _, c := range cs {
		args = append(args, "-i", c.Path)
	}
	for i, c := range cs {
		g := c.Gain
		if g <= 0 {
			g = 1
		}
		at := c.AtSec
		if at < 0 {
			at = 0
		}
		ms := int(at*1000 + 0.5)
		// adelay=ms|ms cho cả hai kênh; aresample bảo đảm cùng tần số lấy mẫu
		fmt.Fprintf(&fc, "[%d:a]aresample=%d,volume=%.3f,adelay=%d|%d[s%d];", i+1, sfxRate, g, ms, ms, i)
	}
	fc.WriteString("[0:a]")
	for i := range cs {
		fmt.Fprintf(&fc, "[s%d]", i)
	}
	fmt.Fprintf(&fc, "amix=inputs=%d:duration=first:dropout_transition=0:normalize=0[aout]", len(cs)+1)

	args = append(args, "-filter_complex", fc.String())
	if HasVideo(ctx, src) {
		args = append(args, "-map", "0:v", "-map", "[aout]", "-c:v", "copy")
	} else {
		args = append(args, "-map", "[aout]")
	}
	args = append(args, "-c:a", "aac", "-b:a", "192k", dst)
	if err := run(ctx, args...); err != nil {
		return fmt.Errorf("chèn hiệu ứng âm thanh thất bại: %w", err)
	}
	return nil
}
