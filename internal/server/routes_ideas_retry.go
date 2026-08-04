package server

import (
	"fmt"
	"net/http"

	"bizstudio/internal/store"
)

// Thử lại hàng loạt cho hàng đợi sản xuất ý tưởng.
//
// Trước đây ý tưởng hỏng chỉ có cách sửa trạng thái từng cái một, mà lỗi hàng
// đợi thường hỏng theo cụm (hết lượt gọi AI, mất mạng, ổ đầy) — sửa tay hàng
// chục ý tưởng là việc không nên bắt người dùng làm.

func (s *Server) routesIdeasRetry(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/ideas/{id}/retry", s.handleIdeaRetry)
	mux.HandleFunc("POST /api/ideas/retry-failed", s.handleIdeaRetryFailed)
	mux.HandleFunc("POST /api/ideas/queue-pending", s.handleIdeaQueuePending)
}

// handleIdeaRetry — POST /api/ideas/{id}/retry: đưa MỘT ý tưởng hỏng về hàng đợi.
func (s *Server) handleIdeaRetry(w http.ResponseWriter, r *http.Request) {
	idea, ok := s.ideaByID(w, r)
	if !ok || !s.ideaEditable(w, idea) {
		return
	}
	if idea.Status != "error" {
		httpErr(w, http.StatusBadRequest,
			"ý tưởng %q đang ở trạng thái %q, không phải lỗi — không cần thử lại",
			shortText(idea.Title, 60), idea.Status)
		return
	}
	requeue(&idea)
	s.saveIdea(&idea)
	s.Log("info", "ideas", fmt.Sprintf("Thử lại ý tưởng %q (lần thứ %d)", shortText(idea.Title, 60), idea.Attempts+1))
	s.ideaRunner().Wake()
	writeJSON(w, http.StatusOK, idea)
}

// handleIdeaRetryFailed — POST /api/ideas/retry-failed: đưa MỌI ý tưởng hỏng
// về hàng đợi một lượt.
func (s *Server) handleIdeaRetryFailed(w http.ResponseWriter, r *http.Request) {
	n := s.requeueWhere(func(i store.Idea) bool { return i.Status == "error" })
	if n > 0 {
		s.Log("info", "ideas", fmt.Sprintf("Đã đưa %d ý tưởng hỏng trở lại hàng đợi", n))
		s.ideaRunner().Wake()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"requeued": n,
		"message":  requeueMessage(n, "ý tưởng hỏng"),
	})
}

// handleIdeaQueuePending — POST /api/ideas/queue-pending: đưa mọi ý tưởng ĐÃ
// DUYỆT nhưng chưa sản xuất vào hàng đợi. Duyệt xong quên bấm sản xuất là
// chuyện thường; đây là nút gom lại một lần.
func (s *Server) handleIdeaQueuePending(w http.ResponseWriter, r *http.Request) {
	n := s.requeueWhere(func(i store.Idea) bool { return i.Status == "approved" })
	if n > 0 {
		s.Log("info", "ideas", fmt.Sprintf("Đã đưa %d ý tưởng đã duyệt vào hàng đợi", n))
		s.ideaRunner().Wake()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"requeued": n,
		"message":  requeueMessage(n, "ý tưởng đã duyệt nhưng chưa sản xuất"),
	})
}

// requeueWhere đưa mọi ý tưởng thoả điều kiện về trạng thái chờ, trả số lượng.
func (s *Server) requeueWhere(match func(store.Idea) bool) int {
	n := 0
	for _, idea := range s.st.Ideas() {
		if !match(idea) {
			continue
		}
		requeue(&idea)
		s.saveIdea(&idea)
		n++
	}
	return n
}

// requeue đặt ý tưởng về trạng thái chờ và xoá lỗi cũ. Giữ nguyên Attempts —
// đó là thứ cho biết ý tưởng nào hỏng dai dẳng chứ không phải trục trặc một lần.
func requeue(idea *store.Idea) {
	idea.Status, idea.Error = "queued", ""
}

func requeueMessage(n int, what string) string {
	if n == 0 {
		return "Không có " + what + " nào."
	}
	return fmt.Sprintf("Đã đưa %d %s vào hàng đợi.", n, what)
}
