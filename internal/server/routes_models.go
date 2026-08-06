package server

import (
	"context"
	"net/http"
	"time"

	"bizstudio/internal/gemini"
)

// Nạp danh sách model từ chính API của Google.
//
// Vì sao cần: danh sách model đổi liên tục — model mới ra, model preview bị gỡ.
// Gõ tay hoặc để sẵn danh sách trong mã nguồn thì chỉ đúng vào ngày viết, sau
// đó người dùng chọn phải model không còn tồn tại và nhận lỗi khó hiểu.

func (s *Server) routesModels(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/tools/models", s.handleListModels)
}

// handleListModels — GET /api/tools/models. Lệnh ĐỌC, không tốn tiền.
func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	groups, err := gemini.NewFromSettings(s.st).ListModels(ctx)
	if err != nil {
		httpErr(w, http.StatusBadRequest, "%v", err)
		return
	}
	writeJSON(w, http.StatusOK, groups)
}
