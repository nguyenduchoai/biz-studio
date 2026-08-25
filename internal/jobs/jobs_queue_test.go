package jobs

import (
	"sync/atomic"
	"testing"
	"time"

	"bizstudio/internal/store"
)

func TestManagerBoundsConcurrentJobs(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := st.Settings()
	cfg.Threads = 1
	st.SaveSettings(cfg)
	m := New(st, func(string, any) {})
	release := make(chan struct{})
	var active, maxActive atomic.Int32
	work := func(func(float64, string)) (string, error) {
		n := active.Add(1)
		if n > maxActive.Load() {
			maxActive.Store(n)
		}
		<-release
		active.Add(-1)
		return "ok", nil
	}
	one := m.Submit("one", "", "", work)
	two := m.Submit("two", "", "", work)
	time.Sleep(150 * time.Millisecond)
	if got := maxActive.Load(); got != 1 {
		t.Fatalf("concurrency = %d, muốn 1", got)
	}
	close(release)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		a, _ := st.Job(one.ID)
		b, _ := st.Job(two.ID)
		if a.Status == "done" && b.Status == "done" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("jobs không hoàn tất sau khi nhả queue")
}

func TestManagerRejectsWhenBoundedQueueIsFull(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := st.Settings()
	cfg.Threads = 1
	st.SaveSettings(cfg)
	m := New(st, func(string, any) {})
	release := make(chan struct{})
	work := func(func(float64, string)) (string, error) { <-release; return "ok", nil }
	jobs := make([]*store.Job, 0, 6)
	for i := 0; i < 6; i++ {
		jobs = append(jobs, m.Submit("queued", "", "", work))
	}
	foundFull := false
	for _, job := range jobs {
		if current, ok := st.Job(job.ID); ok && current.Status == "error" {
			foundFull = true
		}
	}
	close(release)
	if !foundFull {
		t.Fatal("queue vượt capacity nhưng không job nào bị backpressure")
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		terminal := true
		for _, job := range jobs {
			current, _ := st.Job(job.ID)
			if current.Status == "queued" || current.Status == "running" {
				terminal = false
				break
			}
		}
		if terminal {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("queue chưa xả hết sau khi nhả worker")
}
