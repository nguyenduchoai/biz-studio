package server

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"bizstudio/internal/jobs"
	"bizstudio/internal/store"
	"bizstudio/internal/updater"
	"bizstudio/internal/util"
)

// Server — HTTP server chính của Biz Studio.
type Server struct {
	st            *store.Store
	mux           *http.ServeMux
	Hub           *Hub
	Jobs          *jobs.Manager
	DataDir       string
	Port          int
	MobilePort    int
	DataDirID     string
	Platform      string
	mobileSecret  string
	mobileTokenMu sync.Mutex
	mobileUsed    map[string]time.Time
	mobileUploads chan struct{}
	setupPlanMu   sync.Mutex
	setupPlans    map[string]fullSetupGrant
	Updater       *updater.Manager
}

func New(st *store.Store, dataDir string, port, mobilePort int) *Server {
	abs, _ := filepath.Abs(dataDir)
	s := &Server{
		st:            st,
		mux:           http.NewServeMux(),
		Hub:           NewHub(),
		DataDir:       abs,
		Port:          port,
		MobilePort:    mobilePort,
		DataDirID:     util.DataDirID(abs),
		Platform:      runtime.GOOS,
		mobileSecret:  newMobileSecret(),
		mobileUsed:    make(map[string]time.Time),
		mobileUploads: make(chan struct{}, 2),
		setupPlans:    make(map[string]fullSetupGrant),
		Updater:       updater.New(Version, abs),
	}
	s.Jobs = jobs.New(st, s.Hub.Broadcast)

	s.routesState(s.mux)
	s.routesStatic(s.mux)
	s.routesSettings(s.mux)
	s.routesSetup(s.mux)
	s.routesUpdate(s.mux)
	s.routesProjects(s.mux)
	s.routesAssets(s.mux)
	s.routesSessions(s.mux)
	s.routesTools(s.mux)
	s.routesLook(s.mux)
	s.routesModels(s.mux)
	s.routesCharsBible(s.mux)
	s.routesStudio(s.mux)
	s.routesHighlight(s.mux)
	s.routesCollections(s.mux)
	s.routesTimeline(s.mux)
	s.routesBroll(s.mux)
	s.routesHTMLVideo(s.mux)
	s.routesClone(s.mux)
	s.routesDubbing(s.mux)
	s.routesT2V(s.mux)
	s.routesStyle(s.mux)
	s.routesChars(s.mux)
	s.routesIdeas(s.mux)
	s.routesMisc(s.mux)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !allowControlRequest(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, mobileUploadMaxBody)
	if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodDelete {
		s.mux.ServeHTTP(w, r)
		return
	}
	buffered := newBufferedResponse()
	s.mux.ServeHTTP(buffered, r)
	if persistenceErr := s.st.PersistenceError(); persistenceErr != "" {
		httpErr(w, http.StatusInsufficientStorage, "không lưu được dữ liệu: %s", persistenceErr)
		return
	}
	buffered.flushTo(w)
}

type bufferedResponse struct {
	header      http.Header
	body        bytes.Buffer
	status      int
	wroteHeader bool
}

func newBufferedResponse() *bufferedResponse {
	return &bufferedResponse{header: make(http.Header), status: http.StatusOK}
}

func (w *bufferedResponse) Header() http.Header { return w.header }

func (w *bufferedResponse) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status, w.wroteHeader = status, true
}

func (w *bufferedResponse) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.body.Write(body)
}

func (w *bufferedResponse) flushTo(dst http.ResponseWriter) {
	for key, values := range w.header {
		for _, value := range values {
			dst.Header().Add(key, value)
		}
	}
	dst.WriteHeader(w.status)
	_, _ = dst.Write(w.body.Bytes())
}

func newMobileSecret() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic("không tạo được token QR: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// ProjectDir trả (và tạo) thư mục dự án data/projects/<id>.
func (s *Server) ProjectDir(id string) string {
	dir := filepath.Join(s.DataDir, "projects", id)
	for _, sub := range []string{"assets", "outputs", "publish", "tmp"} {
		_ = os.MkdirAll(filepath.Join(dir, sub), 0o755)
	}
	return dir
}

// Log ghi nhật ký + phát SSE.
func (s *Server) Log(level, module, msg string) {
	e := s.st.AddLog(level, module, msg)
	s.Hub.Broadcast("log", e)
}
