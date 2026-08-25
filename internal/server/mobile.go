package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"bizstudio/web"
)

const (
	mobileTokenTTL      = 15 * time.Minute
	mobileUploadMaxBody = int64(10 << 30)
	mobileUploadMaxFile = 50
)

// MobileHandler chỉ công khai đúng hai nghiệp vụ cần cho QR: mở trang chọn
// file và tải file vào dự án. Không gắn bất kỳ /api/* nào vào mux này.
func (s *Server) MobileHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /m/{projectID}", s.requireMobileToken(s.handleMobilePage))
	mux.HandleFunc("POST /m/{projectID}/upload", s.requireMobileToken(s.handleMobileUpload))
	return mux
}

func (s *Server) requireMobileToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.validMobileToken(r.PathValue("projectID"), r.URL.Query().Get("token"), time.Now()) {
			http.Error(w, "QR không hợp lệ hoặc đã hết hạn", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func (s *Server) mobileTokenFor(projectID string) string {
	return s.mobileTokenForAt(projectID, time.Now())
}

func (s *Server) mobileTokenForAt(projectID string, now time.Time) string {
	expires := now.Add(mobileTokenTTL).Unix()
	expiresText := strconv.FormatInt(expires, 10)
	nonceBytes := make([]byte, 8)
	if _, err := rand.Read(nonceBytes); err != nil {
		panic("không tạo được nonce QR: " + err.Error())
	}
	nonce := base64.RawURLEncoding.EncodeToString(nonceBytes)
	mac := hmac.New(sha256.New, []byte(s.mobileSecret))
	_, _ = mac.Write([]byte(projectID + "\n" + expiresText + "\n" + nonce))
	return expiresText + "." + nonce + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Server) validMobileToken(projectID, token string, now time.Time) bool {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return false
	}
	expires, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || now.Unix() > expires || expires > now.Add(mobileTokenTTL+time.Minute).Unix() {
		return false
	}
	mac := hmac.New(sha256.New, []byte(s.mobileSecret))
	_, _ = mac.Write([]byte(projectID + "\n" + parts[0] + "\n" + parts[1]))
	want := []byte(base64.RawURLEncoding.EncodeToString(mac.Sum(nil)))
	got := []byte(parts[2])
	return len(got) == len(want) && subtle.ConstantTimeCompare(got, want) == 1
}

func (s *Server) handleMobilePage(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("projectID")
	if _, ok := s.st.Project(pid); !ok {
		httpErr(w, http.StatusNotFound, "không tìm thấy dự án: %s", pid)
		return
	}
	b, err := fs.ReadFile(web.FS, "static/mobile.html")
	if err != nil {
		httpErr(w, http.StatusInternalServerError, "không đọc được trang mobile: %v", err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	_, _ = w.Write(b)
}

func (s *Server) handleMobileUpload(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("projectID")
	if _, ok := s.st.Project(pid); !ok {
		httpErr(w, http.StatusNotFound, "không tìm thấy dự án: %s", pid)
		return
	}
	select {
	case s.mobileUploads <- struct{}{}:
		defer func() { <-s.mobileUploads }()
	default:
		httpErr(w, http.StatusTooManyRequests, "đang có 2 lượt upload; chờ xong rồi quét QR lại")
		return
	}
	token := r.URL.Query().Get("token")
	if !s.reserveMobileToken(token, time.Now()) {
		httpErr(w, http.StatusConflict, "QR này đã được dùng; hãy quét mã mới")
		return
	}
	success := false
	defer func() {
		if !success {
			s.releaseMobileToken(token)
		}
	}()
	_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(4 * time.Hour))
	r.Body = http.MaxBytesReader(w, r.Body, mobileUploadMaxBody)
	assets, err := saveUploadedAssets(s, pid, r)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httpErr(w, http.StatusRequestEntityTooLarge, "tổng dữ liệu vượt giới hạn 10 GB")
			return
		}
		httpErr(w, http.StatusBadRequest, "tải file lên thất bại: %v", err)
		return
	}
	success = true
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "count": len(assets)})
}

func (s *Server) reserveMobileToken(token string, now time.Time) bool {
	s.mobileTokenMu.Lock()
	defer s.mobileTokenMu.Unlock()
	for used, expires := range s.mobileUsed {
		if now.After(expires) {
			delete(s.mobileUsed, used)
		}
	}
	if _, exists := s.mobileUsed[token]; exists {
		return false
	}
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return false
	}
	expiresUnix, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false
	}
	s.mobileUsed[token] = time.Unix(expiresUnix, 0)
	return true
}

func (s *Server) releaseMobileToken(token string) {
	s.mobileTokenMu.Lock()
	delete(s.mobileUsed, token)
	s.mobileTokenMu.Unlock()
}
