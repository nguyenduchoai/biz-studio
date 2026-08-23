package jobs

import (
	"fmt"
	"time"

	"bizstudio/internal/store"
)

// Broadcast — hàm phát SSE (event, data).
type Broadcast func(event string, data any)

// Manager — chạy tác vụ nền, tự cập nhật store + SSE.
type Manager struct {
	st  *store.Store
	pub Broadcast
}

func New(st *store.Store, pub Broadcast) *Manager {
	m := &Manager{st: st, pub: pub}
	m.reapStale()
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

	go func() {
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
			m.st.AddLog("error", kind, err.Error())
		} else {
			j.Status, j.Progress, j.Output = "done", 100, out
		}
		m.finish(j)
	}()
	return j
}

func (m *Manager) finish(j *store.Job) {
	m.st.SaveJob(j)
	m.pub("job", *j)
}
