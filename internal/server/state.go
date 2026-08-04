package server

import (
	"net/http"
	"sync"
	"time"

	"bizstudio/internal/tts"
	"bizstudio/internal/util"
	"bizstudio/internal/whisper"
)

// Version — số phiên bản hiện trên giao diện. Script đóng gói ghi đè bằng
// -ldflags "-X bizstudio/internal/server.Version=1.8.0"; chạy từ source thì
// hiện "dev" chứ không hiện một con số cũ đã lạc hậu.
var Version = "dev"

var (
	toolsMu   sync.Mutex
	toolsAt   time.Time
	toolsLast map[string]bool
)

func (s *Server) toolsAvail() map[string]bool {
	toolsMu.Lock()
	defer toolsMu.Unlock()
	if time.Since(toolsAt) < 60*time.Second && toolsLast != nil {
		return toolsLast
	}
	cfg := s.st.Settings()
	toolsLast = map[string]bool{
		"ffmpeg":    util.Exists("ffmpeg"),
		"claude":    util.Exists(cfg.ClaudeBin),
		"ytdlp":     util.Exists(cfg.YtdlpBin),
		"geminiKey": cfg.GeminiAPIKey != "",
		"openaiKey": cfg.OpenAIKey != "",
		"pexelsKey": cfg.PexelsKey != "",
		"chrome":    findChromeBin(cfg) != "",
		"vieneu":    tts.VieNeuAvailable(s.st),
		"whisper":   whisper.Available(s.st),
	}
	toolsAt = time.Now()
	return toolsLast
}

func (s *Server) routesState(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) {
		running := 0
		for _, j := range s.st.Jobs() {
			if j.Status == "running" || j.Status == "queued" {
				running++
			}
		}
		writeJSON(w, 200, map[string]any{
			"app":    map[string]string{"name": "Biz Studio", "version": Version},
			"host":   util.Stats(),
			"tools":  s.toolsAvail(),
			"counts": map[string]int{"projects": len(s.st.Projects()), "jobsRunning": running},
			"lanIP":  util.LanIP(),
			"port":   s.Port,
		})
	})
	mux.Handle("GET /api/events/stream", s.Hub)
}
