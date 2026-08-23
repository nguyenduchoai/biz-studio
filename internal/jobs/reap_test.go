package jobs

import (
	"strings"
	"testing"

	"bizstudio/internal/store"
)

// Job sống trong goroutine. Tắt máy giữa chừng là goroutine biến mất nhưng bản
// ghi trong db.json vẫn "running" mãi — người dùng mở lại thấy một job đứng im
// ở 40%, không biết chờ hay chạy lại.
func TestReapStaleMarksInterruptedJobs(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"running", "queued", "done", "error"} {
		j := &store.Job{Kind: "test-" + s, Status: s}
		st.SaveJob(j)
	}

	m := New(st, func(string, any) {})
	_ = m

	counts := map[string]int{}
	var msg string
	for _, j := range st.Jobs() {
		counts[j.Status]++
		if j.Kind == "test-running" {
			msg = j.Error
		}
	}
	if counts["running"] != 0 || counts["queued"] != 0 {
		t.Errorf("còn %d running + %d queued sau khi khởi động lại — job ma",
			counts["running"], counts["queued"])
	}
	if counts["error"] != 3 { // 2 job bị ngắt + 1 job vốn đã error
		t.Errorf("có %d job error, muốn 3", counts["error"])
	}
	if counts["done"] != 1 {
		t.Errorf("job đã xong bị đụng vào: còn %d", counts["done"])
	}
	// Thông báo phải nói được hai điều: vì sao hỏng, và chạy lại có mất công không.
	if !strings.Contains(msg, "ngắt giữa chừng") {
		t.Errorf("không nói vì sao hỏng: %q", msg)
	}
	if !strings.Contains(msg, "dùng lại") {
		t.Errorf("không nói chạy lại sẽ tận dụng bước đã xong: %q", msg)
	}
}

// Không có job nào thì đừng đụng gì — chạy được và không panic.
func TestReapStaleOnEmptyStore(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := New(st, func(string, any) {})
	if n := m.reapStale(); n != 0 {
		t.Errorf("store rỗng mà dọn %d job", n)
	}
}
