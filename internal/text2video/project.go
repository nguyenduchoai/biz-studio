package text2video

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"bizstudio/internal/media"
	"bizstudio/internal/store"
)

// BuildProject tạo một dự án video từ phiên Text → Video để AI (Claude CLI) tự
// dựng hình: copy voice.wav + transcript.json vào assets/ và viết sẵn mô tả +
// yêu cầu edit bám theo giọng đọc. Trả về projectID.
//
// projectsDir là thư mục GỐC chứa dự án (data/projects) — dự án được tạo tại
// <projectsDir>/<projectID>/.
func BuildProject(ctx context.Context, st *store.Store, sess *store.T2VSession, projectsDir string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("tạo dự án bị hủy: %w", err)
	}
	if sess == nil {
		return "", errors.New("thiếu phiên Text → Video")
	}
	if len(sess.Segments) == 0 {
		return "", errors.New("chưa có kịch bản — hãy viết kịch bản trước khi dựng video")
	}
	if strings.TrimSpace(sess.VoicePath) == "" {
		return "", errors.New("chưa có giọng đọc — hãy chạy bước Giọng đọc trước khi dựng video")
	}
	dataDir := filepath.Dir(projectsDir)
	voiceSrc := absFromData(dataDir, sess.VoicePath)
	if _, err := os.Stat(voiceSrc); err != nil {
		return "", fmt.Errorf("không tìm thấy file giọng đọc %s — hãy chạy lại bước Giọng đọc", voiceSrc)
	}
	transcriptSrc := absFromData(dataDir, sess.TranscriptPath)

	w, h, fps := sess.Width, sess.Height, sess.FPS
	if w <= 0 || h <= 0 {
		w, h = 1080, 1920
	}
	if fps <= 0 {
		fps = 30
	}

	p := store.Project{
		Name:     projectName(sess),
		Kind:     "video",
		Width:    w,
		Height:   h,
		FPS:      fps,
		Status:   "draft",
		Tags:     []string{"text2video"},
		Keywords: []string{},
	}
	st.SaveProject(&p) // gán ID + CreatedAt

	projDir := filepath.Join(projectsDir, p.ID)
	for _, sub := range []string{"assets", "outputs", "publish", "tmp"} {
		if err := os.MkdirAll(filepath.Join(projDir, sub), 0o755); err != nil {
			return "", fmt.Errorf("không tạo được thư mục dự án %s: %w", projDir, err)
		}
	}

	if err := addAsset(st, p.ID, projDir, voiceSrc, VoiceFile, "audio", 0,
		"Giọng đọc đã tổng hợp — trục thời gian của video, không được sửa"); err != nil {
		return "", err
	}
	if transcriptSrc != "" {
		if _, err := os.Stat(transcriptSrc); err == nil {
			if err := addAsset(st, p.ID, projDir, transcriptSrc, TranscriptFile, "other", 1,
				"Mốc thời gian từng đoạn lời đọc (start/end tính bằng giây)"); err != nil {
				return "", err
			}
		}
	}

	p.BriefDesc = briefDesc(sess)
	p.EditPrompt = editPrompt(sess, w, h, fps)
	st.SaveProject(&p)
	return p.ID, nil
}

// addAsset copy file vào assets/ của dự án và ghi bản ghi Asset vào store.
func addAsset(st *store.Store, projectID, projDir, src, name, kind string, order int, desc string) error {
	dst := filepath.Join(projDir, "assets", name)
	if err := copyFile(src, dst); err != nil {
		return fmt.Errorf("copy %s vào dự án: %w", name, err)
	}
	a := store.Asset{
		ProjectID: projectID,
		Kind:      kind,
		Name:      name,
		Path:      path.Join("projects", projectID, "assets", name),
		Desc:      desc,
		Order:     order,
	}
	if fi, err := os.Stat(dst); err == nil {
		a.Size = fi.Size()
	}
	if kind == "audio" {
		if info, err := media.Probe(dst); err == nil {
			a.Duration = info.Duration
		}
	}
	st.SaveAsset(&a)
	return nil
}

// projectName đặt tên dự án theo tên phiên.
func projectName(sess *store.T2VSession) string {
	name := strings.TrimSpace(sess.Name)
	if name == "" {
		name = "Text → Video"
	}
	return shortText(name, 80)
}

// briefDesc mô tả nguồn + kịch bản để AI hiểu bối cảnh video.
func briefDesc(sess *store.T2VSession) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Video lời đọc dựng từ phiên Text → Video \"%s\".\n", strings.TrimSpace(sess.Name))
	if u := strings.TrimSpace(sess.SourceURL); u != "" {
		fmt.Fprintf(&b, "Nguồn nội dung: %s\n", u)
	}
	fmt.Fprintf(&b, "Kịch bản gồm %d đoạn, tổng thời lượng giọng đọc %.1f giây.\n\n", len(sess.Segments), sess.VoiceSeconds)
	b.WriteString("Nội dung lời đọc:\n")
	for i, s := range sess.Segments {
		fmt.Fprintf(&b, "%d. (%.1fs) %s\n", i+1, s.Seconds, s.Text)
	}
	return strings.TrimSpace(b.String())
}

// editPrompt viết yêu cầu edit: dựng hình bám sát voice.wav theo mốc transcript.json.
func editPrompt(sess *store.T2VSession, w, h, fps int) string {
	var b strings.Builder
	fmt.Fprintf(&b, `Dựng video minh họa BÁM SÁT giọng đọc đã có sẵn — tuyệt đối không tạo giọng đọc mới.

Nguyên liệu có sẵn trong thư mục dự án:
- assets/%s — file giọng đọc hoàn chỉnh, dài %.2f giây. Đây là TRỤC THỜI GIAN của video: không cắt, không tăng/giảm tốc, không đọc lại.
- assets/%s — mốc thời gian từng đoạn lời đọc, dạng {"language","duration","segments":[{"index","start","end","text"}]} (start/end tính bằng giây).

Yêu cầu:
1. Đọc assets/%s để biết mỗi đoạn lời đọc bắt đầu và kết thúc ở giây nào; dựng hình cho từng đoạn phủ ĐÚNG khoảng [start, end] của đoạn đó.
2. Hình minh họa mỗi đoạn bám sát nội dung "text" của đoạn: chữ lớn dễ đọc, nền gradient hoặc màu phẳng hiện đại, chuyển cảnh mượt. Có thể dùng ffmpeg drawtext/xfade, hoặc ảnh trong assets/ nếu có.
3. Audio cuối cùng LẤY NGUYÊN assets/%s (ffmpeg -i video -i assets/%s -map 0:v -map 1:a -c:v copy -shortest). Nếu thêm nhạc nền thì để âm lượng rất nhỏ (≤ 0,12) và không được lấn giọng đọc.
4. Tổng thời lượng video phải bằng thời lượng giọng đọc (%.2f giây), sai lệch tối đa 0,5 giây.
5. Kích thước video %dx%d, %d fps.
`, VoiceFile, sess.VoiceSeconds, TranscriptFile, TranscriptFile, VoiceFile, VoiceFile, sess.VoiceSeconds, w, h, fps)
	return strings.TrimSpace(b.String())
}
