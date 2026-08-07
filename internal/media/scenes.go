package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"bizstudio/internal/util"
)

// ---------- Phát hiện chuyển cảnh ----------
//
// Khác với cắt khoảng lặng (nghe ÂM THANH tìm chỗ ngừng nói), phát hiện cảnh
// nhìn HÌNH ẢNH tìm chỗ khung hình đổi đột ngột — đây là bước đầu của mọi
// pipeline "phim dài → video kể chuyện": chia phim thành các phân đoạn có
// nghĩa rồi mới viết lời cho từng phân đoạn.
//
// Hai nguyên tắc rút từ việc mổ các công cụ cùng loại:
//  1. KHÔNG cắt bớt stderr của ffmpeg — mốc cảnh nằm rải suốt stream, cắt đuôi
//     là mất các điểm cắt đầu phim.
//  2. KHÔNG đặt trần số cảnh kiểu "lấy 12 điểm đầu" — phim dài sẽ dồn hết vào
//     mấy phút đầu còn phần còn lại thành một cảnh khổng lồ. Cảnh ngắn quá thì
//     GỘP vào cảnh trước, không vứt.

const (
	// defaultSceneThreshold — mức đổi khung hình bị coi là chuyển cảnh (0..1).
	// 0.30–0.40 là dải thường dùng; thấp hơn nhạy hơn (nhiều cảnh hơn).
	defaultSceneThreshold = 0.35
	// defaultMinSceneSec — cảnh ngắn hơn mức này bị gộp vào cảnh trước:
	// một cú chớp đèn hay pan nhanh không phải là "cảnh".
	defaultMinSceneSec = 2.0
)

// Scene — một phân đoạn của video nguồn.
type Scene struct {
	Index int     `json:"index"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	// Frame — ảnh đại diện (giữa cảnh), đường dẫn tuyệt đối; rỗng nếu chưa trích.
	Frame string `json:"frame,omitempty"`
}

func (s Scene) Duration() float64 { return s.End - s.Start }

// showinfo in pts_time của những khung hình lọt qua select — chính là mốc chuyển cảnh.
var reScenePts = regexp.MustCompile(`pts_time:\s*([0-9]+(?:\.[0-9]+)?)`)

// DetectScenes trả danh sách phân đoạn của video theo chuyển cảnh hình ảnh.
// threshold <= 0 → 0.35; minSceneSec <= 0 → 2 giây.
func DetectScenes(ctx context.Context, src string, threshold, minSceneSec float64) ([]Scene, error) {
	if threshold <= 0 {
		threshold = defaultSceneThreshold
	}
	if minSceneSec <= 0 {
		minSceneSec = defaultMinSceneSec
	}
	info, err := Probe(src)
	if err != nil {
		return nil, err
	}
	if info.Duration <= 0 {
		return nil, fmt.Errorf("file không có thời lượng hợp lệ: %s", src)
	}

	// HAI máy dò trong MỘT lượt giải mã, vì bộ so cảnh của ffmpeg chỉ nhìn
	// kênh sáng: hai màu khác hẳn nhau nhưng cùng độ sáng (đo thật: đỏ thuần
	// Y≈81, xanh lá thuần Y≈80) cho điểm 0.000 — cú chuyển cảnh tàng hình.
	//   nhánh [l]: so kênh sáng như chuẩn;
	//   nhánh [c]: ghép chồng hai kênh màu U/V thành một khung xám rồi so —
	//              bắt được các cú "đổi màu không đổi sáng".
	// Mốc của hai nhánh hợp lại, khử trùng trong ParseSceneCuts.
	// +eq(n,0): mỗi nhánh luôn cho KHUNG HÌNH ĐẦU đi qua làm mồi. Không có nó,
	// khi một nhánh thấy mốc còn nhánh kia không thấy gì, ffmpeg chết EINVAL vì
	// muxer nhận một stream rỗng ("received no packets"). Khung mồi có pts 0
	// nên ParseSceneCuts bỏ qua — không thành mốc cắt.
	th := strconv.FormatFloat(threshold, 'f', -1, 64)
	fc := fmt.Sprintf(
		"[0:v]format=yuv420p,split=2[l][c];"+
			"[l]select='gt(scene,%s)+eq(n,0)',showinfo[lo];"+
			"[c]extractplanes=u+v[cu][cv];[cu][cv]vstack=inputs=2,select='gt(scene,%s)+eq(n,0)',showinfo[co]",
		th, th)
	_, se, err := util.RunErr(ctx, "ffmpeg", "-hide_banner", "-i", src,
		"-filter_complex", fc, "-map", "[lo]", "-map", "[co]", "-an", "-f", "null", "-")
	if err != nil {
		return nil, fmt.Errorf("phân tích chuyển cảnh thất bại: %w — %s", err, tail(se, 400))
	}
	cuts := ParseSceneCuts(se)

	return scenesFromCuts(cuts, info.Duration, minSceneSec), nil
}

// ParseSceneCuts đọc mốc chuyển cảnh từ stderr của showinfo. Hàm thuần để
// kiểm thử được bằng stderr thu sẵn, không cần chạy ffmpeg.
func ParseSceneCuts(stderr string) []float64 {
	var cuts []float64
	for _, m := range reScenePts.FindAllStringSubmatch(stderr, -1) {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil && v > 0 {
			cuts = append(cuts, v)
		}
	}
	sort.Float64s(cuts)
	// Hai máy dò cùng thấy một cú chuyển thì mốc có thể lệch nhau 1 khung hình
	// — coi các mốc cách nhau dưới 0.25s là một.
	out := cuts[:0]
	last := -1.0
	for _, c := range cuts {
		if c-last > 0.25 {
			out = append(out, c)
			last = c
		}
	}
	return out
}

// scenesFromCuts dựng danh sách cảnh từ các mốc cắt: cảnh ngắn hơn minSceneSec
// được GỘP vào cảnh trước (bỏ mốc cắt mở ra nó) — không vứt đoạn phim nào.
func scenesFromCuts(cuts []float64, duration, minSceneSec float64) []Scene {
	var scenes []Scene
	start := 0.0
	for _, c := range cuts {
		if c <= start || c >= duration {
			continue
		}
		if c-start < minSceneSec {
			continue // cảnh quá ngắn — gộp vào cảnh đang mở
		}
		scenes = append(scenes, Scene{Start: start, End: c})
		start = c
	}
	// đoạn cuối: ngắn quá thì nhập vào cảnh trước thay vì đứng riêng
	if duration-start < minSceneSec && len(scenes) > 0 {
		scenes[len(scenes)-1].End = duration
	} else {
		scenes = append(scenes, Scene{Start: start, End: duration})
	}
	for i := range scenes {
		scenes[i].Index = i
	}
	return scenes
}

// ExtractSceneFrames trích một khung hình đại diện (giữa cảnh) cho từng cảnh
// vào outDir, ghi đường dẫn vào Scene.Frame. maxW > 0 thì thu nhỏ bề ngang
// (ảnh đưa cho AI xem không cần 4K).
func ExtractSceneFrames(ctx context.Context, src string, scenes []Scene, outDir string, maxW int) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("tạo thư mục khung hình: %w", err)
	}
	for i := range scenes {
		mid := scenes[i].Start + scenes[i].Duration()/2
		dst := filepath.Join(outDir, fmt.Sprintf("scene-%03d.jpg", scenes[i].Index))
		args := []string{"-y", "-hide_banner",
			"-ss", fmt.Sprintf("%.3f", mid), "-i", src,
			"-frames:v", "1", "-q:v", "3"}
		if maxW > 0 {
			args = append(args, "-vf", fmt.Sprintf("scale='min(%d,iw)':-2", maxW))
		}
		if err := run(ctx, append(args, dst)...); err != nil {
			return fmt.Errorf("trích khung hình cảnh %d: %w", scenes[i].Index, err)
		}
		scenes[i].Frame = dst
	}
	return nil
}

// MergeToMaxScenes gộp dần hai cảnh LIỀN KỀ có tổng thời lượng nhỏ nhất cho tới
// khi còn tối đa max cảnh. Trả số lần gộp — người gọi PHẢI nói cho người dùng
// biết (không gộp âm thầm rồi tỏ ra danh sách vốn ngắn như vậy).
func MergeToMaxScenes(scenes []Scene, max int) ([]Scene, int) {
	if max <= 0 || len(scenes) <= max {
		return scenes, 0
	}
	merged := 0
	for len(scenes) > max {
		best, bestDur := 0, -1.0
		for i := 0; i+1 < len(scenes); i++ {
			d := scenes[i].Duration() + scenes[i+1].Duration()
			if bestDur < 0 || d < bestDur {
				best, bestDur = i, d
			}
		}
		scenes[best].End = scenes[best+1].End
		scenes = append(scenes[:best+1], scenes[best+2:]...)
		merged++
	}
	for i := range scenes {
		scenes[i].Index = i
	}
	return scenes, merged
}

// SaveScenesJSON / LoadScenesJSON — lưu danh sách cảnh cạnh video nguồn.
func SaveScenesJSON(path string, scenes []Scene) error {
	raw, err := json.MarshalIndent(scenes, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func LoadScenesJSON(path string) ([]Scene, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("không đọc được danh sách cảnh: %w", err)
	}
	var scenes []Scene
	if err := json.Unmarshal(raw, &scenes); err != nil {
		return nil, fmt.Errorf("danh sách cảnh không đúng định dạng: %w", err)
	}
	return scenes, nil
}
