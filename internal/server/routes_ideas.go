package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"bizstudio/internal/ideas"
	"bizstudio/internal/store"
)

const (
	// ideaGenTimeout — trần thời gian chờ AI đề xuất ý tưởng.
	ideaGenTimeout = 10 * time.Minute
	// ideaBusyHint — ý tưởng đang được hàng đợi sản xuất, không cho sửa/xoá.
	ideaBusyHint = "ý tưởng %q đang được sản xuất — bấm Dừng hàng đợi trước khi sửa hoặc xoá"
)

// ideaQueue — Runner hàng đợi sản xuất, singleton cho cả server (giống aiRunner
// ở routes_sessions.go). Chỉ được có MỘT bộ chạy: hai bộ sẽ tranh nhau Chrome,
// ffmpeg và TTS rồi hỏng cả loạt video.
var (
	ideaQueue     *ideas.Runner
	ideaQueueOnce sync.Once
)

func (s *Server) ideaRunner() *ideas.Runner {
	ideaQueueOnce.Do(func() {
		ideaQueue = ideas.NewRunner(s.st, s.DataDir, s.Hub.Broadcast)
	})
	return ideaQueue
}

// routesIdeas — Ý tưởng & Hàng đợi sản xuất: AI đề xuất ý tưởng video, người
// duyệt, hệ thống tự sản xuất tuần tự từ kịch bản tới video hoàn chỉnh.
func (s *Server) routesIdeas(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/ideas", s.handleIdeaList)
	mux.HandleFunc("POST /api/ideas", s.handleIdeaCreate)
	mux.HandleFunc("POST /api/ideas/generate", s.handleIdeaGenerate)
	mux.HandleFunc("GET /api/ideas/queue", s.handleIdeaQueueState)
	mux.HandleFunc("POST /api/ideas/queue/start", s.handleIdeaQueueStart)
	mux.HandleFunc("POST /api/ideas/queue/stop", s.handleIdeaQueueStop)
	mux.HandleFunc("PUT /api/ideas/{id}", s.handleIdeaUpdate)
	mux.HandleFunc("DELETE /api/ideas/{id}", s.handleIdeaDelete)
	mux.HandleFunc("POST /api/ideas/{id}/approve", s.handleIdeaApprove)
	mux.HandleFunc("POST /api/ideas/{id}/reject", s.handleIdeaReject)
	mux.HandleFunc("POST /api/ideas/{id}/queue", s.handleIdeaQueueAdd)
	s.routesIdeasRetry(mux)
}

// handleIdeaList — GET /api/ideas (mới nhất trước).
func (s *Server) handleIdeaList(w http.ResponseWriter, r *http.Request) {
	list := s.st.Ideas()
	if list == nil {
		list = []store.Idea{}
	}
	writeJSON(w, http.StatusOK, list)
}

// handleIdeaCreate — POST /api/ideas: tự thêm một ý tưởng (đã duyệt sẵn).
func (s *Server) handleIdeaCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title    string   `json:"title"`
		Angle    string   `json:"angle"`
		Hook     string   `json:"hook"`
		Keywords []string `json:"keywords"`
		Width    int      `json:"width"`
		Height   int      `json:"height"`
		FPS      int      `json:"fps"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	title := ideaLine(body.Title)
	if title == "" {
		httpErr(w, http.StatusBadRequest, "thiếu tiêu đề ý tưởng")
		return
	}
	idea := store.Idea{
		Title:    title,
		Angle:    ideaLine(body.Angle),
		Hook:     ideaLine(body.Hook),
		Keywords: ideas.NormalizeKeywords(body.Keywords),
		Status:   "approved",
	}
	idea.Width, idea.Height, idea.FPS = ideaSize(body.Width, body.Height, body.FPS)
	s.saveIdea(&idea)
	s.Log("info", "ideas", fmt.Sprintf("Đã thêm ý tưởng %q", shortText(idea.Title, 60)))
	writeJSON(w, http.StatusOK, idea)
}

// handleIdeaGenerate — POST /api/ideas/generate: job kind=idea_gen, AI đề xuất
// hàng loạt ý tưởng cho một chủ đề / kênh.
func (s *Server) handleIdeaGenerate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Topic    string `json:"topic"`
		Count    int    `json:"count"`
		Audience string `json:"audience"`
		Tone     string `json:"tone"`
		Width    int    `json:"width"`
		Height   int    `json:"height"`
		FPS      int    `json:"fps"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	topic := strings.TrimSpace(body.Topic)
	if topic == "" {
		httpErr(w, http.StatusBadRequest, "chưa có chủ đề — nhập chủ đề hoặc mô tả kênh trước khi sinh ý tưởng")
		return
	}
	count := ideas.ClampCount(body.Count)
	audience, tone := strings.TrimSpace(body.Audience), strings.TrimSpace(body.Tone)
	width, height, fps := ideaSize(body.Width, body.Height, body.FPS)

	j := s.Jobs.Submit("idea_gen", "", "Sinh ý tưởng: "+shortText(topic, 40),
		func(upd func(float64, string)) (string, error) {
			ctx, cancel := context.WithTimeout(context.Background(), ideaGenTimeout)
			defer cancel()
			upd(15, fmt.Sprintf("Đang nhờ AI đề xuất %d ý tưởng…", count))

			list, err := ideas.Generate(ctx, s.st, topic, count, audience, tone)
			if err != nil {
				s.Log("error", "ideas", fmt.Sprintf("Sinh ý tưởng cho %q thất bại: %v", shortText(topic, 60), err))
				return "", err
			}
			for i := range list {
				list[i].Width, list[i].Height, list[i].FPS = width, height, fps
				s.saveIdea(&list[i])
			}
			upd(98, fmt.Sprintf("Đã có %d ý tưởng", len(list)))
			s.Log("info", "ideas", fmt.Sprintf("Đã sinh %d ý tưởng cho chủ đề %q", len(list), shortText(topic, 60)))
			return fmt.Sprintf("%d ý tưởng mới", len(list)), nil
		})
	writeJSON(w, http.StatusOK, j)
}

// handleIdeaUpdate — PUT /api/ideas/{id}: sửa nội dung, trạng thái, khung hình.
// Field không gửi thì giữ nguyên.
func (s *Server) handleIdeaUpdate(w http.ResponseWriter, r *http.Request) {
	idea, ok := s.ideaByID(w, r)
	if !ok || !s.ideaEditable(w, idea) {
		return
	}
	var body struct {
		Title    *string   `json:"title"`
		Angle    *string   `json:"angle"`
		Hook     *string   `json:"hook"`
		Keywords *[]string `json:"keywords"`
		Status   *string   `json:"status"`
		Width    *int      `json:"width"`
		Height   *int      `json:"height"`
		FPS      *int      `json:"fps"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	if body.Title != nil {
		title := ideaLine(*body.Title)
		if title == "" {
			httpErr(w, http.StatusBadRequest, "thiếu tiêu đề ý tưởng")
			return
		}
		idea.Title = title
	}
	if body.Status != nil {
		status := strings.ToLower(strings.TrimSpace(*body.Status))
		if !ideas.ValidStatus(status) {
			httpErr(w, http.StatusBadRequest, "trạng thái không hợp lệ: %q "+
				"(chỉ nhận proposed, approved, rejected, queued, producing, done, error)", *body.Status)
			return
		}
		idea.Status = status
	}
	if body.Angle != nil {
		idea.Angle = ideaLine(*body.Angle)
	}
	if body.Hook != nil {
		idea.Hook = ideaLine(*body.Hook)
	}
	if body.Keywords != nil {
		idea.Keywords = ideas.NormalizeKeywords(*body.Keywords)
	}
	w2, h2, fps2 := idea.Width, idea.Height, idea.FPS
	if body.Width != nil {
		w2 = *body.Width
	}
	if body.Height != nil {
		h2 = *body.Height
	}
	if body.FPS != nil {
		fps2 = *body.FPS
	}
	idea.Width, idea.Height, idea.FPS = ideaSize(w2, h2, fps2)

	s.saveIdea(&idea)
	if idea.Status == "queued" {
		s.ideaRunner().Wake()
	}
	writeJSON(w, http.StatusOK, idea)
}

// handleIdeaDelete — DELETE /api/ideas/{id} (phiên Text → Video đã tạo vẫn giữ
// lại ở trang Text → Video, không xoá theo).
func (s *Server) handleIdeaDelete(w http.ResponseWriter, r *http.Request) {
	idea, ok := s.ideaByID(w, r)
	if !ok || !s.ideaEditable(w, idea) {
		return
	}
	s.st.DeleteIdea(idea.ID)
	s.Log("info", "ideas", fmt.Sprintf("Đã xoá ý tưởng %q", shortText(idea.Title, 60)))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleIdeaApprove — POST /api/ideas/{id}/approve.
func (s *Server) handleIdeaApprove(w http.ResponseWriter, r *http.Request) {
	s.setIdeaStatus(w, r, "approved", "Đã duyệt ý tưởng")
}

// handleIdeaReject — POST /api/ideas/{id}/reject.
func (s *Server) handleIdeaReject(w http.ResponseWriter, r *http.Request) {
	s.setIdeaStatus(w, r, "rejected", "Đã bỏ ý tưởng")
}

// handleIdeaQueueAdd — POST /api/ideas/{id}/queue: xếp ý tưởng vào hàng đợi sản xuất.
func (s *Server) handleIdeaQueueAdd(w http.ResponseWriter, r *http.Request) {
	idea, ok := s.ideaByID(w, r)
	if !ok || !s.ideaEditable(w, idea) {
		return
	}
	switch {
	case idea.Status == "queued": // đã ở trong hàng đợi — bấm lại không phải lỗi
	case idea.Status == "done":
		httpErr(w, http.StatusBadRequest,
			"ý tưởng %q đã dựng xong video — sửa trạng thái về \"đề xuất\" nếu muốn làm lại", shortText(idea.Title, 60))
		return
	case !ideas.CanQueue(idea.Status):
		httpErr(w, http.StatusBadRequest,
			"ý tưởng %q đang ở trạng thái %q, không đưa vào hàng đợi được", shortText(idea.Title, 60), idea.Status)
		return
	default:
		idea.Status, idea.Error = "queued", ""
		s.saveIdea(&idea)
		s.Log("info", "ideas", fmt.Sprintf("Đã đưa ý tưởng %q vào hàng đợi sản xuất", shortText(idea.Title, 60)))
	}
	s.ideaRunner().Wake()
	writeJSON(w, http.StatusOK, idea)
}

// ---------- hàng đợi ----------

// ideaQueueState — trạng thái hàng đợi trả cho FE.
type ideaQueueState struct {
	Running       bool   `json:"running"`
	CurrentIdeaID string `json:"currentIdeaId"`
	Queued        int    `json:"queued"`
	Producing     int    `json:"producing"`
}

// handleIdeaQueueState — GET /api/ideas/queue.
func (s *Server) handleIdeaQueueState(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.queueState())
}

// handleIdeaQueueStart — POST /api/ideas/queue/start.
func (s *Server) handleIdeaQueueStart(w http.ResponseWriter, r *http.Request) {
	s.ideaRunner().Start()
	writeJSON(w, http.StatusOK, s.queueState())
}

// handleIdeaQueueStop — POST /api/ideas/queue/stop.
func (s *Server) handleIdeaQueueStop(w http.ResponseWriter, r *http.Request) {
	s.ideaRunner().Stop()
	writeJSON(w, http.StatusOK, s.queueState())
}

// queueState gom trạng thái runner + số ý tưởng đang chờ / đang chạy.
func (s *Server) queueState() ideaQueueState {
	running, current := s.ideaRunner().Status()
	st := ideaQueueState{Running: running, CurrentIdeaID: current}
	for _, idea := range s.st.Ideas() {
		switch idea.Status {
		case "queued":
			st.Queued++
		case "producing":
			st.Producing++
		}
	}
	return st
}

// ---------- helpers ----------

// ideaByID lấy ý tưởng theo path param, tự trả 404 tiếng Việt nếu không có.
func (s *Server) ideaByID(w http.ResponseWriter, r *http.Request) (store.Idea, bool) {
	id := r.PathValue("id")
	idea, ok := s.st.Idea(id)
	if !ok {
		httpErr(w, http.StatusNotFound, "không tìm thấy ý tưởng %q", id)
		return store.Idea{}, false
	}
	return idea, true
}

// ideaEditable chặn sửa/xoá ý tưởng mà hàng đợi ĐANG sản xuất — sửa giữa chừng
// sẽ làm trạng thái và kết quả lệch nhau. Bản ghi "producing" cũ còn sót lại sau
// khi tắt app thì vẫn cho sửa (runner không còn chạy ý tưởng đó nữa).
func (s *Server) ideaEditable(w http.ResponseWriter, idea store.Idea) bool {
	if _, current := s.ideaRunner().Status(); current == idea.ID {
		httpErr(w, http.StatusBadRequest, ideaBusyHint, shortText(idea.Title, 60))
		return false
	}
	return true
}

// setIdeaStatus đổi trạng thái ý tưởng (duyệt / bỏ) và ghi nhật ký.
func (s *Server) setIdeaStatus(w http.ResponseWriter, r *http.Request, status, logMsg string) {
	idea, ok := s.ideaByID(w, r)
	if !ok || !s.ideaEditable(w, idea) {
		return
	}
	idea.Status = status
	s.saveIdea(&idea)
	s.Log("info", "ideas", fmt.Sprintf("%s %q", logMsg, shortText(idea.Title, 60)))
	writeJSON(w, http.StatusOK, idea)
}

// saveIdea lưu ý tưởng và phát SSE "idea" để giao diện cập nhật ngay.
func (s *Server) saveIdea(idea *store.Idea) {
	s.st.SaveIdea(idea)
	s.Hub.Broadcast("idea", *idea)
}

// ideaLine gộp mọi khoảng trắng (kể cả xuống dòng) thành một dấu cách — tiêu đề
// và hook đều hiển thị trên một dòng.
func ideaLine(v string) string { return strings.Join(strings.Fields(v), " ") }

// ideaSize áp mặc định khung hình dọc 1080×1920@30 khi FE không gửi.
func ideaSize(w, h, fps int) (int, int, int) {
	if w <= 0 || h <= 0 {
		w, h = 1080, 1920
	}
	if fps <= 0 {
		fps = 30
	}
	return w, h, fps
}
