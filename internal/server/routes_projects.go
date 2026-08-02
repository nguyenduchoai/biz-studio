package server

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"bizstudio/internal/store"
)

// routesProjects — CRUD dự án + các job endpoint (qc/thumbnail/publish/render
// nằm ở routes_projects_jobs.go).
func (s *Server) routesProjects(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/projects", s.handleProjectList)
	mux.HandleFunc("POST /api/projects", s.handleProjectCreate)
	mux.HandleFunc("GET /api/projects/{id}", s.handleProjectDetail)
	mux.HandleFunc("PUT /api/projects/{id}", s.handleProjectUpdate)
	mux.HandleFunc("DELETE /api/projects/{id}", s.handleProjectDelete)
	mux.HandleFunc("POST /api/projects/{id}/duplicate", s.handleProjectDuplicate)
	mux.HandleFunc("POST /api/projects/{id}/qc", s.handleProjectQC)
	mux.HandleFunc("POST /api/projects/{id}/thumbnail", s.handleProjectThumbnail)
	mux.HandleFunc("POST /api/projects/{id}/publish", s.handleProjectPublish)
	mux.HandleFunc("POST /api/projects/{id}/render-final", s.handleProjectRenderFinal)
}

func (s *Server) handleProjectList(w http.ResponseWriter, r *http.Request) {
	list := s.st.Projects()
	if list == nil {
		list = []store.Project{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleProjectCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name   string `json:"name"`
		Kind   string `json:"kind"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
		FPS    int    `json:"fps"`
	}
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = "Dự án mới"
	}
	kind := strings.TrimSpace(body.Kind)
	if kind == "" {
		kind = "video"
	}
	if body.Width <= 0 {
		body.Width = 1080
	}
	if body.Height <= 0 {
		body.Height = 1920
	}
	if body.FPS <= 0 {
		body.FPS = 30
	}
	p := store.Project{
		Name: name, Kind: kind,
		Width: body.Width, Height: body.Height, FPS: body.FPS,
		Status: "draft", Tags: []string{}, Keywords: []string{},
	}
	s.st.SaveProject(&p)
	s.ProjectDir(p.ID) // tạo sẵn cấu trúc thư mục dự án
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleProjectDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, ok := s.st.Project(id)
	if !ok {
		httpErr(w, http.StatusNotFound, "không tìm thấy dự án %q", id)
		return
	}
	assets := s.st.AssetsByProject(id)
	if assets == nil {
		assets = []store.Asset{}
	}
	sessions := s.st.SessionsByProject(id)
	if sessions == nil {
		sessions = []store.Session{}
	}
	jobs := []store.Job{}
	for _, j := range s.st.Jobs() {
		if j.ProjectID == id {
			jobs = append(jobs, j)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"project": p, "assets": assets, "sessions": sessions, "jobs": jobs,
	})
}

func (s *Server) handleProjectUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, ok := s.st.Project(id)
	if !ok {
		httpErr(w, http.StatusNotFound, "không tìm thấy dự án %q", id)
		return
	}
	var body store.Project
	if err := readJSON(r, &body); err != nil {
		httpErr(w, http.StatusBadRequest, "%s", err)
		return
	}
	// Chỉ ghi đè các field editable.
	if strings.TrimSpace(body.Name) != "" {
		p.Name = strings.TrimSpace(body.Name)
	}
	p.BriefDesc = body.BriefDesc
	p.EditPrompt = body.EditPrompt
	p.AutoCut = body.AutoCut
	p.AutoSub = body.AutoSub
	p.AutoKey = body.AutoKey
	p.Keywords = body.Keywords
	p.Tags = body.Tags
	if body.Width > 0 {
		p.Width = body.Width
	}
	if body.Height > 0 {
		p.Height = body.Height
	}
	if body.FPS > 0 {
		p.FPS = body.FPS
	}
	if body.Status != "" {
		p.Status = body.Status
	}
	p.Progress = body.Progress
	s.st.SaveProject(&p)
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleProjectDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := s.st.Project(id); !ok {
		httpErr(w, http.StatusNotFound, "không tìm thấy dự án %q", id)
		return
	}
	if err := os.RemoveAll(s.ProjectDir(id)); err != nil {
		httpErr(w, http.StatusInternalServerError, "không xoá được thư mục dự án: %s", err)
		return
	}
	s.st.DeleteProject(id)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleProjectDuplicate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	src, ok := s.st.Project(id)
	if !ok {
		httpErr(w, http.StatusNotFound, "không tìm thấy dự án %q", id)
		return
	}
	np := src
	np.ID, np.Name = "", src.Name+" (bản sao)"
	np.Status, np.Progress = "draft", 0
	np.OutputFile, np.ThumbFile = "", ""
	s.st.SaveProject(&np)
	dstDir := s.ProjectDir(np.ID)

	for _, a := range s.st.AssetsByProject(src.ID) {
		srcPath := filepath.Join(s.DataDir, a.Path)
		if _, err := os.Stat(srcPath); err != nil {
			s.Log("warn", "projects", "Bỏ qua asset thiếu file khi nhân bản: "+a.Path)
			continue
		}
		fileName := filepath.Base(a.Path)
		if err := copyFileTo(srcPath, filepath.Join(dstDir, "assets", fileName)); err != nil {
			httpErr(w, http.StatusInternalServerError, "không copy được asset %q: %s", a.Name, err)
			return
		}
		na := a
		na.ID, na.ProjectID = "", np.ID
		na.Path = "projects/" + np.ID + "/assets/" + fileName
		s.st.SaveAsset(&na)
	}
	writeJSON(w, http.StatusOK, np)
}

// copyFileTo copy nội dung file src sang dst (tạo thư mục cha nếu thiếu).
func copyFileTo(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("không tạo được thư mục %s: %w", filepath.Dir(dst), err)
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
