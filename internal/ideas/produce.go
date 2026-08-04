package ideas

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bizstudio/internal/store"
	"bizstudio/internal/text2video"
)

// produceTimeout — trần thời gian sản xuất MỘT ý tưởng. Giọng đọc và Chrome
// render có thể rất lâu, nhưng không bao giờ được treo mãi mãi vì như vậy cả
// hàng đợi đứng im.
const produceTimeout = 3 * time.Hour

// produce sản xuất trọn vẹn một ý tưởng: kịch bản → giọng đọc → storyboard →
// dựng video. Mọi đường thoát (thành công, lỗi, bị dừng, panic) đều đặt trạng
// thái cuối cùng cho ý tưởng — không bao giờ để kẹt ở "producing".
func (r *Runner) produce(lp *loop, idea store.Idea) {
	ctx, cancel := context.WithTimeout(context.Background(), produceTimeout)
	r.mu.Lock()
	lp.cancel, lp.ideaID = cancel, idea.ID
	r.mu.Unlock()
	defer func() {
		cancel()
		r.mu.Lock()
		lp.cancel, lp.ideaID = nil, ""
		r.mu.Unlock()
	}()
	// Stop() có thể xảy ra ngay TRƯỚC khi cancel được gắn vào loop ở trên; kiểm
	// tra lại để không bắt đầu sản xuất sau khi người dùng đã bấm Dừng.
	if stopped(lp.stop) {
		cancel()
	}
	// Panic trong module con (ffmpeg/Chrome/TTS) không được để ý tưởng kẹt.
	defer func() {
		if rec := recover(); rec != nil {
			r.finish(lp, idea.ID, fmt.Errorf("lỗi nghiêm trọng khi sản xuất: %v", rec), "")
		}
	}()

	title := shortText(idea.Title, 60)
	idea.Status, idea.Error, idea.OutputPath = "producing", "", ""
	idea.Attempts++
	r.save(&idea)
	if idea.Attempts > 1 {
		r.logf("info", fmt.Sprintf("Sản xuất lại ý tưởng %q (lần thứ %d)", title, idea.Attempts))
	} else {
		r.logf("info", fmt.Sprintf("Bắt đầu sản xuất ý tưởng %q", title))
	}

	out, err := r.pipeline(ctx, &idea, title)
	r.finish(lp, idea.ID, err, out)
}

// finish ghi trạng thái cuối cùng của ý tưởng. Đọc lại bản mới nhất trong store
// để không đè lên chỉnh sửa của người dùng trong lúc sản xuất; ý tưởng đã bị xoá
// thì bỏ qua.
func (r *Runner) finish(lp *loop, id string, err error, out string) {
	idea, ok := r.st.Idea(id)
	if !ok {
		if err != nil {
			r.logf("warn", fmt.Sprintf("Ý tưởng %s đã bị xoá khi đang sản xuất: %v", id, err))
		}
		return
	}
	title := shortText(idea.Title, 60)
	if err != nil {
		msg := err.Error()
		if stopped(lp.stop) {
			msg = "đã dừng hàng đợi khi đang sản xuất — " + msg
		}
		idea.Status, idea.Error = "error", msg
		r.save(&idea)
		r.logf("error", fmt.Sprintf("Ý tưởng %q lỗi: %s", title, msg))
		return
	}
	idea.Status, idea.Error, idea.OutputPath = "done", "", out
	r.save(&idea)
	r.logf("info", fmt.Sprintf("Ý tưởng %q đã dựng xong video: %s", title, out))
}

// pipeline chạy đủ 4 bước sản xuất, trả đường dẫn video (tương đối DataDir).
func (r *Runner) pipeline(ctx context.Context, idea *store.Idea, title string) (string, error) {
	sess, err := r.prepareSession(idea)
	if err != nil {
		return "", err
	}
	workDir := text2video.SessionDir(r.dataDir, sess.ID)

	// 1. Kịch bản lời đọc.
	r.logf("info", fmt.Sprintf("Ý tưởng %q: đang viết kịch bản…", title))
	segs, err := text2video.WriteScript(ctx, r.st, sess.SourceText, sess.ScriptEngine, sess.ScriptModel, sess.TargetSeconds)
	if err != nil {
		r.failSession(&sess)
		return "", fmt.Errorf("viết kịch bản thất bại: %w", err)
	}
	sess.Segments, sess.Status, sess.Step = segs, "script", 2
	r.st.SaveT2VSession(&sess)
	r.logf("info", fmt.Sprintf("Ý tưởng %q: đã có kịch bản %d đoạn", title, len(segs)))

	// 2. Giọng đọc (đo thời lượng thật từng đoạn).
	r.logf("info", fmt.Sprintf("Ý tưởng %q: đang tạo giọng đọc…", title))
	if err := text2video.BuildVoice(ctx, r.st, &sess, workDir, nil); err != nil {
		r.failSession(&sess)
		return "", fmt.Errorf("tạo giọng đọc thất bại: %w", err)
	}
	sess.Status, sess.Step = "voice", 3
	r.st.SaveT2VSession(&sess)
	r.logf("info", fmt.Sprintf("Ý tưởng %q: giọng đọc dài %.1f giây", title, sess.VoiceSeconds))

	// 3. Storyboard — thiếu API key ảnh thì BỎ QUA, video vẫn dựng được bằng chữ.
	if err := r.storyboard(ctx, &sess, title); err != nil {
		r.failSession(&sess)
		return "", err
	}

	// 4. Dựng video bằng HTML Video.
	r.logf("info", fmt.Sprintf("Ý tưởng %q: đang dựng video…", title))
	sess.Status = "building"
	r.st.SaveT2VSession(&sess)
	mp4, err := text2video.BuildVideoHTML(ctx, r.st, &sess, workDir, nil)
	if err != nil {
		r.failSession(&sess)
		return "", fmt.Errorf("dựng video thất bại: %w", err)
	}
	sess.OutputPath, sess.Status, sess.Step = r.relPath(mp4), "done", 5
	r.st.SaveT2VSession(&sess)
	return sess.OutputPath, nil
}

// storyboard sinh ảnh từng cảnh. Lỗi nguồn ảnh (chưa có Gemini / Pexels key, hết
// hạn mức…) KHÔNG làm hỏng cả ý tưởng: bỏ qua và dựng video bằng chữ như cũ.
// Chỉ trả lỗi khi việc sản xuất bị hủy (Stop / hết thời gian).
func (r *Runner) storyboard(ctx context.Context, sess *store.T2VSession, title string) error {
	r.logf("info", fmt.Sprintf("Ý tưởng %q: đang tạo ảnh cho từng cảnh…", title))
	err := text2video.BuildStoryboard(ctx, r.st, sess, text2video.SessionDir(r.dataDir, sess.ID), nil)
	r.st.SaveT2VSession(sess) // giữ lại ảnh + mô tả cảnh đã làm được
	if err == nil {
		r.logf("info", fmt.Sprintf("Ý tưởng %q: đã có ảnh cho %d/%d cảnh", title, shotCount(sess), len(sess.Segments)))
		return nil
	}
	if cerr := ctx.Err(); cerr != nil {
		return fmt.Errorf("tạo storyboard bị dừng: %w", cerr)
	}
	r.logf("warn", fmt.Sprintf("Ý tưởng %q: bỏ qua storyboard (%v) — video vẫn được dựng bằng chữ", title, err))
	return nil
}

// prepareSession tạo (hoặc dùng lại) phiên Text → Video của ý tưởng: nguồn là
// chính tiêu đề + góc tiếp cận + hook + từ khóa đã duyệt.
func (r *Runner) prepareSession(idea *store.Idea) (store.T2VSession, error) {
	w, h, fps := idea.Width, idea.Height, idea.FPS
	if w <= 0 || h <= 0 {
		w, h = 1080, 1920
	}
	if fps <= 0 {
		fps = 30
	}
	// Chạy lại một ý tưởng lỗi thì dùng lại phiên cũ thay vì đẻ thêm phiên rác.
	sess, ok := r.st.T2VSession(idea.T2VSessionID)
	if !ok {
		sess = store.T2VSession{}
	}
	sess.Name = sessionName(idea.Title)
	sess.SourceKind = "text"
	sess.SourceText = SourceText(*idea)
	sess.Width, sess.Height, sess.FPS = w, h, fps
	sess.Status, sess.Step, sess.BuildMode = "draft", 1, "html"
	sess.Segments = []store.T2VSegment{}
	sess.OutputPath = ""
	r.st.SaveT2VSession(&sess) // gán ID khi là phiên mới

	if err := os.MkdirAll(text2video.SessionDir(r.dataDir, sess.ID), 0o755); err != nil {
		return sess, fmt.Errorf("không tạo được thư mục phiên Text → Video: %w", err)
	}
	if idea.T2VSessionID != sess.ID {
		idea.T2VSessionID = sess.ID
		if cur, ok := r.st.Idea(idea.ID); ok {
			cur.T2VSessionID = sess.ID
			r.save(&cur)
		}
	}
	return sess, nil
}

// failSession đánh dấu phiên Text → Video lỗi để trang Text → Video không hiển
// thị nhầm là đang chạy dở.
func (r *Runner) failSession(sess *store.T2VSession) {
	sess.Status = "error"
	r.st.SaveT2VSession(sess)
}

// sessionName đặt tên phiên theo tiêu đề ý tưởng.
func sessionName(title string) string {
	t := strings.TrimSpace(title)
	if t == "" {
		return "Ý tưởng chưa đặt tên"
	}
	return shortText(t, 80)
}

// shotCount đếm số cảnh đã có ảnh.
func shotCount(sess *store.T2VSession) int {
	n := 0
	for _, seg := range sess.Segments {
		if strings.TrimSpace(seg.ImagePath) != "" {
			n++
		}
	}
	return n
}

// relPath đổi đường dẫn file thành tương đối DataDir (FE mở qua /data/…); file
// nằm ngoài DataDir thì giữ nguyên đường dẫn tuyệt đối.
func (r *Runner) relPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	rel, err := filepath.Rel(r.dataDir, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return abs
	}
	return rel
}
