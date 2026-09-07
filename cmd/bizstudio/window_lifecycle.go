package main

import (
	"log"
	"os"
	"os/exec"
	"time"

	"bizstudio/internal/desktop"
	"bizstudio/internal/server"
	"bizstudio/internal/store"
)

const browserHandoffGrace = 3 * time.Second

// Chromium can hand a URL to an existing profile process and exit immediately.
// That launcher exit is not a window-close event, especially after an update
// or a force-quit that left the browser alive. Keep the server available when
// ownership is uncertain; an error also opens the system browser as a fallback.
func windowProcessClosed(waitErr error, elapsed time.Duration) bool {
	return waitErr == nil && elapsed >= browserHandoffGrace
}

func quitWhenWindowClosed(cmd *exec.Cmd, st *store.Store, url string, launch windowLaunch) {
	err := cmd.Wait()
	if !windowProcessClosed(err, time.Since(launch.openedAt)) {
		if err != nil {
			log.Printf("Trình duyệt cửa sổ app dừng (%v) — mở lại bằng trình duyệt mặc định.", err)
			if fallbackErr := desktop.OpenDefault(url); fallbackErr != nil {
				log.Printf("Không mở được trình duyệt mặc định: %v. Mở Biz Studio tại %s.", fallbackErr, url)
			}
		}
		log.Printf("Trình duyệt đã chuyển cửa sổ sang tiến trình khác hoặc chưa xác nhận được đóng cửa sổ — giữ Biz Studio tại %s.", url)
		return
	}
	for {
		if launch.superseded() {
			log.Printf("Đã có lượt mở cửa sổ khác — giữ Biz Studio tại %s.", url)
			return
		}
		n := runningWork(st)
		installing := server.SetupInProgress()
		storageErr := st.PersistenceError()
		if n == 0 && !installing && storageErr == "" {
			log.Printf("Đã đóng cửa sổ — thoát Biz Studio.")
			os.Exit(0)
		}
		if storageErr != "" {
			log.Printf("Không thoát vì còn lỗi lưu dữ liệu chưa khắc phục: %s", storageErr)
		}
		log.Printf("Đã đóng cửa sổ nhưng còn %d tác vụ/phiên AI và trạng thái cài đặt=%t — giữ máy chủ ở %s. Xong hết sẽ tự thoát.", n, installing, url)
		time.Sleep(15 * time.Second)
	}
}

func runningWork(st *store.Store) int {
	n := 0
	for _, job := range st.Jobs() {
		if job.Status == "running" || job.Status == "queued" {
			n++
		}
	}
	for _, project := range st.Projects() {
		for _, session := range st.SessionsByProject(project.ID) {
			if session.Status == "running" {
				n++
			}
		}
	}
	for _, idea := range st.Ideas() {
		if idea.Status == "producing" {
			n++
		}
	}
	return n
}

// Only call after acquiring the exclusive instance lock, before any runner
// starts. The old processes are not managed by this new application instance;
// retain their history but do not leave the UI or close guard stuck running.
func recoverInterruptedWork(st *store.Store) {
	for _, project := range st.Projects() {
		interrupted := false
		for _, session := range st.SessionsByProject(project.ID) {
			if session.Status != "running" {
				continue
			}
			session.Status, session.EndedAt = "stopped", time.Now()
			st.SaveSession(&session)
			st.AddEvent(&store.SessionEvent{SessionID: session.ID, Type: "error", Payload: "Biz Studio đã khởi động lại khi phiên AI chưa kết thúc. Lịch sử và tài nguyên đã lưu vẫn được giữ; mở phiên để tiếp tục hoặc chạy lại."})
			interrupted = true
		}
		if interrupted && project.Status == "running" {
			project.Status = "draft"
			st.SaveProject(&project)
		}
	}
	for _, idea := range st.Ideas() {
		if idea.Status != "producing" {
			continue
		}
		idea.Status = "error"
		idea.Error = "Biz Studio đã khởi động lại khi đang sản xuất. Dữ liệu đã lưu vẫn được giữ; đưa ý tưởng vào hàng đợi để chạy lại."
		st.SaveIdea(&idea)
	}
}
