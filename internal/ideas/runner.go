package ideas

import (
	"context"
	"fmt"
	"sync"
	"time"

	"bizstudio/internal/store"
)

const (
	// idlePoll — hàng đợi rỗng thì ngủ bấy nhiêu trước khi hỏi lại store.
	idlePoll = 2 * time.Second
	// stopWait — Stop() chờ tối đa bấy nhiêu để bước đang chạy dừng hẳn.
	stopWait = 30 * time.Second
	// stuckHint — ý tưởng còn kẹt "producing" từ lần chạy trước (app tắt giữa chừng).
	stuckHint = "bị gián đoạn khi đang sản xuất (ứng dụng khởi động lại) — đưa lại vào hàng đợi để chạy lại"
)

// Runner — bộ chạy hàng đợi sản xuất ý tưởng.
//
// TUẦN TỰ là bắt buộc: mỗi lúc chỉ MỘT ý tưởng được sản xuất. Chrome (HTML
// Video), ffmpeg và TTS đều nặng và dùng chung máy — chạy song song sẽ tranh tài
// nguyên và hỏng cả loạt video. Vì vậy Runner chỉ có đúng một vòng lặp, gọi Start
// nhiều lần cũng không sinh thêm vòng lặp thứ hai.
type Runner struct {
	st      *store.Store
	dataDir string
	pub     func(string, any)

	mu  sync.Mutex
	cur *loop // nil = hàng đợi đang tắt
}

// loop — trạng thái của một lần bật hàng đợi. Mỗi lần Start tạo một loop mới nên
// vòng lặp cũ (nếu còn đang thu dọn) không đụng vào vòng lặp mới.
type loop struct {
	stop chan struct{} // đóng khi Stop: ngừng nhận ý tưởng mới
	done chan struct{} // đóng khi vòng lặp đã thoát hẳn
	wake chan struct{} // đánh thức ngay khi có ý tưởng mới vào hàng đợi

	// cancel / ideaID mô tả ý tưởng đang chạy — chỉ đọc/ghi khi giữ Runner.mu.
	cancel context.CancelFunc
	ideaID string
}

// NewRunner tạo bộ chạy hàng đợi. broadcast có thể nil (khi đó không phát SSE).
func NewRunner(st *store.Store, dataDir string, broadcast func(string, any)) *Runner {
	if broadcast == nil {
		broadcast = func(string, any) {}
	}
	return &Runner{st: st, dataDir: dataDir, pub: broadcast}
}

// Start bật hàng đợi. Gọi lại khi đang chạy là KHÔNG-OP (không tạo vòng lặp thứ hai).
func (r *Runner) Start() {
	r.mu.Lock()
	if r.cur != nil {
		r.mu.Unlock()
		return
	}
	lp := &loop{
		stop: make(chan struct{}),
		done: make(chan struct{}),
		wake: make(chan struct{}, 1),
	}
	r.cur = lp
	r.mu.Unlock()

	r.clearStuck()
	r.logf("info", "Bật hàng đợi sản xuất ý tưởng")
	go r.run(lp)
}

// Stop dừng hàng đợi: không nhận ý tưởng mới và HỦY ý tưởng đang chạy (ý tưởng
// đó bị đánh dấu lỗi "đã dừng hàng đợi", đưa lại vào hàng đợi là chạy lại được).
// Chờ tối đa stopWait để vòng lặp thoát hẳn, tránh bỏ lại goroutine chạy ngầm.
func (r *Runner) Stop() {
	r.mu.Lock()
	lp := r.cur
	if lp == nil {
		r.mu.Unlock()
		return
	}
	select {
	case <-lp.stop: // đã yêu cầu dừng trước đó
	default:
		close(lp.stop)
	}
	if lp.cancel != nil {
		lp.cancel()
	}
	r.mu.Unlock()

	select {
	case <-lp.done:
	case <-time.After(stopWait):
		r.logf("warn", fmt.Sprintf("Hàng đợi chưa dừng hẳn sau %s — bước đang chạy sẽ tự kết thúc", stopWait))
	}
}

// Running — hàng đợi có đang bật không.
func (r *Runner) Running() bool {
	running, _ := r.Status()
	return running
}

// Status trả trạng thái hàng đợi và ID ý tưởng đang sản xuất ("" nếu đang rảnh).
func (r *Runner) Status() (bool, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cur == nil {
		return false, ""
	}
	return true, r.cur.ideaID
}

// Wake đánh thức vòng lặp ngay khi vừa có ý tưởng mới vào hàng đợi (khỏi phải
// chờ hết nhịp idlePoll). Hàng đợi đang tắt thì không làm gì.
func (r *Runner) Wake() {
	r.mu.Lock()
	lp := r.cur
	r.mu.Unlock()
	if lp == nil {
		return
	}
	select {
	case lp.wake <- struct{}{}:
	default: // đã có tín hiệu chờ sẵn
	}
}

// run — vòng lặp hàng đợi: lấy ý tưởng chờ cũ nhất, sản xuất xong mới lấy tiếp.
func (r *Runner) run(lp *loop) {
	defer close(lp.done)
	defer func() {
		if rec := recover(); rec != nil {
			r.logf("error", fmt.Sprintf("Hàng đợi gặp lỗi nghiêm trọng và đã dừng: %v", rec))
		}
		r.mu.Lock()
		if r.cur == lp {
			r.cur = nil
		}
		lp.cancel, lp.ideaID = nil, ""
		r.mu.Unlock()
		r.logf("info", "Đã dừng hàng đợi sản xuất ý tưởng")
	}()

	for {
		select {
		case <-lp.stop:
			return
		default:
		}
		idea, ok := r.st.NextQueuedIdea()
		if !ok {
			select {
			case <-lp.stop:
				return
			case <-lp.wake:
			case <-time.After(idlePoll):
			}
			continue
		}
		r.produce(lp, idea)
	}
}

// clearStuck gỡ các ý tưởng còn kẹt "producing" từ lần chạy trước (app tắt giữa
// chừng). Chỉ gọi lúc bật hàng đợi — khi đó chắc chắn không ý tưởng nào đang chạy.
func (r *Runner) clearStuck() {
	for _, idea := range r.st.Ideas() {
		if idea.Status != "producing" {
			continue
		}
		idea.Status, idea.Error = "error", stuckHint
		r.save(&idea)
		r.logf("warn", fmt.Sprintf("Ý tưởng %q %s", shortText(idea.Title, 60), stuckHint))
	}
}

// ---------- tiện ích dùng chung ----------

// save lưu ý tưởng và phát SSE để giao diện cập nhật ngay.
func (r *Runner) save(idea *store.Idea) {
	r.st.SaveIdea(idea)
	r.pub("idea", *idea)
}

// logf ghi nhật ký module "ideas" và phát SSE log.
func (r *Runner) logf(level, msg string) {
	e := r.st.AddLog(level, "ideas", msg)
	r.pub("log", e)
}

// stopped — vòng lặp đã được yêu cầu dừng chưa.
func stopped(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}
