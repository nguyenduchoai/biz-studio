package text2video

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"bizstudio/internal/htmlvideo"
	"bizstudio/internal/store"
)

const (
	minSceneSeconds = 1.2 // sàn thời lượng một cảnh (đoạn quá ngắn vẫn kịp hiện chữ)
	maxBulletItems  = 4
	maxTitleRunes   = 64
	maxBulletRunes  = 90
	maxQuoteRunes   = 220
	maxPhotoRunes   = 120 // chữ đè trên ảnh: tối đa 3 dòng nên phải ngắn
)

// BuildVideoHTML dựng video bằng engine HTML Video: mỗi đoạn kịch bản là một
// cảnh có thời lượng đúng bằng thời lượng THẬT của đoạn giọng đọc, sau đó THAY
// audio của video bằng voice.wav để hình bám sát giọng đã đọc.
// Trả về đường dẫn tuyệt đối file MP4.
func BuildVideoHTML(ctx context.Context, st *store.Store, sess *store.T2VSession, workDir string, upd func(float64, string)) (string, error) {
	if upd == nil {
		upd = func(float64, string) {}
	}
	if sess == nil {
		return "", errors.New("thiếu phiên Text → Video")
	}
	if len(sess.Segments) == 0 {
		return "", errors.New("chưa có kịch bản để dựng video")
	}
	voicePath := filepath.Join(workDir, VoiceFile)
	if _, err := os.Stat(voicePath); err != nil {
		return "", errors.New("chưa có file giọng đọc voice.wav — hãy chạy bước Giọng đọc trước khi dựng video")
	}

	cfg := htmlvideo.Config{
		Aspect:    aspectOf(sess.Width, sess.Height),
		Theme:     "vivid",
		FPS:       sess.FPS,
		Narration: false, // giọng đọc đã có sẵn (voice.wav), không đọc lại
	}
	scenes := buildScenes(sess.Segments, dataDirOf(workDir, sess.ID))

	// htmlvideo.Render báo tiến độ 0..100 → nén về 0..90, chừa chỗ cho bước ghép giọng.
	mp4, err := htmlvideo.Render(ctx, st, scenes, cfg, workDir, func(p float64, d string) {
		upd(p*0.9, d)
	})
	if err != nil {
		return "", err
	}

	upd(92, "Ghép giọng đọc vào video…")
	out := filepath.Join(workDir, VideoFile)
	if err := replaceAudio(ctx, mp4, voicePath, out); err != nil {
		return "", err
	}
	_ = os.Remove(mp4) // bản render trung gian (chưa có giọng) — chỉ xoá khi đã ghép xong
	upd(99, "Hoàn tất video")
	if abs, aerr := filepath.Abs(out); aerr == nil {
		return abs, nil
	}
	return out, nil
}

// replaceAudio thay toàn bộ audio của video bằng file giọng đọc (giữ nguyên
// stream hình, cắt theo bên ngắn hơn để hình bám đúng giọng).
func replaceAudio(ctx context.Context, video, audio, dst string) error {
	if err := runFFmpeg(ctx, "-y", "-i", video, "-i", audio,
		"-map", "0:v", "-map", "1:a",
		"-c:v", "copy", "-c:a", "aac", "-b:a", "192k",
		"-shortest", "-movflags", "+faststart", dst); err != nil {
		return fmt.Errorf("ghép giọng đọc vào video thất bại: %w", err)
	}
	return nil
}

// aspectOf suy tỉ lệ khung hình của htmlvideo từ kích thước phiên.
func aspectOf(w, h int) string {
	switch {
	case w <= 0 || h <= 0:
		return "9:16"
	case w > h:
		return "16:9"
	case w == h:
		return "1:1"
	default:
		return "9:16"
	}
}

// buildScenes map mỗi đoạn kịch bản thành một cảnh HTML: đoạn có ảnh storyboard
// dùng template "photo" (ảnh tràn khung + chữ đè), đoạn chưa có ảnh giữ nguyên
// cách cũ — đoạn đầu "hero", đoạn cuối "outro", ở giữa xen kẽ "bullets"/"quote".
// dataDir dùng để đổi ImagePath (tương đối DataDir) thành đường dẫn tuyệt đối.
func buildScenes(segs []store.T2VSegment, dataDir string) []htmlvideo.Scene {
	n := len(segs)
	scenes := make([]htmlvideo.Scene, 0, n)
	for i, s := range segs {
		text := strings.TrimSpace(s.Text)
		sc := htmlvideo.Scene{
			VoiceText: text, // dùng để sinh file .srt cạnh video
			Duration:  sceneDuration(s),
		}
		if img := shotImage(dataDir, s); img != "" {
			sc.Template = "photo"
			sc.Image = img
			sc.Title = photoTitle(text)
			scenes = append(scenes, sc)
			continue
		}
		// Mỗi đoạn là một câu nói: chỉ dùng bullets khi đoạn THỰC SỰ có nhiều câu,
		// nếu không tiêu đề và gạch đầu dòng sẽ lặp lại cùng một nội dung.
		sentences := splitSentences(text)
		switch role := segRole(s, i, n); {
		case role == roleHook:
			sc.Template = "hero"
			sc.Title, sc.Subtitle = headTail(text)
		case role == roleCTA:
			sc.Template = "outro"
			sc.Title, sc.Subtitle = headTail(text)
		case len(sentences) >= 2:
			sc.Template = "bullets"
			sc.Title = shortText(sentences[0], maxTitleRunes)
			sc.Bullets = bulletsOf(strings.Join(sentences[1:], " "))
		default:
			sc.Template = "quote"
			sc.Title = shortText(text, maxQuoteRunes)
		}
		scenes = append(scenes, sc)
	}
	return scenes
}

// Vai của cảnh trong mạch video.
const (
	roleHook = "hook" // câu mở giữ chân người xem
	roleBody = "body" // nội dung chính
	roleCTA  = "cta"  // kêu gọi hành động, chốt lại
)

// RoleOf — bản công khai của segRole, cho lớp trên và bài kiểm tra dùng lại.
func RoleOf(s store.T2VSegment, i, n int) string { return segRole(s, i, n) }

// segRole trả vai của đoạn. Đoạn chưa gán vai thì suy theo vị trí ĐÚNG NHƯ
// cách làm trước khi có vai — nhờ vậy các phiên cũ render lại không đổi hình.
func segRole(s store.T2VSegment, i, n int) string {
	switch strings.ToLower(strings.TrimSpace(s.Role)) {
	case roleHook:
		return roleHook
	case roleCTA:
		return roleCTA
	case roleBody:
		return roleBody
	}
	if i == 0 {
		return roleHook
	}
	if i == n-1 && n > 1 {
		return roleCTA
	}
	return roleBody
}

// NormalizeRole chuẩn hoá vai do người dùng/AI đặt; không hợp lệ → rỗng (tự suy).
func NormalizeRole(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case roleHook:
		return roleHook
	case roleBody:
		return roleBody
	case roleCTA:
		return roleCTA
	}
	return ""
}

// shotImage trả đường dẫn TUYỆT ĐỐI ảnh storyboard của đoạn; ảnh chưa có hoặc
// file đã bị xoá → "" để cảnh quay về template chữ như trước.
func shotImage(dataDir string, s store.T2VSegment) string {
	p := strings.TrimSpace(s.ImagePath)
	if p == "" {
		return ""
	}
	abs := absFromData(dataDir, p)
	if !fileExists(abs) {
		return ""
	}
	if full, err := filepath.Abs(abs); err == nil {
		return full
	}
	return abs
}

// photoTitle rút gọn lời đọc thành câu chữ đè trên ảnh: ưu tiên giữ trọn câu,
// quá dài mới cắt theo mệnh đề (chữ trên ảnh không bao giờ đứt giữa chừng).
func photoTitle(text string) string {
	t := strings.TrimSpace(text)
	if len([]rune(t)) <= maxPhotoRunes {
		return t
	}
	if ss := splitSentences(t); len(ss) > 0 && len([]rune(ss[0])) <= maxPhotoRunes {
		return ss[0]
	}
	head, _ := splitClause(t, maxPhotoRunes)
	return head
}

// dataDirOf suy DataDir từ thư mục phiên (phép ngược của dataRelPath); workDir
// không theo chuẩn data/text2video/<id> → "" (khi đó ImagePath đã là tuyệt đối).
func dataDirOf(workDir, sessionID string) string {
	if filepath.Base(workDir) == sessionID && filepath.Base(filepath.Dir(workDir)) == DirName {
		return filepath.Dir(filepath.Dir(workDir))
	}
	return ""
}

// sceneDuration lấy ĐÚNG thời lượng thật của đoạn giọng đọc (không làm tròn,
// không áp sàn — mọi sai lệch đều làm hình lệch khỏi giọng ở các đoạn sau).
// Chỉ khi chưa đo được mới ước lượng theo số ký tự.
func sceneDuration(s store.T2VSegment) float64 {
	if s.Seconds > 0 {
		return s.Seconds
	}
	chars := s.Chars
	if chars <= 0 {
		chars = len([]rune(s.Text))
	}
	d := float64(chars) / charsPerSecond
	if d < minSceneSeconds {
		d = minSceneSeconds
	}
	return d
}

// headTail tách câu đầu làm tiêu đề, phần còn lại làm phụ đề.
// Đoạn chỉ có 1 câu: cắt theo mệnh đề thay vì chặt cụt bằng "…" — chữ trên
// màn hình luôn là câu hoàn chỉnh, không bao giờ đứt giữa chừng.
func headTail(text string) (string, string) {
	sentences := splitSentences(text)
	if len(sentences) == 0 {
		return shortText(text, maxTitleRunes), ""
	}
	if len(sentences) == 1 {
		return splitClause(sentences[0], maxTitleRunes)
	}
	title := shortText(sentences[0], maxTitleRunes)
	sub := strings.TrimSpace(strings.Join(sentences[1:], " "))
	return title, shortText(sub, 140)
}

// splitClause tách một câu dài thành (tiêu đề, phần còn lại) tại dấu phẩy gần
// giới hạn nhất, để không phải cắt cụt chữ. Câu đủ ngắn thì giữ nguyên.
func splitClause(s string, limit int) (string, string) {
	s = strings.TrimSpace(s)
	if len([]rune(s)) <= limit {
		return s, ""
	}
	runes := []rune(s)
	cut := -1
	for i, r := range runes {
		if i >= limit {
			break
		}
		if r == ',' || r == ';' || r == ':' {
			cut = i
		}
	}
	if cut <= 0 {
		// Không có dấu phẩy: cắt ở khoảng trắng cuối cùng trước giới hạn.
		for i := limit - 1; i > 0; i-- {
			if runes[i] == ' ' {
				cut = i
				break
			}
		}
	}
	if cut <= 0 {
		return shortText(s, limit), ""
	}
	head := strings.TrimRight(strings.TrimSpace(string(runes[:cut])), ",;:")
	tail := strings.TrimSpace(string(runes[cut+1:]))
	return head, shortText(tail, 140)
}

// bulletsOf tách đoạn thành các gạch đầu dòng ngắn (tối đa 4 ý).
func bulletsOf(text string) []string {
	sentences := splitSentences(text)
	if len(sentences) <= 1 {
		sentences = splitLongPhrase(text)
	}
	out := make([]string, 0, maxBulletItems)
	for _, s := range sentences {
		t := shortText(strings.TrimSpace(s), maxBulletRunes)
		if t == "" {
			continue
		}
		out = append(out, t)
		if len(out) >= maxBulletItems {
			break
		}
	}
	if len(out) == 0 {
		out = append(out, shortText(text, maxBulletRunes))
	}
	return out
}

var reSentenceEnd = regexp.MustCompile(`([.!?…])\s+`)

// splitSentences tách câu theo dấu kết câu (giữ lại dấu).
func splitSentences(text string) []string {
	marked := reSentenceEnd.ReplaceAllString(strings.TrimSpace(text), "$1\n")
	var out []string
	for _, s := range strings.Split(marked, "\n") {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// splitLongPhrase tách một câu dài theo dấu phẩy / chấm phẩy khi không có nhiều câu.
func splitLongPhrase(text string) []string {
	parts := strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == ';' })
	var out []string
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return []string{strings.TrimSpace(text)}
	}
	return out
}
