package jobs

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"bizstudio/internal/store"
)

func queueTestManager(t *testing.T, threads int) (*Manager, *store.Store) {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := st.Settings()
	cfg.Threads = threads
	st.SaveSettings(cfg)
	return New(st, func(string, any) {}), st
}

func nextJobStart(t *testing.T, started <-chan string) string {
	t.Helper()
	select {
	case name := <-started:
		return name
	case <-time.After(10 * time.Second):
		t.Fatal("eligible job never started")
		return ""
	}
}

func waitJobsTerminal(t *testing.T, st *store.Store) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		terminal := true
		for _, j := range st.Jobs() {
			if j.Status == "running" || j.Status == "queued" {
				terminal = false
			}
		}
		if terminal {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("jobs did not reach terminal state")
}

func TestProjectQueueSerializesFIFOWithoutBlockingOtherProjects(t *testing.T) {
	m, st := queueTestManager(t, 2)
	started := make(chan string, 4)
	firstRelease, secondRelease := make(chan struct{}), make(chan struct{})
	var firstOnce, secondOnce sync.Once
	unblockFirst := func() { firstOnce.Do(func() { close(firstRelease) }) }
	unblockSecond := func() { secondOnce.Do(func() { close(secondRelease) }) }
	t.Cleanup(func() { unblockFirst(); unblockSecond(); waitJobsTerminal(t, st) })
	m.Submit("first", "project-a", "", func(func(float64, string)) (string, error) { started <- "a1"; <-firstRelease; return "", nil })
	if got := nextJobStart(t, started); got != "a1" {
		t.Fatal(got)
	}
	m.Submit("second", "project-a", "", func(func(float64, string)) (string, error) { started <- "a2"; <-secondRelease; return "", nil })
	m.Submit("third", "project-a", "", func(func(float64, string)) (string, error) { started <- "a3"; return "", nil })
	m.Submit("other", "project-b", "", func(func(float64, string)) (string, error) { started <- "b"; return "", nil })
	if got := nextJobStart(t, started); got != "b" {
		t.Fatalf("started %s while project-a is busy; unrelated project-b should run", got)
	}
	unblockFirst()
	if got := nextJobStart(t, started); got != "a2" {
		t.Fatalf("per-project FIFO broken: %s", got)
	}
	unblockSecond()
	if got := nextJobStart(t, started); got != "a3" {
		t.Fatalf("per-project FIFO broken: %s", got)
	}
}

func TestSubmitReturnsSnapshotNotWorkerOwnedJob(t *testing.T) {
	m, st := queueTestManager(t, 1)
	returned := m.Submit("snapshot", "project-a", "queued", func(upd func(float64, string)) (string, error) { upd(80, "working"); return "output.mp4", nil })
	waitJobsTerminal(t, st)
	if returned.Status != "queued" || returned.Progress != 0 || returned.Output != "" || returned.Detail != "queued" {
		t.Fatalf("worker changed the HTTP response object: %+v", returned)
	}
}

func TestExternalProjectLeaseDoesNotOccupyWorkerAndWakesOnRelease(t *testing.T) {
	m, st := queueTestManager(t, 1)
	release, ok := st.ProjectWork.TryAcquire("project-a")
	if !ok {
		t.Fatal("external session failed to acquire project")
	}
	t.Cleanup(func() { release(); waitJobsTerminal(t, st) })
	started := make(chan string, 3)
	for _, name := range []string{"a1", "a2"} {
		name := name
		m.Submit(name, "project-a", "", func(func(float64, string)) (string, error) { started <- name; return "", nil })
	}
	m.Submit("other", "project-b", "", func(func(float64, string)) (string, error) { started <- "b"; return "", nil })
	if got := nextJobStart(t, started); got != "b" {
		t.Fatalf("external lease was ignored: %s", got)
	}
	release()
	for _, want := range []string{"a1", "a2"} {
		if got := nextJobStart(t, started); got != want {
			t.Fatalf("after external release got=%s want=%s", got, want)
		}
	}
}

func TestUnscopedJobsCanUseSeparateWorkers(t *testing.T) {
	m, st := queueTestManager(t, 2)
	started := make(chan string, 2)
	release := make(chan struct{})
	t.Cleanup(func() { close(release); waitJobsTerminal(t, st) })
	for _, name := range []string{"one", "two"} {
		name := name
		m.Submit(name, "", "", func(func(float64, string)) (string, error) { started <- name; <-release; return "", nil })
	}
	first, second := nextJobStart(t, started), nextJobStart(t, started)
	if first == second {
		t.Fatal("same unscoped job started twice")
	}
}

func TestProjectLeaseIsReleasedAfterFailureAndPanic(t *testing.T) {
	for _, failure := range []string{"error", "panic"} {
		t.Run(failure, func(t *testing.T) {
			m, st := queueTestManager(t, 1)
			started := make(chan string, 2)
			first := m.Submit("failure", "project-a", "", func(func(float64, string)) (string, error) {
				if failure == "panic" {
					panic("intentional regression fixture")
				}
				return "", fmt.Errorf("intentional regression fixture")
			})
			m.Submit("next", "project-a", "", func(func(float64, string)) (string, error) { started <- "next"; return "", nil })
			if got := nextJobStart(t, started); got != "next" {
				t.Fatal(got)
			}
			waitJobsTerminal(t, st)
			failed, _ := st.Job(first.ID)
			if failed.Status != "error" {
				t.Fatalf("failure not retained: %+v", failed)
			}
		})
	}
}

func TestBusyExternalProjectCannotCreateUnboundedPendingQueue(t *testing.T) {
	m, st := queueTestManager(t, 1)
	release, _ := st.ProjectWork.TryAcquire("project-a")
	t.Cleanup(func() { release(); waitJobsTerminal(t, st) })
	for i := 0; i < m.queueCap; i++ {
		job := m.Submit("queued", "project-a", "", func(func(float64, string)) (string, error) { return "", nil })
		if job.Status != "queued" {
			t.Fatalf("within pending bound rejected: %+v", job)
		}
	}
	rejected := m.Submit("overflow", "project-a", "", func(func(float64, string)) (string, error) { t.Error("rejected job executed"); return "", nil })
	if rejected.Status != "error" {
		t.Fatalf("queue accepted beyond its pending limit: %+v", rejected)
	}
}
