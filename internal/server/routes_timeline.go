package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"bizstudio/internal/media"
	"bizstudio/internal/timeline"
	"bizstudio/internal/util"
)

const timelineRenderTimeout = 60 * time.Minute

// routesTimeline — timeline nhiều lớp âm thanh + phụ đề của một dự án.
func (s *Server) routesTimeline(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/projects/{id}/timeline", s.handleTimelineGet)
	mux.HandleFunc("PUT /api/projects/{id}/timeline", s.handleTimelinePut)
	mux.HandleFunc("POST /api/projects/{id}/timeline/render", s.handleTimelineRender)
	mux.HandleFunc("GET /api/timeline/peaks", s.handleTimelinePeaks)
}

// timelinePath — timeline lưu thành FILE RIÊNG trong thư mục dự án, không nhét
// vào db.json.
//
// db.json nạp hết vào bộ nhớ và ghi lại toàn bộ sau mỗi thay đổi. Một timeline
// có thể mang hàng trăm dòng phụ đề; nhân với số dự án là mỗi lần bấm lưu phải
// ghi lại vài megabyte cho một sửa đổi vài byte.
func (s *Server) timelinePath(projectID string) string {
	return filepath.Join(s.ProjectDir(projectID), "timeline.json")
}

func (s *Server) handleTimelineGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.st.Project(id); !ok {
		httpErr(w, http.StatusNotFound, "không có dự án %q", id)
		return
	}
	doc, err := s.loadTimeline(r.Context(), id)
	if err != nil {
		httpErr(w, http.StatusInternalServerError, "%s", err)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

// loadTimeline đọc timeline đã lưu, hoặc dựng một cái mặc định từ media của dự
// án nếu chưa có.
func (s *Server) loadTimeline(ctx context.Context, id string) (*timeline.Doc, error) {
	raw, err := os.ReadFile(s.timelinePath(id))
	if err == nil {
		var d timeline.Doc
		if json.Unmarshal(raw, &d) == nil {
			d.Normalize()
			return &d, nil
		}
		// File hỏng thì dựng lại từ đầu còn hơn để người dùng kẹt với một trang
		// trắng không mở được.
		s.Log("warn", "timeline", "timeline.json của dự án "+id+" đọc không được — dựng lại từ media")
	}
	return s.defaultTimeline(ctx, id), nil
}

// defaultTimeline dựng timeline khởi điểm: video dài nhất làm nền, mỗi file âm
// thanh một lớp đặt từ giây 0.
func (s *Server) defaultTimeline(ctx context.Context, id string) *timeline.Doc {
	d := &timeline.Doc{ProjectID: id}
	tracks := []timeline.Track{{ID: "src", Name: "Tiếng gốc", Role: timeline.RoleSource}}

	for _, a := range s.st.AssetsByProject(id) {
		abs := filepath.Join(s.DataDir, a.Path)
		switch a.Kind {
		case "video":
			if a.Duration > d.VideoDur {
				d.Video, d.VideoDur = a.Path, a.Duration
			}
		case "audio":
			dur := a.Duration
			if dur <= 0 {
				if info, err := media.Probe(abs); err == nil {
					dur = info.Duration
				}
			}
			tracks = append(tracks, timeline.Track{
				ID: "t-" + a.ID, Name: a.Name, Role: timeline.GuessRole(a.Name),
				Items: []timeline.Item{{
					ID: "i-" + a.ID, Name: a.Name, Path: a.Path, At: 0, In: 0, Out: dur,
				}},
			})
		}
	}
	d.Tracks = tracks
	d.Normalize()
	return d
}

func (s *Server) handleTimelinePut(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.st.Project(id); !ok {
		httpErr(w, http.StatusNotFound, "không có dự án %q", id)
		return
	}
	var d timeline.Doc
	if err := readJSON(r, &d); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	d.ProjectID = id
	d.UpdatedAt = time.Now()
	d.Normalize()

	raw, err := json.MarshalIndent(&d, "", " ")
	if err != nil {
		httpErr(w, http.StatusInternalServerError, "mã hoá timeline: %s", err)
		return
	}
	if err := os.WriteFile(s.timelinePath(id), raw, 0o644); err != nil {
		httpErr(w, http.StatusInternalServerError, "ghi timeline: %s", err)
		return
	}
	writeJSON(w, http.StatusOK, &d)
}

func (s *Server) handleTimelineRender(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	proj, ok := s.st.Project(id)
	if !ok {
		httpErr(w, http.StatusNotFound, "không có dự án %q", id)
		return
	}
	doc, err := s.loadTimeline(r.Context(), id)
	if err != nil {
		httpErr(w, http.StatusInternalServerError, "%s", err)
		return
	}
	if err := doc.Validate(); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}

	dir := s.ProjectDir(id)
	dst := filepath.Join(dir, "outputs", "timeline.mp4")

	j := s.Jobs.Submit("timeline", id, "Dựng timeline: "+proj.Name,
		func(upd func(float64, string)) (string, error) {
			ctx, cancel := context.WithTimeout(context.Background(), timelineRenderTimeout)
			defer cancel()

			// Đường dẫn trong tài liệu là tương đối data dir (để dự án chuyển máy
			// vẫn mở được); ffmpeg cần đường dẫn tuyệt đối.
			abs := *doc
			abs.Tracks = absolutizeTracks(s.DataDir, doc.Tracks)
			srcAbs := filepath.Join(s.DataDir, doc.Video)

			srt := ""
			if len(doc.Subs) > 0 {
				srt = filepath.Join(dir, "tmp", "timeline.srt")
				_ = os.MkdirAll(filepath.Dir(srt), 0o755)
				if err := timeline.WriteSRT(doc, srt); err != nil {
					return "", err
				}
			}

			upd(10, "Dựng lệnh trộn…")
			plan, err := timeline.BuildPlan(&abs, srcAbs, media.HasAudio(ctx, srcAbs), srt)
			if err != nil {
				return "", err
			}
			s.Log("info", "timeline", "filter: "+plan.Filter)

			upd(25, "ffmpeg đang trộn "+plan.Note+"…")
			if _, err := util.Run(ctx, "ffmpeg", append(plan.Args, dst)...); err != nil {
				return "", fmt.Errorf("trộn timeline thất bại: %w", err)
			}
			upd(96, plan.Note)
			s.Log("info", "timeline", "Dựng xong "+filepath.Base(dst)+" — "+plan.Note)
			return s.toolRelPath(dst), nil
		})
	writeJSON(w, http.StatusOK, j)
}

// absolutizeTracks đổi đường dẫn tương đối trong tài liệu thành tuyệt đối.
func absolutizeTracks(dataDir string, in []timeline.Track) []timeline.Track {
	out := make([]timeline.Track, len(in))
	copy(out, in)
	for ti := range out {
		items := make([]timeline.Item, len(out[ti].Items))
		copy(items, out[ti].Items)
		for i := range items {
			if !filepath.IsAbs(items[i].Path) {
				items[i].Path = filepath.Join(dataDir, items[i].Path)
			}
		}
		out[ti].Items = items
	}
	return out
}

// handleTimelinePeaks trả biên độ sóng âm để vẽ lên lớp.
func (s *Server) handleTimelinePeaks(w http.ResponseWriter, r *http.Request) {
	src, ok := s.toolSrcPath(w, r.URL.Query().Get("path"))
	if !ok {
		return
	}
	buckets := 400
	if v := strings.TrimSpace(r.URL.Query().Get("n")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 4000 {
			buckets = n
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	peaks, err := timeline.Peaks(ctx, src, buckets)
	if err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	// Làm tròn 3 chữ số: JSON nhỏ đi khoảng một nửa mà mắt không phân biệt được
	// chênh lệch dưới một phần nghìn trên một cột cao vài chục điểm ảnh.
	for i := range peaks {
		peaks[i] = float64(int(peaks[i]*1000+0.5)) / 1000
	}
	writeJSON(w, http.StatusOK, map[string]any{"peaks": peaks})
}
