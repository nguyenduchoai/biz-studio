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
	return &Manager{st: st, pub: pub}
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
