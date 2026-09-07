package jobs

import (
	"fmt"
	"sync"
	"time"

	"bizstudio/internal/store"
)

// Broadcast — hàm phát SSE (event, data).
type Broadcast func(event string, data any)

// Manager — chạy tác vụ nền, tự cập nhật store + SSE.
type Manager struct {
	st       *store.Store
	pub      Broadcast
	mu       sync.Mutex
	pending  []queuedJob
	active   int
	limit    int
	queueCap int
	wake     chan struct{}
}

type queuedJob struct {
	job store.Job
	fn  func(upd func(progress float64, detail string)) (string, error)
}

func New(st *store.Store, pub Broadcast) *Manager {
	limit := st.Settings().Threads
	if limit < 1 {
		limit = 1
	}
	if limit > 16 {
		limit = 16
	}
	queueCap := limit * 4
	if queueCap < 4 {
		queueCap = 4
	}
	m := &Manager{st: st, pub: pub, limit: limit, queueCap: queueCap, wake: make(chan struct{}, 1)}
	m.reapStale()
	go m.schedule()
	return m
}

// reapStale đánh dấu các job còn "running"/"queued" từ lần chạy trước là đã hỏng.
//
// Job sống trong goroutine, tắt máy giữa chừng là goroutine biến mất nhưng bản
// ghi trong db.json vẫn ghi "running" mãi mãi. Người dùng mở lại app thấy một
// job đứng im ở 40% không bao giờ nhúc nhích, không biết chờ hay chạy lại — mà
// chờ thì chờ đến sáng cũng thế.
func (m *Manager) reapStale() int {
	n := 0
	for _, j := range m.st.Jobs() {
		if j.Status != "running" && j.Status != "queued" {
			continue
		}
		j.Status = "error"
		j.Error = "bị ngắt giữa chừng do thoát ứng dụng — chạy lại; " +
			"những bước nặng đã xong (bóc băng, chấm điểm) sẽ được dùng lại chứ không làm lại"
		m.st.SaveJob(&j)
		n++
	}
	return n
}

// Submit tạo job và chạy fn trong goroutine. fn gọi upd(progress 0..100, detail)
// để báo tiến độ; trả (output, err).
func (m *Manager) Submit(kind, projectID, detail string,
	fn func(upd func(progress float64, detail string)) (string, error)) *store.Job {

	j := &store.Job{Kind: kind, ProjectID: projectID, Status: "queued", Detail: detail}
	m.st.SaveJob(j)
	m.pub("job", *j)

	m.mu.Lock()
	if len(m.pending) >= m.queueCap {
		m.mu.Unlock()
		j.Status = "error"
		j.Error = "hàng đợi đã đầy — chờ các tác vụ hiện tại xong rồi thử lại"
		m.finish(j)
		return j
	}
	// The queued worker owns a value copy. HTTP handlers may safely marshal
	// the returned snapshot while execution updates its own job value.
	m.pending = append(m.pending, queuedJob{job: *j, fn: fn})
	m.mu.Unlock()
	m.notify()
	return j
}

func (m *Manager) run(item queuedJob) {
	j, fn := &item.job, item.fn
	defer func() {
		if r := recover(); r != nil {
			j.Status, j.Error = "error", fmt.Sprintf("panic: %v", r)
			m.finish(j)
		}
	}()
	j.Status = "running"
	m.st.SaveJob(j)
	m.pub("job", *j)

	lastPub := time.Now()
	upd := func(p float64, d string) {
		j.Progress = p
		if d != "" {
			j.Detail = d
		}
		if time.Since(lastPub) > 300*time.Millisecond {
			m.st.SaveJob(j)
			m.pub("job", *j)
			lastPub = time.Now()
		}
	}

	out, err := fn(upd)
	if err != nil {
		j.Status, j.Error = "error", err.Error()
		m.st.AddLog("error", j.Kind, err.Error())
	} else {
		j.Status, j.Progress, j.Output = "done", 100, out
	}
	m.finish(j)
}

func (m *Manager) finish(j *store.Job) {
	m.st.SaveJob(j)
	m.pub("job", *j)
}
