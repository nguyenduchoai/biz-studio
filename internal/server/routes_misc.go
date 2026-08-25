package server

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	qrcode "github.com/skip2/go-qrcode"

	"bizstudio/internal/store"
	"bizstudio/internal/util"
)

func (s *Server) routesMisc(mux *http.ServeMux) {
	// ---------- Prompts CRUD ----------
	mux.HandleFunc("GET /api/prompts", func(w http.ResponseWriter, r *http.Request) {
		list := s.st.Prompts()
		if list == nil {
			list = []store.PromptTemplate{}
		}
		writeJSON(w, http.StatusOK, list)
	})

	mux.HandleFunc("POST /api/prompts", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name string `json:"name"`
			Body string `json:"body"`
		}
		if err := readJSON(r, &body); err != nil {
			httpErr(w, http.StatusBadRequest, "%s", err)
			return
		}
		if strings.TrimSpace(body.Name) == "" {
			httpErr(w, http.StatusBadRequest, "thiếu tên prompt")
			return
		}
		p := store.PromptTemplate{Name: strings.TrimSpace(body.Name), Body: body.Body}
		s.st.SavePrompt(&p)
		writeJSON(w, http.StatusOK, p)
	})

	mux.HandleFunc("PUT /api/prompts/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !promptExists(s.st, id) {
			httpErr(w, http.StatusNotFound, "không tìm thấy prompt: %s", id)
			return
		}
		var body struct {
			Name string `json:"name"`
			Body string `json:"body"`
		}
		if err := readJSON(r, &body); err != nil {
			httpErr(w, http.StatusBadRequest, "%s", err)
			return
		}
		if strings.TrimSpace(body.Name) == "" {
			httpErr(w, http.StatusBadRequest, "thiếu tên prompt")
			return
		}
		p := store.PromptTemplate{ID: id, Name: strings.TrimSpace(body.Name), Body: body.Body}
		s.st.SavePrompt(&p)
		writeJSON(w, http.StatusOK, p)
	})

	mux.HandleFunc("DELETE /api/prompts/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !promptExists(s.st, id) {
			httpErr(w, http.StatusNotFound, "không tìm thấy prompt: %s", id)
			return
		}
		s.st.DeletePrompt(id)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})

	// ---------- Logs ----------
	mux.HandleFunc("GET /api/logs", func(w http.ResponseWriter, r *http.Request) {
		limit := 200
		if v := r.URL.Query().Get("limit"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 {
				httpErr(w, http.StatusBadRequest, "tham số limit không hợp lệ: %s", v)
				return
			}
			limit = n
		}
		logs := s.st.Logs(limit)
		if logs == nil {
			logs = []store.LogEntry{}
		}
		writeJSON(w, http.StatusOK, logs)
	})

	// ---------- Jobs ----------
	mux.HandleFunc("GET /api/jobs", func(w http.ResponseWriter, r *http.Request) {
		jobs := s.st.Jobs()
		if jobs == nil {
			jobs = []store.Job{}
		}
		writeJSON(w, http.StatusOK, jobs)
	})

	mux.HandleFunc("GET /api/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		j, ok := s.st.Job(r.PathValue("id"))
		if !ok {
			httpErr(w, http.StatusNotFound, "không tìm thấy tác vụ: %s", r.PathValue("id"))
			return
		}
		writeJSON(w, http.StatusOK, j)
	})

	// ---------- QR kết nối điện thoại ----------
	mux.HandleFunc("GET /api/qr.png", func(w http.ResponseWriter, r *http.Request) {
		pid := strings.TrimSpace(r.URL.Query().Get("project"))
		if pid == "" {
			httpErr(w, http.StatusBadRequest, "thiếu tham số project")
			return
		}
		if _, ok := s.st.Project(pid); !ok {
			httpErr(w, http.StatusNotFound, "không tìm thấy dự án: %s", pid)
			return
		}
		if s.MobilePort <= 0 {
			httpErr(w, http.StatusServiceUnavailable, "nhận file từ điện thoại đang tắt; kiểm tra kết nối mạng rồi khởi động lại Biz Studio")
			return
		}
		lanIP := util.LanIP()
		if ip := net.ParseIP(lanIP); ip == nil || ip.IsLoopback() {
			httpErr(w, http.StatusServiceUnavailable, "chưa tìm thấy địa chỉ Wi-Fi/LAN; hãy kết nối cùng mạng với điện thoại rồi thử lại")
			return
		}
		target := fmt.Sprintf("http://%s:%d/m/%s?token=%s", lanIP, s.MobilePort, pid, s.mobileTokenFor(pid))
		png, err := qrcode.Encode(target, qrcode.Medium, 280)
		if err != nil {
			httpErr(w, http.StatusInternalServerError, "tạo mã QR thất bại: %v", err)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(png)
	})

}

func promptExists(st *store.Store, id string) bool {
	for _, p := range st.Prompts() {
		if p.ID == id {
			return true
		}
	}
	return false
}
